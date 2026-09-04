package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
