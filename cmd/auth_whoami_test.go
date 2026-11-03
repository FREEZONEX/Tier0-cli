package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestWhoamiAndDoctorUseCurrentKeyIdentity(t *testing.T) {
	for _, keyType := range []string{"personal", "service", "app", "agent", "future_type"} {
		t.Run(keyType, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("TIER0_BASE_URL", "")
			key := "synthetic-" + keyType
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-api-key") != key {
					t.Error("configured key was not forwarded")
				}
				switch r.URL.Path {
				case "/openapi/v2/info":
					fmt.Fprint(w, `{"code":200,"data":{"name":"fixture"}}`)
				case "/openapi/v2/auth/whoami":
					fmt.Fprintf(w, `{"code":200,"data":{"keyType":%q,"permissions":["uns.read"]}}`, keyType)
				default:
					t.Errorf("unexpected endpoint %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: key}); err != nil {
				t.Fatal(err)
			}
			cache := fmt.Sprintf(`{"latest_version":"v1.0.0","checked_at":%d}`, time.Now().Unix())
			if err := os.WriteFile(filepath.Join(home, ".tier0", "update-state.json"), []byte(cache), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"auth", "whoami", "--json"}, {"doctor", "--json"}} {
				out, stderr, err := executeRootForTest(args...)
				if err != nil || !strings.Contains(out, keyType) {
					t.Fatalf("%v did not show the server-reported type: err=%v stderr=%s", args, err, stderr)
				}
				if strings.Contains(out, key) || strings.Contains(stderr, key) {
					t.Fatalf("%v exposed the configured key", args)
				}
			}
		})
	}
}
