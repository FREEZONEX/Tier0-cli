package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

func responseData(response, endpoint string) (json.RawMessage, error) {
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal([]byte(response), &envelope) != nil || envelope.Code == nil || (*envelope.Code != 0 && *envelope.Code != 200) || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, fmt.Errorf("invalid %s response", endpoint)
	}
	return envelope.Data, nil
}

// ValidateWhoami accepts legacy key types while requiring an actual workspace identity.
func ValidateWhoami(response string) error {
	data, err := responseData(response, "whoami")
	if err != nil {
		return err
	}
	var identity struct {
		WorkspaceID int64 `json:"workspaceID"`
	}
	if json.Unmarshal(data, &identity) != nil || identity.WorkspaceID <= 0 {
		return fmt.Errorf("invalid whoami response")
	}
	return nil
}

// ValidateInfo requires the version supplied by Cloud and Enterprise gateways.
func ValidateInfo(response string) error {
	data, err := responseData(response, "info")
	if err != nil {
		return err
	}
	var info struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &info) != nil || strings.TrimSpace(info.Version) == "" {
		return fmt.Errorf("invalid info response")
	}
	return nil
}
