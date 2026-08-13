package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
)

func TestSkillsInstallAndStatusJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	stdout, stderr, err := executeRootForTest("skills", "install", "--no-sync", "--json")
	if err != nil {
		t.Fatalf("skills install error = %v, stderr = %s", err, stderr)
	}
	var installResult upgrade.EmbeddedSkillsResult
	if err := json.Unmarshal([]byte(stdout), &installResult); err != nil {
		t.Fatalf("invalid install JSON %q: %v", stdout, err)
	}
	if installResult.Source != upgrade.SkillSourceEmbedded {
		t.Fatalf("install source = %q", installResult.Source)
	}

	stdout, stderr, err = executeRootForTest("skills", "status", "--json")
	if err != nil {
		t.Fatalf("skills status error = %v, stderr = %s", err, stderr)
	}
	var status upgrade.SkillsStatus
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("invalid status JSON %q: %v", stdout, err)
	}
	if !status.Installed || !status.Healthy || status.Source != upgrade.SkillSourceEmbedded {
		t.Fatalf("unexpected status: %#v", status)
	}

	if _, err := os.Stat(filepath.Join(home, ".tier0", "skills", "SKILL.md")); err != nil {
		t.Fatalf("installed Skill missing: %v", err)
	}
}
