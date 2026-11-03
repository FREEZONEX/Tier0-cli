package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIKeyRedirectCannotCrossOrigin(t *testing.T) {
	for _, prefix := range []string{"sk-per-", "sk-svc-", "sk-app-"} {
		t.Run(prefix, func(t *testing.T) {
			forwarded := 0
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded++ }))
			defer other.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, other.URL+"/target", http.StatusTemporaryRedirect)
			}))
			defer server.Close()
			_, err := New(server.URL, prefix+"synthetic").DoAPI(context.Background(), "/path", http.MethodPost, "{}", false)
			if err == nil || !strings.Contains(err.Error(), "refusing to redirect API key") {
				t.Fatal("expected cross-origin redirect to be rejected")
			}
			if forwarded != 0 {
				t.Fatal("request reached the other origin")
			}
		})
	}
}

func TestRedactDebugPayload(t *testing.T) {
	raw := `{"data":{"clientID":"client-1","username":"user","password":"mqtt-secret","nested":{"apiKey":"api-secret","access_token":"token-secret"}}}`
	got := redactDebugPayload(raw)
	for _, secret := range []string{"mqtt-secret", "api-secret", "token-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("debug payload leaked %q: %s", secret, got)
		}
	}
	for _, visible := range []string{"client-1", "user"} {
		if !strings.Contains(got, visible) {
			t.Fatalf("debug payload lost non-secret %q: %s", visible, got)
		}
	}
}

func TestRedactDebugPayloadPreservesNonJSON(t *testing.T) {
	const raw = "plain diagnostic"
	if got := redactDebugPayload(raw); got != raw {
		t.Fatalf("got %q, want %q", got, raw)
	}
}
