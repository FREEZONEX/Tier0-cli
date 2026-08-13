package embeddedskill

import (
	"io/fs"
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
}
