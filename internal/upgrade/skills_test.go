package upgrade

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckSkillsUpdateReturnsLookupError(t *testing.T) {
	original := fetchSkillRepoLatestCommit
	fetchSkillRepoLatestCommit = func() (string, error) {
		return "", errors.New("HTTP 403 from GitHub")
	}
	t.Cleanup(func() { fetchSkillRepoLatestCommit = original })

	result, err := CheckSkillsUpdate(t.TempDir())
	if err == nil {
		t.Fatal("expected lookup error")
	}
	if result == nil || !strings.Contains(result.ErrorMessage, "HTTP 403") {
		t.Fatalf("unexpected result: %#v", result)
	}
}
