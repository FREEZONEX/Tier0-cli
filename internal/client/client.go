package client

import (
	"bytes"
	"context"
	"encoding/json"
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

// New creates an API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// DoAPI calls an API endpoint.
func (c *Client) DoAPI(ctx context.Context, endpoint, method, body string, debug bool) (string, error) {
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

	resp, err := c.client.Do(req)
	if err != nil {
		return "", errs.New(errs.CategoryNetwork, 0, "request failed: "+err.Error()).
			WithHint("Check network connectivity and the base URL.", "tier0 config").
			WithRetryable()
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errs.New(errs.CategoryNetwork, 0, "failed to read response: "+err.Error()).WithRetryable()
	}

	if debug {
		debugBody := redactDebugPayload(string(respBody))
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
		normalized == "apikey" || normalized == "authorization" || normalized == "privatekey"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
