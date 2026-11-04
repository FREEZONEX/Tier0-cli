package cmd

import (
	"encoding/json"
	"fmt"
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

func TestLoginCompletedWithoutKeyClearsOldIdentity(t *testing.T) {
	for _, oldProfile := range []bool{false, true} {
		for _, key := range []string{"", "   "} {
			for _, mode := range []string{"text", "json", "safe-json"} {
				t.Run(fmt.Sprintf("old=%t/blank=%t/%s", oldProfile, key != "", mode), func(t *testing.T) {
					t.Setenv("HOME", t.TempDir())
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						_ = json.NewEncoder(w).Encode(map[string]any{
							"code": 200, "data": map[string]string{"status": "completed", "apiKey": key},
						})
					}))
					defer server.Close()
					if oldProfile {
						if err := config.ReplaceProfile(config.Profile{BaseURL: server.URL, APIKey: "old-fixture-key", Lang: "zh"}); err != nil {
							t.Fatal(err)
						}
					}
					args := []string{"login", "--base-url", server.URL, "--setup-code", "TESTCODE"}
					if mode != "text" {
						args = append(args, "--json")
					}
					if mode == "safe-json" {
						args = append(args, "--no-key-output")
					}
					out, _, err := executeRootForTest(args...)
					if err == nil || strings.Contains(out, "authorization_complete") {
						t.Fatal("missing key was reported as successful login")
					}
					profile, err := config.LoadProfile()
					if err != nil || profile.APIKey != "" {
						t.Fatal("previous identity was retained")
					}
					if oldProfile && (profile.BaseURL != server.URL || profile.Lang != "zh") {
						t.Fatal("clearing the key changed unrelated settings")
					}
				})
			}
		}
	}
}

func TestLoginNoWaitSafeHintSurvivesPolling(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const testKey = "fixture-only-credential"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200, "data": map[string]string{"status": "completed", "apiKey": testKey},
		})
	}))
	defer server.Close()
	first, _, err := executeRootForTest("login", "--base-url", server.URL, "--no-wait", "--no-key-output")
	if err != nil {
		t.Fatal(err)
	}
	const hint = "Polling with: tier0 "
	var next []string
	for _, line := range strings.Split(first, "\n") {
		if strings.HasPrefix(line, hint) {
			next = strings.Fields(strings.TrimPrefix(line, hint))
		}
	}
	if len(next) != 4 || next[0] != "login" || next[1] != "--setup-code" || next[3] != "--no-key-output" {
		t.Fatal("polling hint did not retain safe output option")
	}
	next = append(next, "--base-url", server.URL)
	second, stderr, err := executeRootForTest(next...)
	if err != nil || strings.Contains(second, testKey[:8]) || strings.Contains(stderr, testKey[:8]) {
		t.Fatal("polling failed or exposed the credential")
	}
	profile, err := config.LoadProfile()
	if err != nil || profile.APIKey != testKey {
		t.Fatal("polling did not save the credential")
	}
}
