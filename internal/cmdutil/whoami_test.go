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
