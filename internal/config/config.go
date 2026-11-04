package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	return withProfileLock(func() error {
		existing, err := LoadProfile()
		if err != nil {
			return err
		}
		if profile.BaseURL != "" {
			existing.BaseURL = profile.BaseURL
		}
		if profile.APIKey != "" {
			existing.APIKey = profile.APIKey
		}
		if profile.Lang != "" {
			existing.Lang = profile.Lang
		}
		return writeProfile(existing)
	})
}

func writeProfile(profile Profile) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := writeProfileAtomically(configFile(), data); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// writeProfileAtomically prevents a failed or interrupted write from truncating
// the previous profile. The temporary file is created on the same filesystem.
func writeProfileAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	// Directory handles cannot be synced portably on Windows.
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
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
