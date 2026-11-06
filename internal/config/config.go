package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	existing, _ := LoadProfile()
	if profile.BaseURL != "" {
		if profile.APIKey == "" && !SameOrigin(existing.BaseURL, profile.BaseURL) {
			existing.APIKey = ""
		}
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

	return ReplaceProfile(existing)
}

// ReplaceProfile persists an entire profile, including an explicitly cleared key.
func ReplaceProfile(profile Profile) error {
	if err := os.MkdirAll(configDir(), 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile(), data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// Origin returns the canonical HTTP origin of a base URL.
func Origin(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.User != nil || u.Opaque != "" || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid base URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("invalid base URL")
	}
	port := u.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return scheme + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port), nil
}

func SameOrigin(a, b string) bool {
	x, err := Origin(a)
	if err != nil {
		return false
	}
	y, err := Origin(b)
	return err == nil && x == y
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
