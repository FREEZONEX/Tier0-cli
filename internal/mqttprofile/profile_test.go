package mqttprofile

import (
	"os"
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
