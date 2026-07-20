package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAgentSkillsSyncArgs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tier0 skills")
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-y", "skills", "add", abs, "-y", "-g", "--copy"}
	if got := AgentSkillsSyncArgs(dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("AgentSkillsSyncArgs() = %#v, want %#v", got, want)
	}
}

func TestSyncAgentSkillsUsesLocalPackage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: tier0\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldLookPath := agentSkillsLookPath
	oldRun := agentSkillsRun
	t.Cleanup(func() {
		agentSkillsLookPath = oldLookPath
		agentSkillsRun = oldRun
	})

	agentSkillsLookPath = func(file string) (string, error) {
		if file != "npx" {
			t.Fatalf("LookPath(%q), want npx", file)
		}
		return "npx-test", nil
	}
	var gotName string
	var gotArgs []string
	agentSkillsRun = func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil, nil
	}

	if err := SyncAgentSkills(dir); err != nil {
		t.Fatal(err)
	}
	if gotName != "npx-test" {
		t.Fatalf("runner name = %q, want npx-test", gotName)
	}
	if want := AgentSkillsSyncArgs(dir); !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("runner args = %#v, want %#v", gotArgs, want)
	}
}

func TestSyncAgentSkillsReportsMissingNpx(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: tier0\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldLookPath := agentSkillsLookPath
	t.Cleanup(func() { agentSkillsLookPath = oldLookPath })
	agentSkillsLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	if err := SyncAgentSkills(dir); err == nil {
		t.Fatal("SyncAgentSkills() error = nil, want missing npx error")
	}
}
