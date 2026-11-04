package mqttprofile

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStoreRoundTripAndDelete(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	if err := store.Prepare("demo"); err != nil {
		t.Fatal(err)
	}
	want := Credential{
		ID:                          42,
		Name:                        "demo-device",
		BaseURL:                     "https://tier0.example",
		Broker:                      "ssl://mqtt.example:8883",
		ClientID:                    "client-base",
		Username:                    "user",
		Password:                    "secret",
		ClientIDRandomSuffixEnabled: true,
	}
	if err := store.Save("demo", want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("demo")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.Password != want.Password || got.Broker != want.Broker || !got.ClientIDRandomSuffixEnabled {
		t.Fatalf("unexpected credential: %#v", got)
	}
	info, err := os.Stat(filepath.Join(store.Dir, "demo.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Windows reports synthetic POSIX permission bits; the file inherits the
	// current user's profile-directory ACL there.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("credential permissions are too broad: %o", info.Mode().Perm())
	}
	if err := store.Delete("demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("demo"); err == nil {
		t.Fatal("expected deleted profile to be missing")
	}
}

func TestLifecycleLockAcrossProcesses(t *testing.T) {
	if dir := os.Getenv("TIER0_TEST_MQTT_LOCK_DIR"); dir != "" {
		store := &Store{Dir: dir}
		lock, err := store.Lock("agent")
		if err == nil {
			lock.Unlock()
			t.Fatal("concurrent process acquired locked profile")
		}
		return
	}
	dir := t.TempDir()
	store := &Store{Dir: dir}
	lock, err := store.Lock("agent")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLifecycleLockAcrossProcesses$")
	cmd.Env = append(os.Environ(), "TIER0_TEST_MQTT_LOCK_DIR="+dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("lock subprocess: %v %s", err, output)
	}
	if err := lock.Unlock(); err != nil {
		t.Fatal(err)
	}
	next, err := store.Lock("agent")
	if err != nil {
		t.Fatalf("lock not released: %v", err)
	}
	next.Unlock()
}

func TestPrepareRejectsUnsupportedSafeInstall(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	old := linkCredential
	linkCredential = func(string, string) error { return errors.New("unsupported") }
	t.Cleanup(func() { linkCredential = old })
	if err := store.Prepare("demo"); err == nil {
		t.Fatal("unsupported hard links accepted")
	}
	entries, err := os.ReadDir(store.Dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe files remain: %v %v", entries, err)
	}
}

func TestStoreRejectsUnsafeAndDuplicateNames(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	for _, name := range []string{"", "../escape", "has space", "a/b"} {
		if err := store.Prepare(name); err == nil {
			t.Fatalf("Prepare(%q) succeeded", name)
		}
	}
	credential := Credential{ID: 1, Broker: "ssl://mqtt.example:8883", ClientID: "c", Username: "u", Password: "p"}
	if err := store.Save("demo", credential); err != nil {
		t.Fatal(err)
	}
	if err := store.Prepare("demo"); err == nil {
		t.Fatal("expected duplicate profile error")
	}
	if err := store.Save("demo", credential); err == nil {
		t.Fatal("expected duplicate save error")
	}
}
