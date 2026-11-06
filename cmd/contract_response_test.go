package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestCommandsRejectNonContractSuccess(t *testing.T) {
	for _, body := range []string{`<html>login</html>`, `{}`, `{"code":200,"data":{}}`} {
		t.Run(body, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TIER0_BASE_URL", "")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"auth", "whoami"}, {"auth", "whoami", "--json"}, {"doctor", "--json"}} {
				out, stderr, err := executeRootForTest(args...)
				if err == nil {
					t.Fatalf("%v accepted invalid response", args)
				}
				if strings.Contains(out, "<html>") || strings.Contains(stderr, "<html>") {
					t.Fatal("raw response leaked")
				}
				if args[0] == "doctor" && (!strings.Contains(out, "invalid info response") || !strings.Contains(out, "invalid whoami response")) {
					t.Fatalf("doctor missed invalid checks: %s", out)
				}
			}
		})
	}
}
