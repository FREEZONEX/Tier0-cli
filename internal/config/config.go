package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Profile holds all persistent CLI settings.
type Profile struct {
	BaseURL string `json:"baseURL"`
	APIKey  string `json:"apiKey"`
	// Lang controls the CLI output language: "en" (default) or "zh".
	Lang string `json:"lang,omitempty"`
}

// configDir returns the configuration directory path.
func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".tier0")
}

// configFile returns the configuration file path.
func configFile() string {
	return filepath.Join(configDir(), "config.json")
}

// SaveProfile persists the profile to disk, merging non-zero fields
// with the existing configuration to avoid overwriting unrelated settings.
func SaveProfile(profile Profile) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	existing, _ := LoadProfile()
	if profile.BaseURL != "" {
		existing.BaseURL = profile.BaseURL
	}
	if profile.APIKey != "" {
		existing.APIKey = profile.APIKey
	}
	// Lang is updated even when set to empty string (allows reset).
	// Use the sentinel value "-" is not needed; callers set Lang explicitly.
	if profile.Lang != "" {
		existing.Lang = profile.Lang
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile(), data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// LoadProfile reads the profile from disk.
// Missing file returns an empty Profile without error.
func LoadProfile() (Profile, error) {
	var profile Profile

	data, err := os.ReadFile(configFile())
	if err != nil {
		if os.IsNotExist(err) {
			return profile, nil
		}
		return profile, fmt.Errorf("failed to read config: %w", err)
	}

	if err := json.Unmarshal(data, &profile); err != nil {
		return profile, fmt.Errorf("failed to parse config: %w", err)
	}
	return profile, nil
}
