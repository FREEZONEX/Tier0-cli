package cmd

import (
	"encoding/json"
	"testing"
)

func TestNormalizeNodeREDFlowsJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "pure flows array",
			input: `[{"id":"tab-1","type":"tab","label":"Flow"}]`,
		},
		{
			name:  "flow data object",
			input: `{"rev":"abc","flows":[{"id":"tab-1","type":"tab","label":"Flow"}]}`,
		},
		{
			name:  "api envelope",
			input: `{"code":200,"msg":"success","data":{"rev":"abc","flows":[{"id":"tab-1","type":"tab","label":"Flow"}]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeNodeREDFlowsJSON(tt.input, false)
			if err != nil {
				t.Fatalf("normalizeNodeREDFlowsJSON returned error: %v", err)
			}
			var flows []map[string]any
			if err := json.Unmarshal([]byte(got), &flows); err != nil {
				t.Fatalf("output is not a flows array: %v", err)
			}
			if len(flows) != 1 || flows[0]["id"] != "tab-1" {
				t.Fatalf("unexpected flows output: %s", got)
			}
		})
	}
}

func TestNormalizeNodeREDFlowsJSONRejectsInvalidShapes(t *testing.T) {
	tests := []string{
		``,
		`{"code":200,"msg":"success"}`,
		`{"flows":{"id":"not-array"}}`,
		`"not an object"`,
	}

	for _, input := range tests {
		if _, err := normalizeNodeREDFlowsJSON(input, false); err == nil {
			t.Fatalf("expected error for input %q", input)
		}
	}
}
