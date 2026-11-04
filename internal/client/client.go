package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
)

// Client is an HTTP API client.
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// BaseURL is the origin bound to this client and its credential.
func (c *Client) BaseURL() string { return c.baseURL }

// New creates an API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second, CheckRedirect: CheckCredentialRedirect},
	}
}

// ErrCredentialRedirect avoids including a potentially signed redirect URL in errors.
var ErrCredentialRedirect = errors.New("refusing to redirect API key to another origin")

// CheckCredentialRedirect prevents an API key from reaching another origin.
func CheckCredentialRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if len(via) > 0 && via[0].Header.Get("x-api-key") != "" &&
		(!strings.EqualFold(req.URL.Scheme, via[0].URL.Scheme) || !strings.EqualFold(req.URL.Host, via[0].URL.Host)) {
		return ErrCredentialRedirect
	}
	return nil
}

// DoAPI calls an API endpoint.
func (c *Client) DoAPI(ctx context.Context, endpoint, method, body string, debug bool) (string, error) {
	return c.doAPI(ctx, endpoint, method, body, debug, false)
}

// DoAPIWithoutRedirect never replays a mutation at a redirect target.
func (c *Client) DoAPIWithoutRedirect(ctx context.Context, endpoint, method, body string, debug bool) (string, error) {
	return c.doAPI(ctx, endpoint, method, body, debug, true)
}

func (c *Client) doAPI(ctx context.Context, endpoint, method, body string, debug, noRedirect bool) (string, error) {
	if strings.TrimSpace(method) == "" {
		return "", errs.New(errs.CategoryInternal, 0, "request method is empty").
			WithSubtype(errs.SubtypeFailedPrecondition)
	}

	url := c.baseURL + endpoint
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewReader([]byte(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return "", errs.New(errs.CategoryInternal, 0, "failed to build request: "+err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tier0-Source", "tier0-cli")
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[debug] ---------- HTTP Request ----------\n")
		fmt.Fprintf(os.Stderr, "[debug] %s %s\n", req.Method, req.URL.String())
		for key, values := range req.Header {
			for _, v := range values {
				if strings.EqualFold(key, "x-api-key") {
					v = v[:min(len(v), 8)] + "..."
				}
				fmt.Fprintf(os.Stderr, "[debug] %s: %s\n", key, v)
			}
		}
		if body != "" {
			fmt.Fprintf(os.Stderr, "[debug] Body: %s\n", redactDebugPayload(body))
		}
		fmt.Fprintf(os.Stderr, "[debug] ----------------------------------\n")
	}

	httpClient := c.client
	if noRedirect {
		copy := *c.client
		copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		httpClient = &copy
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		if errors.Is(err, ErrCredentialRedirect) {
			err = ErrCredentialRedirect
		}
		return "", errs.New(errs.CategoryNetwork, 0, "request failed: "+err.Error()).
			WithHint("Check network connectivity and the base URL.", "tier0 config").
			WithRetryable()
	}
	defer resp.Body.Close()
	if noRedirect && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return "", apierr.New(resp.StatusCode, "redirect refused for file mutation")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// HTTP status is already known even if the body is incomplete. Do not
		// include a partial body: credential responses can contain secrets.
		if resp.StatusCode >= 400 {
			return "", apierr.New(resp.StatusCode, "HTTP error response body could not be read")
		}
		return "", errs.New(errs.CategoryNetwork, 0, "failed to read response: "+err.Error()).WithHTTPStatus(resp.StatusCode).WithRetryable()
	}

	if debug {
		debugBody := redactDebugPayload(string(respBody))
		// Credential creation responses can contain one-time passwords even if
		// the response is truncated and cannot be parsed as JSON.
		if (strings.HasSuffix(endpoint, "/mqtt-auth/create") || strings.Contains(endpoint, "/assets/files")) && !json.Valid(respBody) {
			debugBody = "<debug body redacted>"
		}
		fmt.Fprintf(os.Stderr, "[debug] ---------- HTTP Response ---------\n")
		fmt.Fprintf(os.Stderr, "[debug] Status: %d %s\n", resp.StatusCode, resp.Status)
		if len(debugBody) > 4096 {
			fmt.Fprintf(os.Stderr, "[debug] Body: %s... (%d bytes truncated)\n", debugBody[:4096], len(debugBody))
		} else {
			fmt.Fprintf(os.Stderr, "[debug] Body: %s\n", debugBody)
		}
		fmt.Fprintf(os.Stderr, "[debug] ----------------------------------\n")
	}

	if resp.StatusCode >= 400 {
		return "", apierr.New(resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

func redactDebugPayload(raw string) string {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	redactDebugValue(value)
	data, err := json.Marshal(value)
	if err != nil {
		return "<debug body redacted>"
	}
	return string(data)
}

func redactDebugValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if sensitiveDebugKey(key) {
				typed[key] = "***"
				continue
			}
			redactDebugValue(child)
		}
	case []any:
		for _, child := range typed {
			redactDebugValue(child)
		}
	}
}

func sensitiveDebugKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	return strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "signature") ||
		normalized == "postfields" || normalized == "posturl" || normalized == "fileurl" ||
		normalized == "apikey" || normalized == "authorization" || normalized == "privatekey"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
