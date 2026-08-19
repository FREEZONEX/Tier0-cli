package client

import (
	"strings"
	"testing"
)

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
