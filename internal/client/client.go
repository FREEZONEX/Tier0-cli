package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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
	if method == "" {
		method = http.MethodPost
	}

	url := c.baseURL + endpoint
	body = fixJSON(body)
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
			fmt.Fprintf(os.Stderr, "[debug] Body: %s\n", body)
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
		fmt.Fprintf(os.Stderr, "[debug] ---------- HTTP Response ---------\n")
		fmt.Fprintf(os.Stderr, "[debug] Status: %d %s\n", resp.StatusCode, resp.Status)
		if len(respBody) > 4096 {
			fmt.Fprintf(os.Stderr, "[debug] Body: %s... (%d bytes truncated)\n", string(respBody[:4096]), len(respBody))
		} else {
			fmt.Fprintf(os.Stderr, "[debug] Body: %s\n", string(respBody))
		}
		fmt.Fprintf(os.Stderr, "[debug] ----------------------------------\n")
	}

	if resp.StatusCode >= 400 {
		return "", apierr.New(resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

// fixJSON attempts to repair JSON with missing quotes, which commonly happens
// in shells such as PowerShell. For example, {path:/} becomes {"path":"/"}.
func fixJSON(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return body
	}
	// Already valid JSON; no repair needed.
	if json.Valid([]byte(trimmed)) {
		return trimmed
	}

	// Repair objects by quoting unquoted keys and string values.
	// Step 1: quote keys, for example {key: becomes {"key":
	keyRe := regexp.MustCompile(`([{,]\s*)([A-Za-z_][A-Za-z0-9_]*)\s*:`)
	fixed := keyRe.ReplaceAllString(trimmed, `$1"$2":`)

	// Step 2: quote unquoted string values, excluding numbers, booleans, and null.
	valRe := regexp.MustCompile(`(:\s*)([^"{}\[\],\s\d][^,}\]]*)([,}\]])`)
	fixed = valRe.ReplaceAllString(fixed, `$1"$2"$3`)

	// Step 3: handle unquoted strings in arrays.
	arrValRe := regexp.MustCompile(`([,\[]\s*)([^"{}\[\],\s\d][^,\]]*)([,\]])`)
	fixed = arrValRe.ReplaceAllString(fixed, `$1"$2"$3`)

	// Fallback: return the original value if the repaired result is still invalid.
	if json.Valid([]byte(fixed)) {
		return fixed
	}
	return trimmed
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
