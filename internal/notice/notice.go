// Package notice provides background version-check and update notification,
// following the same _notice pattern used by lark-cli:
//
//   - JSON mode  → injects a "_notice" key into the existing JSON output
//   - Plain mode → prints a one-line hint to stderr after the command output
//
// The check is performed concurrently with the command itself (fire-and-forget
// goroutine started before the command runs). The result is collected via a
// channel at the very end, so it adds zero latency on the happy path.
package notice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

// UpdateNotice is the structured update hint embedded in JSON output.
type UpdateNotice struct {
	UpdateAvailable bool   `json:"update_available"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	Command         string `json:"command"`
	Message         string `json:"message"`
}

// Notice is the top-level _notice envelope (mirrors lark-cli structure).
type Notice struct {
	Update *UpdateNotice `json:"update,omitempty"`
}

// Checker holds the channel that receives the async check result.
// Create one with Start(), then call Emit() after the command finishes.
type Checker struct {
	ch chan *upgrade.Result
}

// Start fires off a background version check and returns a Checker.
// Call this before running the main command logic.
func Start() *Checker {
	ch := make(chan *upgrade.Result, 1)
	go func() {
		result, _ := upgrade.Check()
		ch <- result
	}()
	return &Checker{ch: ch}
}

// Emit collects the background result (non-blocking — uses whatever arrived,
// or skips silently on timeout / error) and writes the notice.
//
//   - jsonMode=true : always writes resp to stdout; if an update is available
//     the "_notice" key is injected before writing.
//   - jsonMode=false: resp is ignored (caller already printed it); an update
//     hint is printed to stderr when a newer version exists.
func (c *Checker) Emit(resp string, jsonMode bool, stdout, stderr io.Writer) {
	// Non-blocking receive: if the goroutine hasn't finished yet, skip.
	var result *upgrade.Result
	select {
	case result = <-c.ch:
	default:
	}

	hasUpdate := result != nil &&
		!result.UpToDate &&
		result.LatestVersion != "" &&
		!version.IsDev()

	if jsonMode {
		// Always write resp so callers don't need to duplicate the print.
		if hasUpdate {
			notice := buildNotice(result)
			fmt.Fprintln(stdout, injectNotice(resp, notice))
		} else {
			fmt.Fprintln(stdout, resp)
		}
		return
	}

	// Plain mode: update hint goes to stderr only.
	if hasUpdate {
		fmt.Fprintf(stderr, "\n%s\n",
			i18n.T(
				fmt.Sprintf("💡 New version available: %s → %s  Run: tier0 upgrade",
					result.CurrentVersion, result.LatestVersion),
				fmt.Sprintf("💡 发现新版本: %s → %s  运行: tier0 upgrade",
					result.CurrentVersion, result.LatestVersion),
			),
		)
	}
}

func buildNotice(result *upgrade.Result) *Notice {
	return &Notice{
		Update: &UpdateNotice{
			UpdateAvailable: true,
			CurrentVersion:  result.CurrentVersion,
			LatestVersion:   result.LatestVersion,
			Command:         "tier0 upgrade",
			Message: i18n.T(
				fmt.Sprintf("New version available: %s → %s  Run: tier0 upgrade",
					result.CurrentVersion, result.LatestVersion),
				fmt.Sprintf("发现新版本: %s → %s  运行: tier0 upgrade",
					result.CurrentVersion, result.LatestVersion),
			),
		},
	}
}

// injectNotice merges the _notice key into an existing JSON string.
// If resp is not a JSON object, it is returned unchanged.
func injectNotice(resp string, n *Notice) string {
	trimmed := strings.TrimSpace(resp)
	if !strings.HasPrefix(trimmed, "{") {
		return resp
	}

	noticeBytes, err := json.Marshal(n)
	if err != nil {
		return resp
	}

	// Remove trailing "}" and append ,"_notice":{...}}
	idx := strings.LastIndex(trimmed, "}")
	if idx < 0 {
		return resp
	}

	var buf bytes.Buffer
	buf.WriteString(trimmed[:idx])
	// If the object already has keys, add a comma separator.
	inner := strings.TrimSpace(trimmed[1:idx])
	if inner != "" {
		buf.WriteByte(',')
	}
	buf.WriteString(`"_notice":`)
	buf.Write(noticeBytes)
	buf.WriteByte('}')
	return buf.String()
}
