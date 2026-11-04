package auth

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

type managedConfirmFunc func(context.Context, string, string, string) (bool, bool, error)

func (f managedConfirmFunc) ConfirmManaged(ctx context.Context, url, setup, key string) (bool, bool, error) {
	return f(ctx, url, setup, key)
}

func TestManagedConfirmationRecoversLostResponse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	if err := config.SaveProfile(config.Profile{BaseURL: "https://old", APIKey: "old-key"}); err != nil {
		t.Fatal(err)
	}
	pending := config.ManagedAuthPending{BaseURL: "https://new", SetupCode: "setup", PickupVerifier: "verifier"}
	if err := config.BeginManagedAuth(pending); err != nil {
		t.Fatal(err)
	}
	if err := ResumeManagedAuth(context.Background(), nil); !errors.Is(err, ErrManagedAwaitingKey) {
		t.Fatalf("waiting state: %v", err)
	}
	attempts := 0
	client := managedConfirmFunc(func(_ context.Context, url, setup, key string) (bool, bool, error) {
		attempts++
		if url != pending.BaseURL || setup != pending.SetupCode || key != "new-key" {
			t.Fatal("confirmation did not use the saved attempt")
		}
		if attempts == 1 {
			return false, false, fmt.Errorf("response lost after accepting %s", key)
		}
		return true, false, nil
	})
	err := SaveAndConfirmManaged(context.Background(), pending.SetupCode, "new-key", client)
	if !errors.Is(err, ErrManagedConfirmUncertain) || strings.Contains(err.Error(), "new-key") {
		t.Fatalf("uncertain confirmation must not leak key: %v", err)
	}
	profile, err := config.LoadProfile()
	if err != nil || profile.APIKey != "old-key" {
		t.Fatalf("unconfirmed key replaced active profile: %v", err)
	}
	stored, err := config.LoadManagedAuth()
	if err != nil || stored == nil || stored.APIKey != "new-key" {
		t.Fatalf("proof not retained after lost response: %v", err)
	}
	if err := ResumeManagedAuth(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	profile, err = config.LoadProfile()
	if err != nil || profile.APIKey != "new-key" || profile.BaseURL != pending.BaseURL || attempts != 2 {
		t.Fatalf("confirmed key not promoted: attempts=%d err=%v", attempts, err)
	}
}

func TestManagedConfirmationRejectsOnlyDefinitiveFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	if err := config.SaveProfile(config.Profile{APIKey: "old-key"}); err != nil {
		t.Fatal(err)
	}
	if err := config.BeginManagedAuth(config.ManagedAuthPending{BaseURL: "https://new", SetupCode: "setup", PickupVerifier: "verifier"}); err != nil {
		t.Fatal(err)
	}
	ambiguous := managedConfirmFunc(func(context.Context, string, string, string) (bool, bool, error) { return true, true, nil })
	if err := SaveAndConfirmManaged(context.Background(), "setup", "key", ambiguous); !errors.Is(err, ErrManagedConfirmUncertain) {
		t.Fatalf("ambiguous result: %v", err)
	}
	if stored, err := config.LoadManagedAuth(); err != nil || stored == nil {
		t.Fatalf("ambiguous result discarded recovery state: %v", err)
	}
	rejected := managedConfirmFunc(func(context.Context, string, string, string) (bool, bool, error) { return false, true, nil })
	if err := ResumeManagedAuth(context.Background(), rejected); !errors.Is(err, ErrManagedRejected) {
		t.Fatalf("rejected result: %v", err)
	}
	if stored, err := config.LoadManagedAuth(); err != nil || stored != nil {
		t.Fatalf("rejected state remains: %v", err)
	}
	profile, err := config.LoadProfile()
	if err != nil || profile.APIKey != "old-key" {
		t.Fatalf("rejection changed existing key: %v", err)
	}
}
