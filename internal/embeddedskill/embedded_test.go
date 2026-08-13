package embeddedskill

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedSkillContainsRuntimeFilesOnly(t *testing.T) {
	content, err := FS()
	if err != nil {
		t.Fatalf("FS() error = %v", err)
	}
	if data, err := fs.ReadFile(content, "SKILL.md"); err != nil || len(data) == 0 {
		t.Fatalf("embedded SKILL.md missing or empty: %v", err)
	}

	excluded := []string{
		"README.md",
		"CHANGELOG.md",
		"install-openclaw.sh",
		"flow/references/protocal",
	}
	for _, path := range excluded {
		if _, err := fs.Stat(content, path); err == nil {
			t.Errorf("maintainer-only path %q was embedded", path)
		}
	}

	sourceData, err := fs.ReadFile(content, "_source.json")
	if err != nil {
		t.Fatalf("embedded source metadata missing: %v", err)
	}
	var source struct {
		Repository string `json:"repository"`
		Commit     string `json:"commit"`
	}
	if err := json.Unmarshal(sourceData, &source); err != nil {
		t.Fatalf("invalid embedded source metadata: %v", err)
	}
	if source.Repository != "https://github.com/FREEZONEX/Tier0-skill" {
		t.Fatalf("embedded source repository = %q", source.Repository)
	}
	if len(source.Commit) != 40 || strings.Trim(source.Commit, "0123456789abcdefABCDEF") != "" {
		t.Fatalf("embedded source commit is not a full Git SHA: %q", source.Commit)
	}
}
