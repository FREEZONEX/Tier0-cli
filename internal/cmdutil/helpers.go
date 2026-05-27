// Package cmdutil provides shared utilities for CLI commands.
package cmdutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/client"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
)

// DoAPI loads the profile, creates a client, and calls the API.
func DoAPI(ctx context.Context, endpoint, method, body string, debug bool) (string, error) {
	profile, err := config.LoadProfile()
	if err != nil {
		return "", fmt.Errorf(i18n.T("failed to load config: %w", "加载配置失败: %w"), err)
	}
	if profile.APIKey == "" {
		return "", apierr.New(401, `{"code":401,"msg":"API Key not found"}`)
	}
	c := client.New(profile.BaseURL, profile.APIKey)
	return c.DoAPI(ctx, endpoint, method, body, debug)
}

// ResolveBaseURL resolves the effective base URL from arg, env, or config.
func ResolveBaseURL(baseURLArg string) string {
	if baseURLArg != "" {
		return strings.TrimRight(baseURLArg, "/")
	}
	if envURL := os.Getenv("TIER0_BASE_URL"); envURL != "" {
		return strings.TrimRight(envURL, "/")
	}
	profile, _ := config.LoadProfile()
	if profile.BaseURL != "" {
		return strings.TrimRight(profile.BaseURL, "/")
	}
	return "https://tier0.dev"
}

// HandleCommandError writes the error to stderr and returns it.
// JSON mode produces structured error envelopes; plain mode prints human-readable hints.
func HandleCommandError(stderr io.Writer, err error, jsonMode bool) error {
	if err == nil {
		return nil
	}
	var ae *apierr.APIError
	if errors.As(err, &ae) {
		if jsonMode {
			fmt.Fprintln(stderr, ae.Format())
		} else {
			fmt.Fprintf(stderr, "\n✗ %s\n", ae.Message)
			if ae.Hint != "" {
				fmt.Fprintf(stderr, i18n.T("  → %s\n", "  → %s\n"), ae.Hint)
			}
			if ae.HintCommand != "" {
				fmt.Fprintf(stderr, i18n.T("  Run: %s\n", "  执行: %s\n"), ae.HintCommand)
			}
		}
		return err
	}
	if jsonMode {
		msg := strings.ReplaceAll(err.Error(), `"`, `\"`)
		fmt.Fprintf(stderr, `{"ok":false,"error":{"code":0,"message":"%s"}}`+"\n", msg)
	} else {
		fmt.Fprintf(stderr, "\n✗ %v\n", err)
	}
	return err
}

// JSONString builds a JSON string from a value, or returns "{}" on failure.
func JSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// CheckOK parses the standard backend envelope {"code":N,"msg":"..."} and returns
// an error if the backend signals a business-logic failure.
//
// The backend (unitedrhino/go-zero convention) uses:
//   - code 200  → success
//   - code 0    → field absent / zero-valued, treat as success
//   - anything else (400, 500, …) → failure
//   - data.success false → bulk operation has failed items
//
// HTTP-level errors (4xx/5xx status) are already handled by DoAPI; this catches
// the cases where the server responds HTTP 200 but embeds an error in the body.
func CheckOK(resp string) error {
	var rv struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Success *bool            `json:"success"`
			Results []bulkResultItem `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp), &rv); err != nil {
		return nil // not a standard envelope, assume OK
	}
	if rv.Code != 0 && rv.Code != 200 {
		return apierr.New(rv.Code, resp)
	}
	if rv.Data.Success != nil && !*rv.Data.Success {
		code, msg := firstBulkError(rv.Data.Results)
		if msg == "" {
			msg = "batch operation failed"
		}
		if code == 0 {
			code = 400
		}
		return apierr.New(code, JSONString(map[string]any{"code": code, "msg": msg}))
	}
	return nil
}

type bulkResultItem struct {
	Success *bool  `json:"success"`
	Topic   string `json:"topic"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func firstBulkError(results []bulkResultItem) (int, string) {
	for _, item := range results {
		if item.Success != nil && *item.Success {
			continue
		}
		if item.Error == nil {
			continue
		}
		msg := item.Error.Message
		if item.Topic != "" && msg != "" {
			msg = item.Topic + ": " + msg
		}
		return item.Error.Code, msg
	}
	return 0, ""
}

// ExtractData unwraps the standard backend envelope {"code":N,"msg":"...","data":{...}}
// and returns the raw JSON of the "data" field.
// If the response has no "data" field the original string is returned unchanged,
// so callers that receive a flat response still work correctly.
func ExtractData(resp string) string {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp), &envelope); err == nil && len(envelope.Data) > 0 {
		return string(envelope.Data)
	}
	return resp
}
