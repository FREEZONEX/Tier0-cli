package cmdutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestDoWhoamiUsesConfiguredProductKey(t *testing.T) {
	for _, keyType := range []string{"personal", "service", "app"} {
		t.Run(keyType, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			key := "synthetic-" + keyType
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/openapi/v2/auth/whoami" || r.Header.Get("x-api-key") != key {
					t.Error("identity request did not use the configured key and current path")
				}
				fmt.Fprintf(w, `{"code":200,"data":{"keyType":%q}}`, keyType)
			}))
			defer server.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: key}); err != nil {
				t.Fatal(err)
			}
			response, err := DoWhoami(context.Background(), false)
			if err != nil || !strings.Contains(response, `"keyType":"`+keyType+`"`) {
				t.Fatalf("identity request failed: %v", err)
			}
		})
	}
}

func TestEnvironmentKeyDoesNotOverrideConfiguredProductKeys(t *testing.T) {
	for _, tc := range []struct{ name, prefix string }{{"personal", "sk-per-"}, {"service", "sk-svc-"}, {"app", "sk-app-"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			configuredKey := tc.prefix + "fixture-configured"
			environmentKey := tc.prefix + "fixture-environment"
			unexpectedRequests := 0
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				unexpectedRequests++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer other.Close()
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Header.Get("x-api-key") != configuredKey {
					t.Error("request did not use the configured key")
				}
				if r.URL.Path != "/openapi/v2/auth/whoami" && r.URL.Path != "/openapi/v2/example" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				fmt.Fprint(w, `{"code":200}`)
			}))
			defer server.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: configuredKey}); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TIER0_API_KEY", environmentKey)
			t.Setenv("TIER0_BASE_URL", other.URL)
			if _, err := DoWhoami(context.Background(), false); err != nil {
				t.Fatalf("whoami failed: %v", err)
			}
			if _, err := DoAPI(context.Background(), "/openapi/v2/example", http.MethodPost, "{}", false); err != nil {
				t.Fatalf("API request failed: %v", err)
			}
			if requests != 2 || unexpectedRequests != 0 {
				t.Fatalf("unexpected request routing: configured=%d environment=%d", requests, unexpectedRequests)
			}
		})
	}
}

func TestEnvironmentKeyAloneDoesNotAuthenticate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_API_KEY", "fixture-environment-only")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	t.Setenv("TIER0_BASE_URL", server.URL)
	if _, err := DoWhoami(context.Background(), false); err == nil {
		t.Fatal("environment-only key must not silently authenticate")
	} else if strings.Contains(err.Error(), "fixture-environment-only") {
		t.Fatal("authentication error exposed the environment key")
	}
	if _, err := DoAPI(context.Background(), "/openapi/v2/example", http.MethodPost, "{}", false); err == nil {
		t.Fatal("environment-only key must not silently authenticate")
	} else if strings.Contains(err.Error(), "fixture-environment-only") {
		t.Fatal("authentication error exposed the environment key")
	}
	if requests != 0 {
		t.Fatal("request was sent with an environment-only key")
	}
}
