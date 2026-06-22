package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// normalizeNodeREDFlowsJSON accepts any Flow JSON shape the CLI commonly sees
// and returns the pure Node-RED flows array expected by the deploy API.
func normalizeNodeREDFlowsJSON(input string, pretty bool) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("empty JSON")
	}

	var raw any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return "", err
	}

	flows, err := extractNodeREDFlows(raw)
	if err != nil {
		return "", err
	}
	if pretty {
		out, err := json.MarshalIndent(flows, "", "  ")
		if err != nil {
			return "", err
		}
		return string(out) + "\n", nil
	}
	out, err := json.Marshal(flows)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func extractNodeREDFlows(raw any) (any, error) {
	switch v := raw.(type) {
	case []any:
		return v, nil
	case map[string]any:
		if data, ok := v["data"]; ok {
			return extractNodeREDFlows(data)
		}
		if flows, ok := v["flows"]; ok {
			if _, ok := flows.([]any); !ok {
				return nil, fmt.Errorf("flows must be an array")
			}
			return flows, nil
		}
		return nil, fmt.Errorf("object must contain data or flows")
	default:
		return nil, fmt.Errorf("expected an array or object")
	}
}
