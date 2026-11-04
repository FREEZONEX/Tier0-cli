package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestHTTPManagedConfirmationContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != managedConfirmPath {
			t.Errorf("unexpected confirmation request")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req struct {
			Setup string `json:"setupCode"`
			Key   string `json:"apiKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Setup != "setup" || req.Key != "proof" {
			t.Errorf("incorrect confirmation request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"code":200,"data":{"status":"confirmed"}}`))
	}))
	t.Cleanup(server.Close)
	confirmed, rejected, err := (HTTPManagedConfirmationClient{}).ConfirmManaged(context.Background(), server.URL, "setup", "proof")
	if err != nil || !confirmed || rejected {
		t.Fatalf("confirmed=%v rejected=%v err=%v", confirmed, rejected, err)
	}
}

func TestHTTPManagedConfirmationDoesNotForwardProofOnRedirect(t *testing.T) {
	var forwarded atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { forwarded.Store(true) }))
	t.Cleanup(target.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(server.Close)
	confirmed, rejected, err := (HTTPManagedConfirmationClient{}).ConfirmManaged(context.Background(), server.URL, "setup", "proof")
	if err == nil || confirmed || rejected || forwarded.Load() {
		t.Fatalf("redirect forwarded proof or was treated as final")
	}
}

func TestHTTPManagedConfirmationFailsClosedOnOldOrUnavailableServer(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{
		{http.StatusNotFound, `{"code":200,"data":{"status":"rejected"}}`},
		{http.StatusServiceUnavailable, `{"code":200,"data":{"status":"rejected"}}`},
		{http.StatusOK, `{"code":200,"data":{"status":"pending"}}`},
		{http.StatusOK, `{"code":500,"data":{"status":"rejected"}}`},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		confirmed, rejected, err := (HTTPManagedConfirmationClient{}).ConfirmManaged(context.Background(), server.URL, "setup", "proof")
		server.Close()
		if err == nil || confirmed || rejected {
			t.Fatalf("status %d incorrectly treated as a definitive response", tc.status)
		}
	}
	if _, _, err := (HTTPManagedConfirmationClient{}).ConfirmManaged(context.Background(), "http://remote.example", "setup", "proof"); err == nil {
		t.Fatal("plaintext non-loopback confirmation must fail closed")
	}
}

func TestHTTPManagedConfirmationDefinitiveRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":200,"data":{"status":"rejected"}}`))
	}))
	t.Cleanup(server.Close)
	confirmed, rejected, err := (HTTPManagedConfirmationClient{}).ConfirmManaged(context.Background(), server.URL, "setup", "proof")
	if err != nil || confirmed || !rejected {
		t.Fatalf("definitive rejection was not recognized")
	}
}
