// Package errs defines the structured error taxonomy for the Tier0 CLI.
//
// Every failure surfaces as a typed CLIError with a stable Category that maps
// to a well-known shell exit code. JSON consumers parse the unified envelope;
// shell scripts branch on the exit code; humans read the hint.
//
// Wire format (stderr, --json mode):
//
//	{"ok":false,"error":{"type":"authentication","code":401,"message":"...","hint":"...","hint_command":"tier0 login"}}
package errs

import "encoding/json"

// Category identifies the failure domain. The value is written directly into
// the JSON "type" field and is wire-stable — do not rename existing constants.
type Category string

const (
	// CategoryValidation covers malformed user input: bad flag values, missing
	// required arguments, mutually exclusive flags.
	CategoryValidation Category = "validation" // exit 2

	// CategoryAuthentication covers missing or expired credentials.
	CategoryAuthentication Category = "authentication" // exit 3

	// CategoryAuthorization covers valid credentials that lack the required
	// workspace or resource permission.
	CategoryAuthorization Category = "authorization" // exit 3

	// CategoryConfig covers local configuration problems: missing config file,
	// unset base URL, unset API key before login.
	CategoryConfig Category = "config" // exit 3

	// CategoryNetwork covers transport-layer failures: DNS resolution, TCP
	// connection refused, request timeout, TLS errors.
	CategoryNetwork Category = "network" // exit 4

	// CategoryAPI covers server-side errors: HTTP 5xx, backend business-code
	// failures that do not fit a more specific category.
	CategoryAPI Category = "api" // exit 1

	// CategoryInternal covers unexpected CLI-side failures: JSON decode errors,
	// broken invariants, programming mistakes.
	CategoryInternal Category = "internal" // exit 5

	// CategoryConfirmation signals that a high-risk operation requires explicit
	// --yes consent before it may proceed.
	CategoryConfirmation Category = "confirmation" // exit 10
)

// ExitCode maps a Category to its canonical shell exit code.
//
//	2  validation      — caller passed bad input
//	3  authentication  — re-authenticate (tier0 login)
//	3  authorization   — contact workspace admin
//	3  config          — run tier0 config
//	4  network         — transient; safe to retry
//	1  api             — server-side failure
//	5  internal        — CLI bug; file an issue
//	10 confirmation    — re-run with --yes
func ExitCode(c Category) int {
	switch c {
	case CategoryValidation:
		return 2
	case CategoryAuthentication, CategoryAuthorization, CategoryConfig:
		return 3
	case CategoryNetwork:
		return 4
	case CategoryInternal:
		return 5
	case CategoryConfirmation:
		return 10
	default:
		return 1
	}
}

// CLIError is a typed, categorized error for the Tier0 CLI.
// Use New() or the package-level constructors to create one.
type CLIError struct {
	Cat         Category
	Code        int
	Msg         string
	HintText    string
	HintCmd     string
	IsRetryable bool
}

func (e *CLIError) Error() string {
	if e.HintText != "" {
		return e.Msg + " — " + e.HintText
	}
	return e.Msg
}

// Category satisfies the Categorizer interface.
func (e *CLIError) Category() Category { return e.Cat }

// New creates a CLIError with the given category, numeric code, and message.
// Attach hints with WithHint; mark transient failures with WithRetryable.
func New(cat Category, code int, msg string) *CLIError {
	return &CLIError{Cat: cat, Code: code, Msg: msg}
}

// WithHint attaches a human-readable recovery hint and an optional ready-to-run
// CLI command. Both are carried in the JSON envelope and shown in plain mode.
func (e *CLIError) WithHint(hint, cmd string) *CLIError {
	e.HintText = hint
	e.HintCmd = cmd
	return e
}

// WithRetryable marks the error as safe to retry automatically (e.g. transient
// network blip). Scripts may use this flag to drive retry loops.
func (e *CLIError) WithRetryable() *CLIError {
	e.IsRetryable = true
	return e
}

// ── JSON envelope ────────────────────────────────────────────────────────────

// Envelope is the unified JSON error structure written to stderr on failure.
// All error paths must produce exactly this shape so that AI agents and scripts
// can parse a single, stable contract.
type Envelope struct {
	OK    bool        `json:"ok"`
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries the structured error fields inside the envelope.
type ErrorDetail struct {
	Type        Category `json:"type"`
	Code        int      `json:"code,omitempty"`
	Message     string   `json:"message"`
	Hint        string   `json:"hint,omitempty"`
	HintCommand string   `json:"hint_command,omitempty"`
	Retryable   bool     `json:"retryable,omitempty"`
}

// Format renders an Envelope as a single-line JSON string.
func (env Envelope) Format() string {
	b, _ := json.Marshal(env)
	return string(b)
}

// BuildEnvelope constructs a JSON Envelope from a CLIError.
func BuildEnvelope(e *CLIError) Envelope {
	return Envelope{
		OK: false,
		Error: ErrorDetail{
			Type:        e.Cat,
			Code:        e.Code,
			Message:     e.Msg,
			Hint:        e.HintText,
			HintCommand: e.HintCmd,
			Retryable:   e.IsRetryable,
		},
	}
}
