package auth

import (
	"context"
	"errors"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

var (
	ErrManagedAwaitingKey      = errors.New("managed authorization is awaiting its key")
	ErrManagedRejected         = errors.New("managed authorization expired or was rejected")
	ErrManagedConfirmUncertain = errors.New("managed authorization confirmation is temporarily unavailable")
)

// ManagedConfirmationClient must report rejected only for a definitive server verdict.
type ManagedConfirmationClient interface {
	ConfirmManaged(ctx context.Context, baseURL, setupCode, apiKey string) (confirmed, rejected bool, err error)
}

// SaveAndConfirmManaged persists the proof before attempting confirmation.
func SaveAndConfirmManaged(ctx context.Context, setupCode, apiKey string, client ManagedConfirmationClient) error {
	if err := config.SaveManagedAuthKey(setupCode, apiKey); err != nil {
		return err
	}
	return ResumeManagedAuth(ctx, client)
}

// ResumeManagedAuth retries confirmation after a lost response or process restart.
func ResumeManagedAuth(ctx context.Context, client ManagedConfirmationClient) error {
	pending, err := config.LoadManagedAuth()
	if err != nil {
		return err
	}
	if pending == nil {
		return nil
	}
	if pending.APIKey == "" {
		return ErrManagedAwaitingKey
	}
	if client == nil {
		return ErrManagedConfirmUncertain
	}
	confirmed, rejected, err := client.ConfirmManaged(ctx, pending.BaseURL, pending.SetupCode, pending.APIKey)
	if err != nil || confirmed == rejected {
		// Transport errors can include request data; keep credentials out of error messages.
		return ErrManagedConfirmUncertain
	}
	if rejected {
		if err := config.DiscardManagedAuth(pending.SetupCode); err != nil {
			return err
		}
		return ErrManagedRejected
	}
	return config.FinishManagedAuth(pending.SetupCode, pending.APIKey)
}
