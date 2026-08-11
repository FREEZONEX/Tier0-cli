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
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
)

// DoAPI loads the profile, creates a client, and calls the API.
func DoAPI(ctx context.Context, endpoint, method, body string, debug bool) (string, error) {
	profile, err := config.LoadProfile()
	if err != nil {
		return "", errs.New(errs.CategoryConfig, 0,
			"failed to load config: "+err.Error()).
			WithHint("Run tier0 config to set up the CLI.", "tier0 config")
	}
	if profile.APIKey == "" {
		return "", errs.New(errs.CategoryAuthentication, 401,
			"API Key not configured.").
			WithHint("Authenticate first.", "tier0 login")
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

// CategoryOf returns the errs.Category for any error the CLI may produce.
// This is the single dispatch point used by HandleCommandError and main().
func CategoryOf(err error) errs.Category {
	if err == nil {
		return errs.CategoryAPI
	}
	var ce *errs.CLIError
	if errors.As(err, &ce) {
		return ce.Cat
	}
	var ae *apierr.APIError
	if errors.As(err, &ae) {
		return ae.Category()
	}
	var hre *highrisk.HighRiskError
	if errors.As(err, &hre) {
		return errs.CategoryConfirmation
	}
	return errs.CategoryAPI
}

// IsClassified reports whether an error already belongs to the CLI's stable
// error taxonomy. CategoryAPI alone cannot distinguish a real APIError from an
// unclassified Cobra error, so main uses this check before selecting fallback
// behavior.
func IsClassified(err error) bool {
	if err == nil {
		return false
	}
	var ce *errs.CLIError
	if errors.As(err, &ce) {
		return true
	}
	var ae *apierr.APIError
	if errors.As(err, &ae) {
		return true
	}
	var hre *highrisk.HighRiskError
	return errors.As(err, &hre)
}

// HandleCommandError writes a structured error to stderr and returns the error
// unchanged so the caller can propagate it up the Cobra RunE chain.
//
// In --json mode every error path emits a single unified JSON envelope:
//
//	{"ok":false,"error":{"type":"authentication","code":401,"message":"...","hint":"..."}}
//
// In plain mode a human-readable ✗ message with optional hint is printed.
//
// NOTE: callers must NOT also print the error themselves — this function is the
// single point of stderr output. main() only sets the exit code, it does not
// print again.
func HandleCommandError(stderr io.Writer, err error, jsonMode bool) error {
	if err == nil {
		return nil
	}

	var ce *errs.CLIError
	var ae *apierr.APIError

	switch {
	case errors.As(err, &ce):
		if jsonMode {
			fmt.Fprintln(stderr, errs.BuildEnvelope(ce).Format())
		} else {
			printPlain(stderr, ce.Msg, ce.HintText, ce.HintCmd)
		}

	case errors.As(err, &ae):
		if jsonMode {
			fmt.Fprintln(stderr, ae.ToEnvelope().Format())
		} else {
			fmt.Fprintf(stderr, "\n✗ %s\n", ae.Message)
			if ae.Hint != "" {
				fmt.Fprintf(stderr, "  → %s\n", ae.Hint)
			}
			if ae.HintCommand != "" {
				fmt.Fprintf(stderr, "  Run: %s\n", ae.HintCommand)
			}
		}

	default:
		if jsonMode {
			e := errs.New(errs.CategoryAPI, 0, err.Error())
			fmt.Fprintln(stderr, errs.BuildEnvelope(e).Format())
		} else {
			fmt.Fprintf(stderr, "\n✗ %v\n", err)
		}
	}

	return err
}

func printPlain(w io.Writer, msg, hint, hintCmd string) {
	fmt.Fprintf(w, "\n✗ %s\n", msg)
	if hint != "" {
		fmt.Fprintf(w, "  → %s\n", hint)
	}
	if hintCmd != "" {
		fmt.Fprintf(w, "  Run: %s\n", hintCmd)
	}
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
			return apierr.NewBusiness(outer.Code, resp)
		}
	}

	// Step 2 — per-item results check (partial-success batch pattern).
	var envelope struct {
		Data struct {
			Success *bool `json:"success"`
			Results []struct {
				Topic   string `json:"topic"`
				Name    string `json:"name"`
				Path    string `json:"path"`
				Success *bool  `json:"success"`
				Error   *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp), &envelope); err != nil {
		return nil
	}
	var errsSlice []string
	for _, r := range envelope.Data.Results {
		label := r.Topic
		if label == "" {
			label = r.Path
		}
		if label == "" {
			label = r.Name
		}
		if label == "" {
			label = "(item)"
		}
		if r.Error != nil && r.Error.Code != 0 && r.Error.Code != 200 {
			errsSlice = append(errsSlice, fmt.Sprintf("%s: %s (code %d)", label, r.Error.Message, r.Error.Code))
			continue
		}
		if r.Success != nil && !*r.Success {
			msg := "business success=false"
			if r.Error != nil && r.Error.Message != "" {
				msg = r.Error.Message
			}
			errsSlice = append(errsSlice, fmt.Sprintf("%s: %s", label, msg))
		}
	}
	if len(errsSlice) == 0 && envelope.Data.Success != nil && !*envelope.Data.Success {
		errsSlice = append(errsSlice, "response data.success=false")
	}
	if len(errsSlice) == 0 {
		return nil
	}
	combined := strings.Join(errsSlice, "; ")
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
