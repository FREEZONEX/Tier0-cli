package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"10MB", 10 << 20, true},
		{"10MiB", 10 << 20, true},
		{"10mb", 10 << 20, true},
		{"5m", 5 << 20, true},
		{"10485760", 10485760, true},
		{"512", 512, true},
		{"1GB", 1 << 30, true},
		{"2GB", 2 << 30, true},
		{"", 0, false},
		{"MB", 0, false},
		{"10XB", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, err := parseSize(c.in)
		if c.ok {
			if err != nil {
				t.Errorf("parseSize(%q) unexpected error: %v", c.in, err)
				continue
			}
			if got != c.want {
				t.Errorf("parseSize(%q) = %d, want %d", c.in, got, c.want)
			}
		} else if err == nil {
			t.Errorf("parseSize(%q) = %d, nil; want error", c.in, got)
		}
	}
}

func TestPartCountFor(t *testing.T) {
	cases := []struct {
		size, partSize int64
		want           int
	}{
		{5 << 20, 5 << 20, 1},       // 恰好整除
		{(5 << 20) + 1, 5 << 20, 2}, // 余 1 字节 → 2 片
		{100 << 20, 10 << 20, 10},   // 恰好 10 片
		{(100 << 20) + 1, 10 << 20, 11},
		{100, 30, 4},
		{0, 10 << 20, 0},
	}
	for _, c := range cases {
		if got := partCountFor(c.size, c.partSize); got != c.want {
			t.Errorf("partCountFor(%d, %d) = %d, want %d", c.size, c.partSize, got, c.want)
		}
	}
}

func TestPartLen(t *testing.T) {
	st := &uploadState{Size: 100, PartSize: 30, PartCount: 4}
	want := []int64{30, 30, 30, 10}
	for n := 1; n <= st.PartCount; n++ {
		if got := st.partLen(n); got != want[n-1] {
			t.Errorf("partLen(%d) = %d, want %d", n, got, want[n-1])
		}
	}
}

func TestStateFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".tier0-upload-state.json")
	store := newStateStore(path)

	st := &uploadState{
		LocalPath: "/tmp/x.bin",
		FileName:  "x.bin",
		Size:      3 << 20,
		FileKey:   "key-1",
		UploadID:  "up-1",
		PartSize:  1 << 20,
		PartCount: 3,
		Parts:     map[int]string{1: "etag-1"},
	}
	if err := store.save(st); err != nil {
		t.Fatalf("save: %v", err)
	}
	if !store.exists() {
		t.Fatal("state file must exist after save")
	}

	loaded, err := store.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.FileKey != "key-1" || loaded.UploadID != "up-1" {
		t.Errorf("loaded state mismatch: %+v", loaded)
	}
	if loaded.PartSize != 1<<20 || loaded.PartCount != 3 || loaded.Size != 3<<20 {
		t.Errorf("loaded numeric fields mismatch: %+v", loaded)
	}
	if !loaded.isCompleted(1) {
		t.Error("part 1 must be completed after round trip")
	}
	if loaded.isCompleted(2) {
		t.Error("part 2 must not be completed")
	}
	if got := loaded.Parts[1]; got != "etag-1" {
		t.Errorf("part 1 etag = %q, want etag-1", got)
	}
}

func TestStateAddPartPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := newStateStore(path)
	st := &uploadState{PartCount: 4, Parts: map[int]string{}}
	if err := store.save(st); err != nil {
		t.Fatal(err)
	}

	st.addPart(2, "etag-2")
	if err := store.save(st); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.load()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Parts[2]; got != "etag-2" {
		t.Errorf("part 2 etag = %q, want etag-2", got)
	}
	if loaded.isCompleted(1) {
		t.Error("part 1 must not be completed")
	}
}

func TestStateFilePathDeterministic(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.bin")
	p2 := filepath.Join(dir, "b.bin")

	s1, err := multipartStatePath(p1)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := multipartStatePath(p1)
	if err != nil {
		t.Fatal(err)
	}
	s3, err := multipartStatePath(p2)
	if err != nil {
		t.Fatal(err)
	}

	if s1 != s2 {
		t.Errorf("same local path must map to the same state file: %q vs %q", s1, s2)
	}
	if s1 == s3 {
		t.Errorf("different local paths must map to different state files: %q", s1)
	}
	if filepath.Dir(s1) != dir {
		t.Errorf("state file must sit next to the source file: %q", s1)
	}
	base := filepath.Base(s1)
	if !strings.HasPrefix(base, ".tier0-upload-") || !strings.HasSuffix(base, ".json") {
		t.Errorf("unexpected state filename: %q", base)
	}
}

func TestMultipartStateExists(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.bin")
	if multipartStateExists(p) {
		t.Fatal("no state file yet, must not exist")
	}
	// 写入一个状态文件后再检查
	statePath, err := multipartStatePath(p)
	if err != nil {
		t.Fatal(err)
	}
	store := newStateStore(statePath)
	if err := store.save(&uploadState{Parts: map[int]string{}}); err != nil {
		t.Fatal(err)
	}
	if !multipartStateExists(p) {
		t.Fatal("state file written, must exist")
	}
}
