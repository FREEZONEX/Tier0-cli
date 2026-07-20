package cmdutil

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// RequestPreview is the stable representation of an HTTP request that would
// be sent by a dry-run. Credentials and headers are intentionally excluded.
type RequestPreview struct {
	Method string      `json:"method"`
	URL    string      `json:"url"`
	Body   interface{} `json:"body,omitempty"`
}

type dryRunData struct {
	API []RequestPreview `json:"api"`
}

type dryRunEnvelope struct {
	OK     bool       `json:"ok"`
	DryRun bool       `json:"dry_run"`
	Data   dryRunData `json:"data"`
}

// NewRequestPreview builds a credential-free preview using the same base URL
// resolution as real API calls.
func NewRequestPreview(method, endpoint string, body interface{}) RequestPreview {
	return RequestPreview{
		Method: method,
		URL:    ResolveBaseURL("") + endpoint,
		Body:   body,
	}
}

// WriteDryRun emits the common request-preview contract. JSON mode always
// returns a success envelope; plain mode carries an explicit marker on stdout.
func WriteDryRun(w io.Writer, preview RequestPreview, jsonMode bool) error {
	if strings.TrimSpace(preview.Method) == "" {
		return fmt.Errorf("dry-run request method is empty")
	}
	if strings.TrimSpace(preview.URL) == "" {
		return fmt.Errorf("dry-run request URL is empty")
	}

	if jsonMode {
		return json.NewEncoder(w).Encode(dryRunEnvelope{
			OK:     true,
			DryRun: true,
			Data: dryRunData{
				API: []RequestPreview{preview},
			},
		})
	}

	if _, err := fmt.Fprintln(w, "# dry-run: request not sent"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s %s\n", preview.Method, preview.URL); err != nil {
		return err
	}
	if preview.Body == nil {
		return nil
	}
	body, err := json.Marshal(preview.Body)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "  %s\n", body)
	return err
}
