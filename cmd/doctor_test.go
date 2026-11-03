package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestDoctorInfoUsesConfiguredAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("x-api-key"); got != "test-api-key" {
			t.Errorf("x-api-key = %q, want configured key", got)
		}
		if got := r.Header.Get("X-Tier0-Source"); got != "tier0-cli" {
			t.Errorf("X-Tier0-Source = %q, want tier0-cli", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"name":"edge-enterprise","version":"2.2.5","capabilities":["uns.read"],"mqttBroker":"tcp://mqtt.example.test:1883"}}`))
	}))
	defer server.Close()

	summary, err := doctorInfo(context.Background(), server.URL, "test-api-key", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"edge-enterprise", "version=2.2.5", "mqtt=tcp://mqtt.example.test:1883", "capabilities=uns.read"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q does not contain %q", summary, want)
		}
	}
}

func TestDoctorUsesProfileTargetForConfiguredKeys(t *testing.T) {
	for _, tc := range []struct{ name, prefix string }{{"personal", "sk-per-"}, {"service", "sk-svc-"}, {"app", "sk-app-"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			key := tc.prefix + "synthetic-doctor"
			unexpectedRequests := 0
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { unexpectedRequests++ }))
			defer other.Close()
			t.Setenv("TIER0_BASE_URL", other.URL)
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Header.Get("x-api-key") != key {
					t.Error("profile key was not used")
				}
				switch r.URL.Path {
				case "/openapi/v1/info":
					fmt.Fprint(w, `{"code":200,"data":{"name":"fixture"}}`)
				case "/openapi/v2/auth/whoami":
					fmt.Fprintf(w, `{"code":200,"data":{"keyType":%q}}`, tc.name)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: key}); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := executeRootForTest("doctor", "--json")
			if err != nil || !strings.Contains(out, `"type=`+tc.name+`"`) || !strings.Contains(out, server.URL) {
				t.Fatalf("doctor did not diagnose the saved profile: err=%v", err)
			}
			if requests != 2 || unexpectedRequests != 0 {
				t.Fatalf("unexpected request routing: profile=%d other=%d", requests, unexpectedRequests)
			}
			if strings.Contains(out, key) || strings.Contains(stderr, key) {
				t.Fatal("doctor output exposed the key")
			}
		})
	}
}

func TestDoctorWithUnboundKeyDoesNotSendRequests(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	t.Setenv("TIER0_BASE_URL", server.URL)
	if err := config.SaveProfile(config.Profile{APIKey: "synthetic-unbound"}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("doctor", "--json")
	if err == nil || !strings.Contains(out, "saved API key has no configured base URL") {
		t.Fatal("doctor should report the missing bound URL")
	}
	if requests != 0 {
		t.Fatal("doctor sent a request for an unbound key")
	}
	if strings.Contains(out, "synthetic-unbound") || strings.Contains(stderr, "synthetic-unbound") {
		t.Fatal("doctor output exposed the key")
	}
}

func TestDoctorRejectsCrossOriginRedirects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	requests := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic-redirect"}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("doctor", "--json")
	if err == nil || !strings.Contains(out, "refusing to redirect API key") {
		t.Fatal("doctor should report the blocked redirect")
	}
	if requests != 0 {
		t.Fatal("doctor followed a cross-origin redirect")
	}
	if strings.Contains(out, "synthetic-redirect") || strings.Contains(stderr, "synthetic-redirect") {
		t.Fatal("doctor output exposed the key")
	}
}

func TestDoctorAllowsSameOriginRedirect(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "synthetic-same-origin" {
			t.Error("key was not preserved for the profile origin")
		}
		switch r.URL.Path {
		case "/openapi/v1/info":
			http.Redirect(w, r, "/info", http.StatusTemporaryRedirect)
		case "/info":
			fmt.Fprint(w, `{"code":200,"data":{"name":"fixture"}}`)
		case "/openapi/v2/auth/whoami":
			fmt.Fprint(w, `{"code":200,"data":{"keyType":"service"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic-same-origin"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeRootForTest("doctor", "--json"); err != nil {
		t.Fatalf("same-origin redirect should work: %v", err)
	}
}

func TestDoctorWithoutKeyUsesEnvironmentForConnectivity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/v1/info" || r.Header.Get("x-api-key") != "" {
			t.Error("unexpected unauthenticated connectivity request")
		}
		fmt.Fprint(w, `{"code":200,"data":{"name":"fixture"}}`)
	}))
	defer server.Close()
	t.Setenv("TIER0_BASE_URL", server.URL)
	out, _, err := executeRootForTest("doctor", "--json")
	if err == nil || !strings.Contains(out, "fixture") || !strings.Contains(out, "API key is not configured") {
		t.Fatal("doctor should diagnose connectivity and the missing key")
	}
}
