package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestDownloadRejectsExternalStorageWithoutExposingSignedURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	requests := 0
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer storage.Close()
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, storage.URL+"/object?signature=synthetic-signed-secret", http.StatusFound)
	}))
	defer gateway.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic-key"}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("assets", "download", "--file-path", "workspace/example", "--json")
	if err == nil || !strings.Contains(stderr, "refusing to redirect") {
		t.Fatal("expected blocked external download")
	}
	if requests != 0 {
		t.Fatal("external storage received request")
	}
	if strings.Contains(out+stderr, "synthetic-signed-secret") || strings.Contains(out+stderr, "synthetic-key") {
		t.Fatal("output exposed a credential")
	}
}
