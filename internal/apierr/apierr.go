// Package apierr provides structured error types for tier0 CLI API calls.
// When the backend returns a non-2xx HTTP status, the CLI wraps it in an
// APIError and attaches a human-readable hint so AI agents and users know
// exactly what to do next.
package apierr

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIError is a structured error returned by the backend API.
type APIError struct {
	// HTTPStatus is the HTTP status code (401, 403, 404 …).
	HTTPStatus int
	// Code is the backend ResultVO code (may differ from HTTPStatus).
	Code int `json:"code,omitempty"`
	// Message is the human-readable error message from the backend.
	Message string
	// Hint is the suggested next action for the caller / AI agent.
	Hint string
	// HintCommand is a ready-to-run CLI command that fixes the problem, if applicable.
	HintCommand string
}

func (e *APIError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("HTTP %d: %s — %s", e.HTTPStatus, e.Message, e.Hint)
	}
	return fmt.Sprintf("HTTP %d: %s", e.HTTPStatus, e.Message)
}

// JSONError is the JSON representation output to stderr / injected into responses.
type JSONError struct {
	OK    bool        `json:"ok"`
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries the structured error fields.
type ErrorDetail struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	Hint        string `json:"hint,omitempty"`
	HintCommand string `json:"hint_command,omitempty"`
}

// Format returns the JSON representation of the error.
func (e *APIError) Format() string {
	je := JSONError{
		OK: false,
		Error: ErrorDetail{
			Code:        e.HTTPStatus,
			Message:     e.Message,
			Hint:        e.Hint,
			HintCommand: e.HintCommand,
		},
	}
	if e.Code != 0 {
		je.Error.Code = e.Code
	}
	b, _ := json.Marshal(je)
	return string(b)
}

// Parse attempts to extract the backend ResultVO message from the raw response body.
// Falls back to the raw body if it is not valid JSON.
func parseBackendMessage(rawBody string) (code int, msg string) {
	var rv struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(rawBody), &rv); err == nil && rv.Msg != "" {
		return rv.Code, rv.Msg
	}
	return 0, strings.TrimSpace(rawBody)
}

// New builds an APIError from an HTTP status code and the raw response body,
// automatically attaching an appropriate hint.
func New(httpStatus int, rawBody string) *APIError {
	code, msg := parseBackendMessage(rawBody)
	e := &APIError{
		HTTPStatus: httpStatus,
		Code:       code,
		Message:    msg,
	}
	e.Hint, e.HintCommand = hintFor(httpStatus, msg)
	return e
}

// hintFor returns a hint string and optional ready-to-run command for the
// given HTTP status and error message.
func hintFor(status int, msg string) (hint, cmd string) {
	msgLower := strings.ToLower(msg)
	switch status {
	case 401:
		return "API Key is missing or expired. Re-authenticate.", "tier0 login"
	case 403:
		if strings.Contains(msgLower, "read") || strings.Contains(msgLower, "readonly") {
			return "Your API Key does not have write permission. Use a key with write access.", ""
		}
		return "Access denied. Check Workspace permissions or use a different API Key.", "tier0 login"
	case 404:
		if strings.Contains(msgLower, "topic") {
			return "Topic not found in UNS. Check the path or create it first.", "tier0 api /openapi/v1/uns/browse --body '{\"path\":\"/\"}'"
		}
		if strings.Contains(msgLower, "flow") {
			return "Flow not found. List available flows first.", "tier0 flow list"
		}
		return "Resource not found. Verify the path or ID.", ""
	case 400:
		if strings.Contains(msgLower, "schema") || strings.Contains(msgLower, "validation") {
			return "Request body does not match the topic schema. Check the required fields.", ""
		}
		if strings.Contains(msgLower, "api key") || strings.Contains(msgLower, "apikey") {
			return "API Key not found in request. Run login first.", "tier0 login"
		}
		return "Bad request. Check the request body parameters.", ""
	case 500:
		return "Internal server error. Try again later or contact the platform administrator.", ""
	case 501:
		return "This capability is not enabled on the server.", ""
	}
	return "", ""
}
