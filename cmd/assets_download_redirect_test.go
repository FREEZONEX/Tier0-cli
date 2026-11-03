package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestAssetsDownloadRejectsCrossOriginRedirect(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	forwarded := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded++ }))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/target", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	key := "synthetic-download-key"
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: key}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("assets", "download", "--file-path", "workspace/fixture")
	if err == nil || forwarded != 0 {
		t.Fatal("cross-origin download redirect must be rejected")
	}
	if strings.Contains(out, key) || strings.Contains(stderr, key) {
		t.Fatal("download output exposed the key")
	}
}
