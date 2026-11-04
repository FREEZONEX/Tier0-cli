package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"
)

var ErrManagedAuthConflict = errors.New("managed authorization is missing, changed, or conflicts with the current profile")

// ManagedAuthPending is a recoverable attempt, kept separate from the active profile.
type ManagedAuthPending struct {
	BaseURL        string `json:"baseURL"`
	SetupCode      string `json:"setupCode"`
	PickupVerifier string `json:"pickupVerifier"`
	APIKey         string `json:"apiKey,omitempty"`
	ProfileDigest  string `json:"profileDigest"`
}

func (ManagedAuthPending) String() string   { return "[managed authorization redacted]" }
func (ManagedAuthPending) GoString() string { return "[managed authorization redacted]" }
func (ManagedAuthPending) MarshalJSON() ([]byte, error) {
	return []byte(`"[managed authorization redacted]"`), nil
}

// The disk-only type deliberately does not implement the logging-safe marshaler.
type managedAuthPendingDisk ManagedAuthPending

func managedPendingFile() string { return filepath.Join(configDir(), "managed-auth-pending.json") }

func withProfileLock(fn func() error) error {
	if err := os.MkdirAll(configDir(), 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	lock := flock.New(filepath.Join(configDir(), "config.lock"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to lock config: %w", err)
	}
	if !ok {
		return errors.New("timed out waiting for config lock")
	}
	defer lock.Unlock()
	return fn()
}

func profileDigest(profile Profile) (string, error) {
	encoded, err := json.Marshal(profile)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func readManagedPending() (*ManagedAuthPending, error) {
	data, err := os.ReadFile(managedPendingFile())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read pending authorization: %w", err)
	}
	var pending managedAuthPendingDisk
	if err := json.Unmarshal(data, &pending); err != nil {
		return nil, errors.New("pending authorization file is invalid")
	}
	if pending.BaseURL == "" || pending.SetupCode == "" || pending.PickupVerifier == "" || pending.ProfileDigest == "" {
		return nil, errors.New("pending authorization file is invalid")
	}
	result := ManagedAuthPending(pending)
	return &result, nil
}

func writeManagedPending(pending ManagedAuthPending) error {
	data, err := json.MarshalIndent(managedAuthPendingDisk(pending), "", "  ")
	if err != nil {
		return errors.New("failed to encode pending authorization")
	}
	if err := writeProfileAtomically(managedPendingFile(), data); err != nil {
		return fmt.Errorf("failed to save pending authorization: %w", err)
	}
	return nil
}

func LoadManagedAuth() (pending *ManagedAuthPending, err error) {
	err = withProfileLock(func() error {
		var readErr error
		pending, readErr = readManagedPending()
		return readErr
	})
	return pending, err
}

// BeginManagedAuth persists setup and verifier before opening the browser.
func BeginManagedAuth(pending ManagedAuthPending) error {
	if pending.BaseURL == "" || pending.SetupCode == "" || pending.PickupVerifier == "" || pending.APIKey != "" || pending.ProfileDigest != "" {
		return ErrManagedAuthConflict
	}
	return withProfileLock(func() error {
		current, err := readManagedPending()
		if err != nil {
			return err
		}
		if current != nil {
			if current.BaseURL == pending.BaseURL && current.SetupCode == pending.SetupCode && current.PickupVerifier == pending.PickupVerifier {
				return nil
			}
			return ErrManagedAuthConflict
		}
		profile, err := LoadProfile()
		if err != nil {
			return err
		}
		pending.ProfileDigest, err = profileDigest(profile)
		if err != nil {
			return err
		}
		return writeManagedPending(pending)
	})
}

// SaveManagedAuthKey must succeed before sending a confirmation request.
func SaveManagedAuthKey(setupCode, apiKey string) error {
	if setupCode == "" || apiKey == "" {
		return ErrManagedAuthConflict
	}
	return withProfileLock(func() error {
		pending, err := readManagedPending()
		if err != nil {
			return err
		}
		if pending == nil || pending.SetupCode != setupCode || (pending.APIKey != "" && pending.APIKey != apiKey) {
			return ErrManagedAuthConflict
		}
		pending.APIKey = apiKey
		return writeManagedPending(*pending)
	})
}

// FinishManagedAuth promotes only a confirmed key; changed profiles cause a conflict.
func FinishManagedAuth(setupCode, apiKey string) error {
	if setupCode == "" || apiKey == "" {
		return ErrManagedAuthConflict
	}
	return withProfileLock(func() error {
		pending, err := readManagedPending()
		if err != nil {
			return err
		}
		if pending == nil || pending.SetupCode != setupCode || pending.APIKey != apiKey {
			return ErrManagedAuthConflict
		}
		profile, err := LoadProfile()
		if err != nil {
			return err
		}
		if profile.BaseURL != pending.BaseURL || profile.APIKey != pending.APIKey {
			digest, err := profileDigest(profile)
			if err != nil {
				return err
			}
			if digest != pending.ProfileDigest {
				return ErrManagedAuthConflict
			}
			profile.BaseURL, profile.APIKey = pending.BaseURL, pending.APIKey
			if err := writeProfile(profile); err != nil {
				return err
			}
		}
		return removeManagedPending()
	})
}

// DiscardManagedAuth is for a server-confirmed permanent failure only.
func DiscardManagedAuth(setupCode string) error {
	if setupCode == "" {
		return ErrManagedAuthConflict
	}
	return withProfileLock(func() error {
		pending, err := readManagedPending()
		if err != nil {
			return err
		}
		if pending == nil || pending.SetupCode != setupCode {
			return ErrManagedAuthConflict
		}
		return removeManagedPending()
	})
}

func removeManagedPending() error {
	if err := os.Remove(managedPendingFile()); err != nil {
		return fmt.Errorf("failed to remove pending authorization: %w", err)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(configDir())
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
