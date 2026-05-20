package upgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const updateCacheTTL = 24 * time.Hour

// updateState mirrors lark-cli's ~/.lark-cli/update-state.json structure.
type updateState struct {
	LatestVersion string `json:"latest_version"`
	CheckedAt     int64  `json:"checked_at"` // Unix timestamp
}

// updateStateFile returns the path to ~/.tier0/update-state.json.
func updateStateFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".tier0", "update-state.json")
}

// loadCachedState reads the cached update state.
// Returns nil (no error) when the file does not exist.
func loadCachedState() (*updateState, error) {
	data, err := os.ReadFile(updateStateFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s updateState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// saveCachedState writes the update state to disk.
func saveCachedState(s *updateState) {
	dir := filepath.Dir(updateStateFile())
	_ = os.MkdirAll(dir, 0755)
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(updateStateFile(), data, 0600)
}

// isCacheValid reports whether a cached state is still within the TTL window.
func isCacheValid(s *updateState) bool {
	if s == nil || s.LatestVersion == "" {
		return false
	}
	age := time.Since(time.Unix(s.CheckedAt, 0))
	return age < updateCacheTTL
}
