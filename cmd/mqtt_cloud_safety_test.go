package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/mqttprofile"
)

func TestCloudMQTTProfileBindsVersionAndProtectsPassword(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/openapi/v2/info":
			fmt.Fprint(w, `{"code":200,"data":{"mqttBroker":"tcp://localhost:1883"}}`)
		case "/openapi/v2/mqtt-auth/create":
			fmt.Fprint(w, `{"code":200,"data":{"id":42,"name":"agent","clientID":"client","username":"user","password":"synthetic-secret"}}`)
		case "/openapi/v2/mqtt-auth/delete":
			fmt.Fprint(w, `{"code":200,"data":{"success":true}}`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("mqtt", "auth", "create", "--cloud", "--name", "agent", "--save", "agent", "--json")
	if err != nil || strings.Contains(out, "synthetic-secret") {
		t.Fatalf("create err=%v stderr=%s password_exposed=%v", err, stderr, strings.Contains(out, "synthetic-secret"))
	}
	store, err := mqttprofile.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	profile, err := store.Load("agent")
	if err != nil || profile.APIVersion != "v2" {
		t.Fatalf("profile version=%q err=%v", profile.APIVersion, err)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(filepath.Join(store.Dir, "agent.json")); err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("profile permissions: %v %v", info, err)
		}
		if info, err := os.Stat(store.Dir); err != nil || info.Mode().Perm() != 0700 {
			t.Fatalf("directory permissions: %v %v", info, err)
		}
	}
	_, _, err = executeRootForTest("mqtt", "auth", "delete", "--credential", "agent", "--cloud=false", "--yes", "--json")
	if err == nil || len(calls) != 2 {
		t.Fatalf("mismatch should not send delete: calls=%v err=%v", calls, err)
	}
	_, stderr, err = executeRootForTest("mqtt", "auth", "delete", "--credential", "agent", "--yes", "--json")
	if err != nil {
		t.Fatalf("delete: %v %s", err, stderr)
	}
	if strings.Join(calls, ",") != "/openapi/v2/info,/openapi/v2/mqtt-auth/create,/openapi/v2/mqtt-auth/delete" {
		t.Fatalf("calls=%v", calls)
	}
}

func TestMQTTCreateUncertainResponsesAndLocalSaveFailure(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		status         int
		collide        bool
	}{
		{"malformed", `{"password":"secret-value`, 200, false},
		{"missing_data", `{"code":200}`, 200, false},
		{"missing_password", `{"code":200,"data":{"id":42,"clientID":"id","username":"u"}}`, 200, false},
		{"remote_error", `{"code":403,"msg":"denied"}`, 403, false},
		{"save_collision", `{"code":200,"data":{"id":42,"name":"agent","clientID":"id","username":"u","password":"secret-value"}}`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TIER0_BASE_URL", "")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if tc.collide {
					store, _ := mqttprofile.DefaultStore()
					if err := os.WriteFile(filepath.Join(store.Dir, "agent.json"), []byte("existing"), 0600); err != nil {
						t.Error(err)
					}
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
				t.Fatal(err)
			}
			args := []string{"mqtt", "auth", "create", "--cloud", "--name", "agent", "--json", "--debug"}
			if tc.collide {
				args = append(args, "--save", "agent", "--broker", "tcp://localhost:1883")
			}
			out, stderr, err := executeRootForTest(args...)
			var response struct {
				Error struct {
					Retryable bool   `json:"retryable"`
					Message   string `json:"message"`
					Hint      string `json:"hint"`
				} `json:"error"`
			}
			if err == nil || calls != 1 || json.Unmarshal([]byte(stderr), &response) != nil || response.Error.Retryable || !strings.Contains(response.Error.Hint, "Inspect remote") || strings.Contains(out+stderr, "secret-value") {
				t.Fatalf("err=%v calls=%d output=%s", err, calls, stderr)
			}
			if tc.collide {
				store, _ := mqttprofile.DefaultStore()
				b, e := os.ReadFile(filepath.Join(store.Dir, "agent.json"))
				if e != nil || string(b) != "existing" {
					t.Fatal("existing profile was overwritten")
				}
			}
		})
	}
}

func TestMQTTProfileOriginIncludesCaseSensitivePath(t *testing.T) {
	if validateMQTTProfileOrigin("agent", "https://example.test/tenant-A", "https://example.test/tenant-a") == nil {
		t.Fatal("different paths accepted")
	}
	if err := validateMQTTProfileOrigin("agent", "https://EXAMPLE.test:443/tenant-A/", "https://example.test/tenant-A"); err != nil {
		t.Fatal(err)
	}
}

func TestMQTTConfiguredOriginOverridesEnvironmentForSavedProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "https://other.example")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/v2/mqtt-auth/delete" {
			t.Errorf("wrong path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"code":200,"data":{"success":true}}`)
	}))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	store, _ := mqttprofile.DefaultStore()
	if err := store.Save("agent", mqttprofile.Credential{ID: 42, BaseURL: server.URL, APIVersion: "v2", Broker: "tcp://localhost:1883", ClientID: "id", Username: "u", Password: "p"}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("mqtt", "auth", "delete", "--credential", "agent", "--dry-run", "--json")
	if err != nil || !strings.Contains(out, server.URL+"/openapi/v2/mqtt-auth/delete") {
		t.Fatalf("preview=%s err=%v stderr=%s", out, err, stderr)
	}
	if _, stderr, err = executeRootForTest("mqtt", "publish", "--credential", "agent", "--topic", "test", "--message", "ok", "--dry-run", "--json"); err != nil {
		t.Fatalf("publish preview: %v %s", err, stderr)
	}
	if _, stderr, err = executeRootForTest("mqtt", "auth", "delete", "--credential", "agent", "--yes", "--json"); err != nil {
		t.Fatalf("delete: %v %s", err, stderr)
	}
}

func TestMQTTCreateKeepsClientDuringDiscovery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	var server *httptest.Server
	requests := 0
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("x-api-key") != "initial-key" {
			t.Error("identity changed during command")
		}
		switch r.URL.Path {
		case "/openapi/v2/info":
			if err := config.SaveProfile(config.Profile{BaseURL: "https://other.example", APIKey: "other-key"}); err != nil {
				t.Error(err)
			}
			fmt.Fprint(w, `{"code":200,"data":{"mqttBroker":"tcp://localhost:1883"}}`)
		case "/openapi/v2/mqtt-auth/create":
			fmt.Fprint(w, `{"code":200,"data":{"id":42,"name":"agent","clientID":"id","username":"u","password":"synthetic"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "initial-key"}); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := executeRootForTest("mqtt", "auth", "create", "--cloud", "--name", "agent", "--save", "agent", "--json"); err != nil {
		t.Fatalf("%v %s", err, stderr)
	}
	store, _ := mqttprofile.DefaultStore()
	p, err := store.Load("agent")
	if err != nil || p.BaseURL != server.URL || requests != 2 {
		t.Fatalf("profile origin=%q requests=%d err=%v", p.BaseURL, requests, err)
	}
}

func TestLegacyMQTTProfileRequiresExplicitVersionAndConfirmedDelete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	calls := 0
	confirmed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/openapi/v1/mqtt-auth/delete" {
			t.Errorf("wrong path %s", r.URL.Path)
		}
		if confirmed {
			fmt.Fprint(w, `{"code":200,"data":{"success":true}}`)
		} else {
			fmt.Fprint(w, `{"code":200,"data":{}}`)
		}
	}))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	store, _ := mqttprofile.DefaultStore()
	c := mqttprofile.Credential{ID: 42, BaseURL: server.URL, Broker: "tcp://localhost:1883", ClientID: "id", Username: "user", Password: "secret"}
	if err := store.Save("legacy", c); err != nil {
		t.Fatal(err)
	}
	_, _, err := executeRootForTest("mqtt", "auth", "delete", "--credential", "legacy", "--yes", "--json")
	if err == nil || calls != 0 {
		t.Fatalf("legacy must require version: calls=%d err=%v", calls, err)
	}
	_, _, err = executeRootForTest("mqtt", "auth", "delete", "--credential", "legacy", "--cloud=false", "--yes", "--json")
	if err == nil || calls != 1 {
		t.Fatalf("unconfirmed deletion: calls=%d err=%v", calls, err)
	}
	if _, err = store.Load("legacy"); err != nil {
		t.Fatal("unconfirmed deletion removed profile")
	}
	confirmed = true
	_, _, err = executeRootForTest("mqtt", "auth", "delete", "--credential", "legacy", "--cloud=false", "--yes", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load("legacy"); err == nil {
		t.Fatal("confirmed deletion retained profile")
	}
}

func TestMQTTDeleteRejectsProfileFromDifferentConfiguredOrigin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") }))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TIER0_BASE_URL", "https://other.example")
	store, _ := mqttprofile.DefaultStore()
	if err := store.Save("other", mqttprofile.Credential{ID: 42, BaseURL: "https://other.example", APIVersion: "v2", Broker: "tcp://localhost:1883", ClientID: "id", Username: "user", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := executeRootForTest("mqtt", "auth", "delete", "--credential", "other", "--yes", "--json")
	var body map[string]any
	if err == nil || json.Unmarshal([]byte(stderr), &body) != nil {
		t.Fatalf("expected structured origin failure: %v %s", err, stderr)
	}
}

func TestMQTTCreateDoesNotMarkUncertainFailureRetryable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(503)
		fmt.Fprint(w, `{"code":503,"msg":"unavailable"}`)
	}))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := executeRootForTest("mqtt", "auth", "create", "--cloud", "--name", "agent", "--json")
	var response struct {
		Error struct {
			Retryable bool   `json:"retryable"`
			Message   string `json:"message"`
		} `json:"error"`
	}
	if err == nil || calls != 1 || json.Unmarshal([]byte(stderr), &response) != nil || response.Error.Retryable || !strings.Contains(response.Error.Message, "unknown") {
		t.Fatalf("calls=%d err=%v stderr=%s", calls, err, stderr)
	}
}
