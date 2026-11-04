package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestLoginOutputAndSavedCredential(t *testing.T) {
	const testKey = "fixture-only-credential"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/core/cli-auth/status" {
			t.Errorf("unexpected login request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200, "data": map[string]string{"status": "completed", "apiKey": testKey},
		})
	}))
	defer server.Close()

	for _, tc := range []struct {
		name    string
		args    []string
		json    bool
		showKey bool
	}{
		{"legacy-json", []string{"--json"}, true, true},
		{"safe-json", []string{"--json", "--no-key-output"}, true, false},
		{"safe-text", []string{"--no-key-output"}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			args := append([]string{"login", "--base-url", server.URL, "--setup-code", "TESTCODE"}, tc.args...)
			out, stderr, err := executeRootForTest(args...)
			if err != nil {
				t.Fatalf("login failed: %v", err)
			}
			profile, err := config.LoadProfile()
			if err != nil || profile.APIKey != testKey || profile.BaseURL != server.URL {
				t.Fatal("login did not save the expected credential and origin")
			}
			if tc.json {
				var event map[string]any
				if err := json.Unmarshal([]byte(out), &event); err != nil || event["event"] != "authorization_complete" {
					t.Fatal("invalid completion event")
				}
				value, present := event["api_key"]
				if present != tc.showKey || (present && value != testKey) {
					t.Fatal("unexpected credential delivery contract")
				}
			}
			if !tc.showKey && (strings.Contains(out, testKey[:8]) || strings.Contains(stderr, testKey[:8])) {
				t.Fatal("safe output exposed a credential or prefix")
			}
		})
	}
}
