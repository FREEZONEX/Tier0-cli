// Package highrisk provides a confirmation gate for high-risk CLI operations.
//
// When a destructive command is invoked without --yes, the handler should call
// Guard() to produce a HighRiskError. main() detects this error type and exits
// with code 10, writing a structured JSON envelope to stderr so AI agents can
// surface it to users and await explicit approval before retrying with --yes.
//
// Protocol (mirrors lark-cli convention):
//   exit code 10  →  stderr contains {"ok":false,"error":{"type":"confirmation_required",...}}
//   agent MUST show risk.action + key params to the user, wait for approval,
//   then re-run the original command appending --yes.
package highrisk

import (
	"encoding/json"
	"fmt"
	"io"
)

// HighRiskError is returned by CLI handlers that require explicit user consent.
// main() detects this type and exits with code 10.
type HighRiskError struct {
	// Action is the CLI command that triggered the gate, e.g. "flow deploy".
	Action string
	// Summary is a short description of what will happen, shown to the user.
	Summary string
}

func (e *HighRiskError) Error() string {
	return fmt.Sprintf("high-risk operation requires --yes: %s", e.Action)
}

// envelope is the JSON written to stderr on exit 10.
type envelope struct {
	OK    bool        `json:"ok"`
	Error riskPayload `json:"error"`
}

type riskPayload struct {
	Type    string   `json:"type"`
	Message string   `json:"message"`
	Hint    string   `json:"hint"`
	Risk    riskMeta `json:"risk"`
}

type riskMeta struct {
	Level  string `json:"level"`
	Action string `json:"action"`
}

// WriteEnvelope writes the structured JSON confirmation-required envelope to w.
// Call this from main() when a HighRiskError is detected, before os.Exit(10).
func WriteEnvelope(w io.Writer, e *HighRiskError) {
	env := envelope{
		OK: false,
		Error: riskPayload{
			Type:    "confirmation_required",
			Message: e.Summary,
			Hint:    "add --yes to confirm",
			Risk: riskMeta{
				Level:  "high-risk-write",
				Action: e.Action,
			},
		},
	}
	b, _ := json.Marshal(env)
	fmt.Fprintln(w, string(b))
}

// Guard checks whether --yes was provided. If not, it returns a *HighRiskError
// with the given action label and human-readable summary. Pass this error up
// the call stack; main() will handle exit(10) + stderr JSON automatically.
//
//	confirmed: set to true when --yes is present in the parsed flags.
//	action:    short label shown in the envelope, e.g. "flow deploy".
//	summary:   one sentence describing the irreversible effect.
func Guard(confirmed bool, action, summary string) error {
	if confirmed {
		return nil
	}
	return &HighRiskError{Action: action, Summary: summary}
}
