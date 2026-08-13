package upgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

func TestEnsureEmbeddedSkillsInstallsAndRepairsBaseline(t *testing.T) {
	oldVersion := version.BuildVersion
	version.BuildVersion = "v9.9.9-test"
	t.Cleanup(func() { version.BuildVersion = oldVersion })

	dir := filepath.Join(t.TempDir(), "skills")
	result, err := EnsureEmbeddedSkills(dir, false)
	if err != nil {
		t.Fatalf("EnsureEmbeddedSkills() error = %v", err)
	}
	if result.Action != "installed" || result.Source != SkillSourceEmbedded {
		t.Fatalf("unexpected install result: %#v", result)
	}
	if !validSkillPackage(dir) {
		t.Fatal("embedded Skill was not installed")
	}
	if _, err := os.Stat(filepath.Join(dir, "flow", "references", "protocal")); !os.IsNotExist(err) {
		t.Fatal("maintainer-only protocol directory was installed")
	}

	current, err := EnsureEmbeddedSkills(dir, false)
	if err != nil || current.Action != "in_sync" {
		t.Fatalf("current embedded Skill result = %#v, err = %v", current, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("damaged"), 0o644); err != nil {
		t.Fatal(err)
	}
	repaired, err := EnsureEmbeddedSkills(dir, false)
	if err != nil {
		t.Fatalf("repair error = %v", err)
	}
	if repaired.Action != "replaced" || repaired.Reason != "damaged_embedded_baseline" {
		t.Fatalf("unexpected repair result: %#v", repaired)
	}
}

func TestEnsureEmbeddedSkillsPreservesIndependentRemoteUpdate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	remoteSkill := []byte("remote independent Skill")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), remoteSkill, 0o644); err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(SkillsMetadata{
		Version: "remote-sha",
		Source:  SkillSourceRemote,
		Commit:  "remote-sha",
	})
	if err := os.WriteFile(filepath.Join(dir, "_meta.json"), metadata, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := EnsureEmbeddedSkills(dir, false)
	if err != nil {
		t.Fatalf("EnsureEmbeddedSkills() error = %v", err)
	}
	if result.Action != "preserved" || result.Source != SkillSourceRemote {
		t.Fatalf("remote Skill was not preserved: %#v", result)
	}
	got, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil || string(got) != string(remoteSkill) {
		t.Fatalf("remote Skill content changed: %q, err = %v", got, err)
	}

	forced, err := EnsureEmbeddedSkills(dir, true)
	if err != nil {
		t.Fatalf("forced install error = %v", err)
	}
	if forced.Source != SkillSourceEmbedded || forced.Reason != "forced" {
		t.Fatalf("unexpected forced result: %#v", forced)
	}
}

func TestEnsureEmbeddedSkillsMigratesLegacyBundledButPreservesLegacyRemote(t *testing.T) {
	oldVersion := version.BuildVersion
	version.BuildVersion = "v0.6.6-test"
	t.Cleanup(func() { version.BuildVersion = oldVersion })

	legacyBundled := filepath.Join(t.TempDir(), "bundled")
	if err := writeLegacySkill(legacyBundled, "v0.6.5"); err != nil {
		t.Fatal(err)
	}
	bundledResult, err := EnsureEmbeddedSkills(legacyBundled, false)
	if err != nil {
		t.Fatal(err)
	}
	if bundledResult.Source != SkillSourceEmbedded || bundledResult.Reason != "legacy_embedded_baseline" {
		t.Fatalf("legacy bundled Skill was not migrated: %#v", bundledResult)
	}

	legacyRemote := filepath.Join(t.TempDir(), "remote")
	if err := writeLegacySkill(legacyRemote, "a1b2c3d4"); err != nil {
		t.Fatal(err)
	}
	remoteResult, err := EnsureEmbeddedSkills(legacyRemote, false)
	if err != nil {
		t.Fatal(err)
	}
	if remoteResult.Action != "preserved" || remoteResult.Source != "external" {
		t.Fatalf("legacy remote Skill was not preserved: %#v", remoteResult)
	}
}

func writeLegacySkill(dir, skillVersion string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("legacy Skill"), 0o644); err != nil {
		return err
	}
	metadata, err := json.Marshal(map[string]string{
		"version":    skillVersion,
		"updated_at": "2026-08-13T00:00:00Z",
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "_meta.json"), metadata, 0o644)
}

func TestSkillsLastUpdatedReadsLegacyMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "_meta.json"), []byte(`{"version":"old","updated_at":"2026-08-13T00:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SkillsLastUpdated(dir); got != "2026-08-13T00:00:00Z" {
		t.Fatalf("SkillsLastUpdated() = %q", got)
	}
}

func TestListSkillsIncludesRootPackage(t *testing.T) {
	dir := t.TempDir()
	skill := "---\nname: tier0\ndescription: Tier0 platform operations\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ListSkills(dir)
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	if len(result.Skills) != 1 || result.Skills[0].Name != "tier0" {
		t.Fatalf("root Skill not listed: %#v", result)
	}
}

func TestFindSkillsDirPrefersCanonicalUpdateLocation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	canonical := filepath.Join(home, ".tier0", "skills")
	legacyBin := filepath.Join(t.TempDir(), "bin")
	legacy := filepath.Join(legacyBin, "skill")
	for _, dir := range []string{canonical, legacy} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := FindSkillsDir(filepath.Join(legacyBin, "tier0")); got != canonical {
		t.Fatalf("FindSkillsDir() = %q, want %q", got, canonical)
	}
}

func TestRuntimeSkillPathExcluded(t *testing.T) {
	tests := map[string]bool{
		"README.md":                             true,
		"flow/references/protocal":              true,
		"flow/references/protocal/project/file": true,
		"flow/references/protocols/README.md":   false,
		"uns/SKILL.md":                          false,
		"nested/.git":                           true,
	}
	for path, want := range tests {
		if got := runtimeSkillPathExcluded(path); got != want {
			t.Errorf("runtimeSkillPathExcluded(%q) = %t, want %t", path, got, want)
		}
	}
}
