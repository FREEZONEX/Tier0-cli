package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestCloudFileUploadDownloadDelete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var data []byte
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "" {
			t.Error("API key sent to storage")
		}
		if r.Method == http.MethodPost {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				return
			}
			f, _, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer f.Close()
			data, _ = io.ReadAll(f)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write(data)
	}))
	defer storage.Close()
	var paths []string
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/openapi/v2/assets/files":
			fmt.Fprintf(w, `{"code":200,"data":{"fileId":1,"filePath":"workspace/test","postUrl":%q,"postFields":{"key":"test"}}}`, storage.URL)
		case "/openapi/v2/assets/files/download":
			http.Redirect(w, r, storage.URL+"/file", http.StatusFound)
		case "/openapi/v2/assets/files/delete":
			fmt.Fprint(w, `{"code":200,"data":{"deleted":true}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer gateway.Close()
	trustDownloadServers(t, storage)
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(input, []byte("file content"), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "download.txt")
	for _, args := range [][]string{
		{"assets", "upload", input, "--cloud", "--json"},
		{"assets", "download", "--cloud", "--file-path", "workspace/test", "--output", output, "--json"},
		{"assets", "delete", "--cloud", "--file-path", "workspace/test", "--yes", "--json"},
	} {
		if _, stderr, err := executeRootForTest(args...); err != nil {
			t.Fatalf("%v: %v %s", args, err, stderr)
		}
	}
	b, err := os.ReadFile(output)
	if err != nil || string(b) != "file content" {
		t.Fatalf("download mismatch: %v", err)
	}
	if strings.Join(paths, ",") != "/openapi/v2/assets/files,/openapi/v2/assets/files/download,/openapi/v2/assets/files/delete" {
		t.Fatalf("paths=%v", paths)
	}
}

func TestFileUploadDoesNotFollowRedirectOrExposeURL(t *testing.T) {
	redirected := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected++ }))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	p := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	err := postFileToURL(context.Background(), origin.URL+"/?signature=synthetic-private", nil, p, "text/plain", false)
	if err == nil || redirected != 0 || strings.Contains(err.Error(), "synthetic-private") {
		t.Fatalf("err=%v redirects=%d", err, redirected)
	}
}

func TestCloudDownloadNotFoundDoesNotFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	calls := 0
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; http.NotFound(w, r) }))
	defer gateway.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := executeRootForTest("assets", "download", "--cloud", "--file-path", "missing", "--json")
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
