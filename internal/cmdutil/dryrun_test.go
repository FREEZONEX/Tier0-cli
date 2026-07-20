package cmdutil

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteDryRunJSONContract(t *testing.T) {
	t.Setenv("TIER0_BASE_URL", "https://example.test/")
	preview := NewRequestPreview("PATCH", "/openapi/v1/items/1", map[string]any{"name": "updated"})

	var out strings.Builder
	if err := WriteDryRun(&out, preview, true); err != nil {
		t.Fatal(err)
	}

	var envelope struct {
		OK     bool `json:"ok"`
		DryRun bool `json:"dry_run"`
		Data   struct {
			API []RequestPreview `json:"api"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out.String()), &envelope); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out.String())
	}
	if !envelope.OK || !envelope.DryRun || len(envelope.Data.API) != 1 {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	call := envelope.Data.API[0]
	if call.Method != "PATCH" || call.URL != "https://example.test/openapi/v1/items/1" {
		t.Fatalf("request was not transcribed faithfully: %#v", call)
	}
}

func TestWriteDryRunPlainOutputMarksRequestAsUnsent(t *testing.T) {
	var out strings.Builder
	preview := RequestPreview{Method: "POST", URL: "https://example.test/items", Body: map[string]any{"id": 1}}

	if err := WriteDryRun(&out, preview, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "# dry-run: request not sent") {
		t.Fatalf("missing dry-run marker: %s", out.String())
	}
	if !strings.Contains(out.String(), "POST https://example.test/items") {
		t.Fatalf("missing request line: %s", out.String())
	}
}
