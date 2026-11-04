package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const managedConfirmPath = "/api/core/cli-auth/managed/confirm"
const managedConfirmMaxResponseBytes = 64 << 10

// HTTPManagedConfirmationClient calls only the explicitly negotiated managed endpoint.
type HTTPManagedConfirmationClient struct {
	Client *http.Client
}

func (c HTTPManagedConfirmationClient) ConfirmManaged(ctx context.Context, baseURL, setupCode, apiKey string) (bool, bool, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" || setupCode == "" || apiKey == "" {
		return false, false, ErrManagedConfirmUncertain
	}
	if base.Scheme == "http" && base.Hostname() != "localhost" {
		ip := net.ParseIP(base.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return false, false, ErrManagedConfirmUncertain
		}
	}
	base.Path = strings.TrimRight(base.Path, "/") + managedConfirmPath
	body, err := json.Marshal(struct {
		SetupCode string `json:"setupCode"`
		APIKey    string `json:"apiKey"`
	}{setupCode, apiKey})
	if err != nil {
		return false, false, ErrManagedConfirmUncertain
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base.String(), bytes.NewReader(body))
	if err != nil {
		return false, false, ErrManagedConfirmUncertain
	}
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 10 * time.Second}
	if c.Client != nil {
		client = *c.Client
	}
	// A redirect must never forward the proof to a different endpoint or host.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return false, false, ErrManagedConfirmUncertain
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, false, ErrManagedConfirmUncertain
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, managedConfirmMaxResponseBytes)).Decode(&envelope); err != nil || envelope.Code != http.StatusOK {
		return false, false, ErrManagedConfirmUncertain
	}
	switch envelope.Data.Status {
	case "confirmed":
		return true, false, nil
	case "rejected":
		return false, true, nil
	default:
		return false, false, errors.New("managed confirmation returned an unknown state")
	}
}
