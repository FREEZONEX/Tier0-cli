package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func managedTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
}

func TestManagedAuthRecoveryAndPromotion(t *testing.T) {
	managedTestHome(t)
	old := Profile{BaseURL: "https://old.example", APIKey: "old-key", Lang: "zh"}
	if err := SaveProfile(old); err != nil {
		t.Fatal(err)
	}
	want := ManagedAuthPending{BaseURL: "https://new.example", SetupCode: "setup-secret", PickupVerifier: "verifier-secret"}
	if err := BeginManagedAuth(want); err != nil {
		t.Fatal(err)
	}
	if err := BeginManagedAuth(want); err != nil {
		t.Fatal(err)
	}
	profile, err := LoadProfile()
	if err != nil || profile != old {
		t.Fatalf("unconfirmed login changed active profile: %+v %v", profile, err)
	}
	if err := SaveManagedAuthKey(want.SetupCode, "new-key"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(managedPendingFile())
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("pending file permissions are not private: %v", err)
	}
	// Reload from disk to model a new process resuming after an interrupted response.
	resumed, err := LoadManagedAuth()
	if err != nil || resumed == nil || resumed.APIKey != "new-key" || resumed.PickupVerifier != want.PickupVerifier {
		t.Fatalf("pending login did not survive restart: %v", err)
	}
	encoded, err := json.Marshal(resumed)
	if err != nil || string(encoded) != `"[managed authorization redacted]"` || strings.Contains(fmt.Sprintf("%+v %#v", resumed, resumed), "new-key") {
		t.Fatal("pending credentials are not redacted in structured output")
	}
	if err := FinishManagedAuth(want.SetupCode, "wrong-key"); !errors.Is(err, ErrManagedAuthConflict) {
		t.Fatalf("wrong confirmation promoted: %v", err)
	}
	if err := FinishManagedAuth(want.SetupCode, "new-key"); err != nil {
		t.Fatal(err)
	}
	profile, err = LoadProfile()
	if err != nil || profile.BaseURL != want.BaseURL || profile.APIKey != "new-key" || profile.Lang != old.Lang {
		t.Fatalf("confirmed profile incorrect: %v", err)
	}
	pending, err := LoadManagedAuth()
	if err != nil || pending != nil {
		t.Fatalf("completed pending state remains: %v", err)
	}
}

func TestManagedAuthProcessCrashRecovery(t *testing.T) {
	if os.Getenv("CLI_MANAGED_CRASH_CHILD") == "1" {
		pending := ManagedAuthPending{BaseURL: "https://new.example", SetupCode: "setup", PickupVerifier: "verifier"}
		if BeginManagedAuth(pending) != nil || SaveManagedAuthKey("setup", "key") != nil {
			os.Exit(18)
		}
		os.Exit(17)
	}
	managedTestHome(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestManagedAuthProcessCrashRecovery$")
	cmd.Env = append(os.Environ(), "CLI_MANAGED_CRASH_CHILD=1")
	if err := cmd.Run(); err == nil {
		t.Fatal("child did not crash as expected")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 17 {
		t.Fatalf("child could not save pending state: %v", err)
	}
	pending, err := LoadManagedAuth()
	if err != nil || pending == nil || pending.APIKey != "key" || pending.SetupCode != "setup" {
		t.Fatalf("failed to recover child state: %v", err)
	}
}

func TestManagedAuthProtectsChangedProfileAndConcurrentBegin(t *testing.T) {
	managedTestHome(t)
	if err := SaveProfile(Profile{BaseURL: "https://old.example", APIKey: "old-key"}); err != nil {
		t.Fatal(err)
	}
	attempts := []ManagedAuthPending{
		{BaseURL: "https://a.example", SetupCode: "a", PickupVerifier: "a-secret"},
		{BaseURL: "https://b.example", SetupCode: "b", PickupVerifier: "b-secret"},
	}
	results := make(chan error, 2)
	for _, attempt := range attempts {
		attempt := attempt
		go func() { results <- BeginManagedAuth(attempt) }()
	}
	successes, conflicts := 0, 0
	for range attempts {
		switch err := <-results; {
		case err == nil:
			successes++
		case errors.Is(err, ErrManagedAuthConflict):
			conflicts++
		default:
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent begin successes=%d conflicts=%d", successes, conflicts)
	}
	pending, err := LoadManagedAuth()
	if err != nil || pending == nil {
		t.Fatalf("load pending: %v", err)
	}
	if err := SaveManagedAuthKey(pending.SetupCode, "new-key"); err != nil {
		t.Fatal(err)
	}
	if err := SaveProfile(Profile{APIKey: "other-login-key"}); err != nil {
		t.Fatal(err)
	}
	if err := FinishManagedAuth(pending.SetupCode, "new-key"); !errors.Is(err, ErrManagedAuthConflict) {
		t.Fatalf("changed profile overwritten: %v", err)
	}
	profile, err := LoadProfile()
	if err != nil || profile.APIKey != "other-login-key" {
		t.Fatalf("other login lost: %v", err)
	}
	if err := DiscardManagedAuth(pending.SetupCode); err != nil {
		t.Fatal(err)
	}
}

func TestManagedAuthFinishAfterPromotionInterrupted(t *testing.T) {
	managedTestHome(t)
	pending := ManagedAuthPending{BaseURL: "https://new.example", SetupCode: "setup", PickupVerifier: "verifier"}
	if err := BeginManagedAuth(pending); err != nil {
		t.Fatal(err)
	}
	if err := SaveManagedAuthKey(pending.SetupCode, "key"); err != nil {
		t.Fatal(err)
	}
	// Model a crash after profile rename and before pending-file removal.
	if err := writeProfile(Profile{BaseURL: pending.BaseURL, APIKey: "key"}); err != nil {
		t.Fatal(err)
	}
	if err := FinishManagedAuth(pending.SetupCode, "key"); err != nil {
		t.Fatalf("finish after restart: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir(), "managed-auth-pending.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending file remains: %v", err)
	}
}
