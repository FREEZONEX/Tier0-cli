// Package mqttprofile persists MQTT credentials created through Tier0.
package mqttprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const fileVersion = 1

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Credential is a locally saved MQTT credential. Password is intentionally
// omitted from command diagnostics and must never be logged.
type Credential struct {
	Version                     int       `json:"version"`
	ID                          int64     `json:"id"`
	Name                        string    `json:"name"`
	BaseURL                     string    `json:"baseURL"`
	Broker                      string    `json:"broker"`
	ClientID                    string    `json:"clientID"`
	Username                    string    `json:"username"`
	Password                    string    `json:"password"`
	ClientIDRandomSuffixEnabled bool      `json:"clientIDRandomSuffixEnabled"`
	CreatedAt                   time.Time `json:"createdAt"`
}

// Store manages one credential file per profile name.
type Store struct {
	Dir string
}

// DefaultStore returns the store under ~/.tier0/mqtt.
func DefaultStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home: %w", err)
	}
	return &Store{Dir: filepath.Join(home, ".tier0", "mqtt")}, nil
}

// ValidateName rejects profile names that could escape the credential folder.
func ValidateName(name string) error {
	if !profileNamePattern.MatchString(strings.TrimSpace(name)) {
		return fmt.Errorf("profile name must start with a letter or digit and contain only letters, digits, '.', '_', or '-' (max 64 characters)")
	}
	return nil
}

// Prepare validates that a new profile can be saved before a remote credential
// is created, preventing avoidable orphaned server credentials.
func (s *Store) Prepare(name string) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("create MQTT credential directory: %w", err)
	}
	if err := os.Chmod(s.Dir, 0o700); err != nil {
		return fmt.Errorf("secure MQTT credential directory: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("MQTT credential profile %q already exists", name)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect MQTT credential profile: %w", err)
	}

	probe, err := os.CreateTemp(s.Dir, ".write-test-*")
	if err != nil {
		return fmt.Errorf("MQTT credential directory is not writable: %w", err)
	}
	probePath := probe.Name()
	if closeErr := probe.Close(); closeErr != nil {
		_ = os.Remove(probePath)
		return fmt.Errorf("close MQTT credential write test: %w", closeErr)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("clean up MQTT credential write test: %w", err)
	}
	return nil
}

// Save atomically writes a new credential profile with user-only permissions.
func (s *Store) Save(name string, credential Credential) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if credential.ID <= 0 || strings.TrimSpace(credential.Broker) == "" ||
		strings.TrimSpace(credential.ClientID) == "" || strings.TrimSpace(credential.Username) == "" ||
		credential.Password == "" {
		return errors.New("credential is incomplete")
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("create MQTT credential directory: %w", err)
	}
	if err := os.Chmod(s.Dir, 0o700); err != nil {
		return fmt.Errorf("secure MQTT credential directory: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("MQTT credential profile %q already exists", name)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect MQTT credential profile: %w", err)
	}

	credential.Version = fileVersion
	if credential.CreatedAt.IsZero() {
		credential.CreatedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(credential, "", "  ")
	if err != nil {
		return fmt.Errorf("encode MQTT credential: %w", err)
	}

	tmp, err := os.CreateTemp(s.Dir, ".credential-*.tmp")
	if err != nil {
		return fmt.Errorf("create MQTT credential temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure MQTT credential temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write MQTT credential: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync MQTT credential: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close MQTT credential: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install MQTT credential: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

// Load reads a named credential profile.
func (s *Store) Load(name string) (Credential, error) {
	var credential Credential
	path, err := s.path(name)
	if err != nil {
		return credential, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return credential, fmt.Errorf("MQTT credential profile %q does not exist", name)
		}
		return credential, fmt.Errorf("read MQTT credential profile: %w", err)
	}
	if err := json.Unmarshal(data, &credential); err != nil {
		return credential, fmt.Errorf("parse MQTT credential profile: %w", err)
	}
	if credential.Version != fileVersion {
		return credential, fmt.Errorf("unsupported MQTT credential profile version %d", credential.Version)
	}
	if credential.Password == "" || credential.ClientID == "" || credential.Username == "" {
		return credential, errors.New("MQTT credential profile is incomplete")
	}
	return credential, nil
}

// Delete removes only the named local credential profile.
func (s *Store) Delete(name string) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete MQTT credential profile: %w", err)
	}
	return nil
}

func (s *Store) path(name string) (string, error) {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if strings.TrimSpace(s.Dir) == "" {
		return "", errors.New("MQTT credential directory is empty")
	}
	return filepath.Join(s.Dir, name+".json"), nil
}
