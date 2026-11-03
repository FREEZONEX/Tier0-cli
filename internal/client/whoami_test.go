package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestWhoamiUsesCurrentContractForProductKeys(t *testing.T) {
	for _, keyType := range []string{"personal", "service", "app"} {
		t.Run(keyType, func(t *testing.T) {
			key := "synthetic-" + keyType
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/openapi/v2/auth/whoami" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("x-api-key") != key {
					t.Error("configured key was not forwarded")
				}
				if r.Header.Get("Authorization") != "" || r.Header.Get("X-Tier0-App-Data-Subject") != "" {
					t.Error("unexpected identity or delegation header")
				}
				body, _ := io.ReadAll(r.Body)
				if string(body) != "{}" {
					t.Errorf("unexpected request body: %s", body)
				}
				fmt.Fprintf(w, `{"code":200,"data":{"keyType":%q,"permissions":["uns.read"]}}`, keyType)
			}))
			defer server.Close()
			response, err := New(server.URL, key).Whoami(context.Background(), false)
			if err != nil || !strings.Contains(response, `"keyType":"`+keyType+`"`) {
				t.Fatalf("identity response missing expected key type: err=%v", err)
			}
		})
	}
}

func TestWhoamiFallsBackOnlyOnHTTPNotFound(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if r.Header.Get("x-api-key") != "synthetic-key" {
					t.Error("configured key was not forwarded")
				}
				if len(paths) == 1 {
					w.WriteHeader(status)
					io.WriteString(w, `{"code":403,"msg":"denied"}`)
					return
				}
				io.WriteString(w, `{"code":200}`)
			}))
			defer server.Close()
			_, err := New(server.URL, "synthetic-key").Whoami(context.Background(), false)
			want := []string{"/openapi/v2/auth/whoami"}
			if status == http.StatusNotFound {
				want = append(want, "/openapi/v1/auth/whoami")
			}
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("requests = %v, want %v", paths, want)
			}
			if (status == http.StatusNotFound || status == http.StatusOK) != (err == nil) {
				t.Fatalf("unexpected error status: %v", err)
			}
		})
	}
}
