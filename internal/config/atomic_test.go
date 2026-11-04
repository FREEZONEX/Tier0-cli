package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteProfileAtomicallyReplacesProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"apiKey":"old"}`), 0600); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"apiKey":"new"}`)
	if err := writeProfileAtomically(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(want) {
		t.Fatalf("profile = %q, error = %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("profile permissions = %o", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("directory = %v, error = %v", entries, err)
	}
}

func TestWriteProfileAtomicallyCleansUpAfterRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeProfileAtomically(path, []byte("secret")); err == nil {
		t.Fatal("expected rename onto directory to fail")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Fatalf("temporary file was not removed: %v, error = %v", entries, err)
	}
}
