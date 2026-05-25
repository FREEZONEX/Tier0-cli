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
