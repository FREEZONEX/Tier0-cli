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

// CheckResponse is the single entry-point for validating any API response.
// It performs two checks in order:
//
//  1. Outer envelope: {"code":N,"msg":"..."} — non-200/non-0 code signals failure.
//  2. Partial-success results: {"data":{"results":[{"error":{"code":N,...}}]}} —
//     batch endpoints return HTTP 200 even when individual items fail; this
//     detects per-item errors and surfaces them as a combined error message.
//
// All mutation commands should call CheckResponse instead of CheckOK.
func CheckResponse(resp string) error {
	// Step 1 — outer envelope check.
	var outer struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(resp), &outer); err == nil {
		if outer.Code != 0 && outer.Code != 200 {
			return apierr.New(outer.Code, resp)
		}
	}

	// Step 2 — per-item results check (partial-success batch pattern).
	var envelope struct {
		Data struct {
			Results []struct {
				Topic string `json:"topic"`
				Name  string `json:"name"`
				Error *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp), &envelope); err != nil {
		return nil
	}
	var errs []string
	for _, r := range envelope.Data.Results {
		if r.Error != nil && r.Error.Code != 0 && r.Error.Code != 200 {
			label := r.Topic
			if label == "" {
				label = r.Name
			}
			errs = append(errs, fmt.Sprintf("%s: %s (code %d)", label, r.Error.Message, r.Error.Code))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	combined := strings.Join(errs, "; ")
	return apierr.New(400, fmt.Sprintf(`{"code":400,"msg":"%s"}`, strings.ReplaceAll(combined, `"`, `'`)))
}

// CheckOK is an alias for CheckResponse kept for backward compatibility.
func CheckOK(resp string) error { return CheckResponse(resp) }

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
