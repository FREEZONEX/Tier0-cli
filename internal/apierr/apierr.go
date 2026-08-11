// Package apierr provides structured error types for tier0 CLI API calls.
// When the backend returns a non-2xx HTTP status, the CLI wraps it in an
// APIError and attaches a human-readable hint so AI agents and users know
// exactly what to do next.
package apierr

import (
	"encoding/json"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/errs"
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
		return e.Message + " — " + e.Hint
	}
	return e.Message
}

// Category returns the errs.Category corresponding to the HTTP status code.
// This is used by the central error dispatcher to select the exit code and
// the JSON "type" field.
func (e *APIError) Category() errs.Category {
	switch e.HTTPStatus {
	case 401:
		return errs.CategoryAuthentication
	case 403:
		return errs.CategoryAuthorization
	case 400, 422:
		return errs.CategoryValidation
	case 404:
		return errs.CategoryAPI
	case 500, 501, 502, 503:
		return errs.CategoryAPI
	default:
		return errs.CategoryAPI
	}
}

// ToEnvelope converts an APIError into the unified errs.Envelope for JSON output.
func (e *APIError) ToEnvelope() errs.Envelope {
	code := e.HTTPStatus
	if e.Code != 0 {
		code = e.Code
	}
	return errs.Envelope{
		OK: false,
		Error: errs.ErrorDetail{
			Type:        e.Category(),
			Code:        code,
			Message:     e.Message,
			Hint:        e.Hint,
			HintCommand: e.HintCommand,
			Retryable:   e.HTTPStatus >= 500 && e.HTTPStatus <= 599,
		},
	}
}

// Format returns the JSON representation of the error (unified envelope).
// Kept for backward compatibility; prefer ToEnvelope().Format().
func (e *APIError) Format() string {
	b, _ := json.Marshal(e.ToEnvelope())
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

// NewBusiness builds an APIError from a Tier0 business-envelope code. Business
// codes are not HTTP status codes and must never become retryable merely
// because their numeric value is greater than 500.
func NewBusiness(code int, rawBody string) *APIError {
	parsedCode, msg := parseBackendMessage(rawBody)
	if parsedCode != 0 {
		code = parsedCode
	}
	e := &APIError{
		Code:    code,
		Message: msg,
	}
	e.Hint, e.HintCommand = hintFor(0, msg)
	return e
}

// hintFor returns a hint string and optional ready-to-run command for the
// given HTTP status and error message.
func hintFor(status int, msg string) (hint, cmd string) {
	msgLower := strings.ToLower(msg)
	// 存储配额超限（后端 CodeStorageQuotaExceeded / "storage quota" 关键字）：
	// 提示删除文件或升级套餐。按消息关键字识别，兼容 status 为 HTTP 状态码
	// 或后端业务码两种调用路径（assets upload 经 ResultVO 传入的是业务码）。
	if strings.Contains(msgLower, "quota") {
		return "Storage quota exceeded for your plan. Delete files or upgrade your plan.", ""
	}
	switch status {
	case 401:
		return "API Key is missing or expired. Re-authenticate.", "tier0 login"
	case 403:
		if strings.Contains(msgLower, "read") || strings.Contains(msgLower, "readonly") {
			return "Your API Key does not have write permission. Use a key with write access.", ""
		}
		if strings.Contains(msgLower, "expired") {
			return "Presigned upload URL expired. Re-run with --resume to continue.", ""
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
		if strings.Contains(msgLower, "multipart") || strings.Contains(msgLower, "part size") {
			return "Multipart upload request rejected. Check the part size and file size, then retry.", ""
		}
		return "Bad request. Check the request body parameters.", ""
	case 500:
		return "Internal server error. Try again later or contact the platform administrator.", ""
	case 501:
		return "This capability is not enabled on the server.", ""
	}
	return "", ""
}
