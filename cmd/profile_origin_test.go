package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestConfigOriginRebindProtectsWhoamiAndDoctor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	oldKey := "synthetic-old-credential"
	newKey := "synthetic-new-credential"
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request to old origin") }))
	defer old.Close()
	var keys []string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("x-api-key"))
		switch r.URL.Path {
		case "/openapi/v2/info":
			fmt.Fprint(w, `{"code":200,"data":{"name":"fixture","version":"dev"}}`)
		case "/openapi/v2/auth/whoami":
			fmt.Fprint(w, `{"code":200,"data":{"workspaceID":1,"keyType":"service"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer target.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: old.URL, APIKey: oldKey}); err != nil {
		t.Fatal(err)
	}
	out, _, err := executeRootForTest("config", "--base-url", target.URL, "--json")
	if err != nil || !strings.Contains(out, `"apiKeyConfigured":false`) {
		t.Fatalf("switch failed: %v %s", err, out)
	}
	if _, _, err = executeRootForTest("auth", "whoami", "--json"); err == nil {
		t.Fatal("whoami accepted cleared key")
	}
	if _, _, err = executeRootForTest("doctor", "--json"); err == nil {
		t.Fatal("doctor accepted cleared key")
	}
	for _, key := range keys {
		if key != "" {
			t.Fatal("key sent without rebinding")
		}
	}
	if _, _, err = executeRootForTest("config", "--base-url", old.URL, "--api-key", oldKey); err != nil {
		t.Fatal(err)
	}
	if _, _, err = executeRootForTest("config", "--base-url", target.URL, "--api-key", newKey); err != nil {
		t.Fatal(err)
	}
	keys = nil
	for _, args := range [][]string{{"auth", "whoami", "--json"}, {"doctor", "--json"}} {
		out, stderr, err := executeRootForTest(args...)
		if err != nil {
			t.Fatalf("%v failed: %v", args, err)
		}
		if strings.Contains(out, newKey) || strings.Contains(stderr, newKey) {
			t.Fatal("output leaked key")
		}
	}
	if len(keys) != 3 {
		t.Fatalf("expected three authenticated requests, got %d", len(keys))
	}
	for _, key := range keys {
		if key != newKey {
			t.Fatal("request did not use explicitly rebound key")
		}
	}
}

func TestConfigRetainsSameOriginAndRejectsInvalidURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := config.SaveProfile(config.Profile{BaseURL: "https://EXAMPLE.test", APIKey: "synthetic-key"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeRootForTest("config", "--base-url", "https://example.test:443/path"); err != nil {
		t.Fatal(err)
	}
	p, _ := config.LoadProfile()
	if p.APIKey != "synthetic-key" {
		t.Fatal("same origin cleared key")
	}
	if _, _, err := executeRootForTest("config", "--base-url", "https://user:secret@example.test"); err == nil {
		t.Fatal("accepted credentials in URL")
	}
	p, _ = config.LoadProfile()
	if p.APIKey != "synthetic-key" || p.BaseURL != "https://example.test:443/path" {
		t.Fatal("invalid URL changed profile")
	}
}

func TestExplicitKeyBindsDefaultAndDoesNotUseEnvironmentOverride(t *testing.T) {
	for _, kind := range []string{"personal", "service", "app"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TIER0_BASE_URL", "")
			defaultTarget := cmdutil.ResolveBaseURL("")
			if _, _, err := executeRootForTest("config", "--api-key", "synthetic-"+kind, "--json"); err != nil {
				t.Fatal(err)
			}
			profile, err := config.LoadProfile()
			if err != nil || profile.BaseURL != defaultTarget || profile.APIKey != "synthetic-"+kind {
				t.Fatal("explicit key was not bound to the default origin")
			}
		})
	}
	t.Run("saved target wins over environment", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("TIER0_BASE_URL", "https://other.example")
		if err := config.SaveProfile(config.Profile{BaseURL: "https://saved.example"}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := executeRootForTest("config", "--api-key", "synthetic-key"); err != nil {
			t.Fatal(err)
		}
		profile, _ := config.LoadProfile()
		if profile.BaseURL != "https://saved.example" || profile.APIKey != "synthetic-key" {
			t.Fatal("key was not bound to the saved target")
		}
	})
	t.Run("ambiguous environment requires explicit target", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("TIER0_BASE_URL", "https://other.example")
		if _, _, err := executeRootForTest("config", "--api-key", "synthetic-key"); err == nil {
			t.Fatal("accepted an ambiguous target")
		}
		profile, err := config.LoadProfile()
		if err != nil || profile.APIKey != "" || profile.BaseURL != "" {
			t.Fatal("failed configuration changed the profile")
		}
		if _, _, err := executeRootForTest("config", "--base-url", "https://other.example", "--api-key", "synthetic-key"); err != nil {
			t.Fatal(err)
		}
		profile, _ = config.LoadProfile()
		if profile.BaseURL != "https://other.example" || profile.APIKey != "synthetic-key" {
			t.Fatal("explicit target and key were not saved")
		}
	})
}
