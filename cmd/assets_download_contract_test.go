package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func trustDownloadServers(t *testing.T, servers ...*httptest.Server) {
	t.Helper()
	roots := x509.NewCertPool()
	for _, server := range servers {
		roots.AddCert(server.Certificate())
	}
	old := http.DefaultTransport
	transport := old.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = old; transport.CloseIdleConnections() })
}

func TestAssetsDownloadV2PresignedStorage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	key := "synthetic-api-key"
	signature := "synthetic-presigned-secret"
	storageRequests := 0
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageRequests++
		if r.URL.Query().Get("X-Amz-Signature") != signature {
			t.Error("signature missing")
		}
		for _, h := range []string{"x-api-key", "Authorization", "Cookie", "Referer"} {
			if r.Header.Get(h) != "" {
				t.Errorf("storage received %s", h)
			}
		}
		fmt.Fprint(w, "file content")
	}))
	defer storage.Close()
	gateway := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/v2/assets/files/download" || r.Method != http.MethodGet || r.URL.Query().Get("filePath") != "workspace/file" {
			t.Error("wrong gateway contract")
		}
		if r.Header.Get("x-api-key") != key {
			t.Error("gateway did not receive key")
		}
		http.Redirect(w, r, storage.URL+"/bucket/file?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature="+signature, http.StatusFound)
	}))
	defer gateway.Close()
	trustDownloadServers(t, gateway, storage)
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: key}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "file")
	out, stderr, err := executeRootForTest("assets", "download", "--file-path", "workspace/file", "-o", file, "--json")
	if err != nil {
		t.Fatalf("download failed: %v %s", err, stderr)
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "file content" || storageRequests != 1 {
		t.Fatal("file not downloaded")
	}
	if strings.Contains(out+stderr, signature) || strings.Contains(out+stderr, key) {
		t.Fatal("output exposed credentials")
	}
}

func TestDownloadRedirectSafety(t *testing.T) {
	parse := func(s string) *url.URL {
		u, err := url.Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	origin := &http.Request{URL: parse("https://gateway.test/file?private=1"), Header: http.Header{"X-Api-Key": {"key"}}}
	external := &http.Request{URL: parse("https://storage.test/object?signature=secret"), Header: http.Header{"X-Api-Key": {"key"}, "Authorization": {"auth"}, "Cookie": {"cookie"}, "Referer": {"signed"}, "X-Other-Secret": {"secret"}}}
	if err := downloadRedirect(external, []*http.Request{origin}); err != nil || len(external.Header) != 0 {
		t.Fatal("cross-origin headers not removed")
	}
	back := &http.Request{URL: parse("https://gateway.test/again"), Header: origin.Header.Clone()}
	if err := downloadRedirect(back, []*http.Request{origin, external}); err != nil || len(back.Header) != 0 {
		t.Fatal("credentials restored after crossing origin")
	}
	same := &http.Request{URL: parse("https://gateway.test/other?query=1"), Header: origin.Header.Clone()}
	if err := downloadRedirect(same, []*http.Request{origin}); err != nil || same.Header.Get("X-Api-Key") != "key" {
		t.Fatal("same-origin redirect lost key")
	}
	for _, target := range []string{"http://storage.test/object", "https://user:password@storage.test/object"} {
		req := &http.Request{URL: parse(target)}
		if downloadRedirect(req, []*http.Request{origin}) == nil {
			t.Fatal("unsafe redirect accepted")
		}
	}
	if downloadRedirect(same, []*http.Request{origin, origin, origin, origin, origin, origin}) == nil {
		t.Fatal("redirect limit not enforced")
	}
}

func TestAssetsDownloadFallbackOnlyOnDirectV2NotFound(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		fallback bool
	}{{"not found", 404, true}, {"forbidden", 403, false}, {"server error", 500, false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			var paths []string
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if r.URL.Path == "/openapi/v2/assets/files/download" {
					w.WriteHeader(tc.status)
					return
				}
				if r.URL.Path == "/openapi/v1/assets/files/download" {
					http.Redirect(w, r, "/legacy-file", http.StatusFound)
					return
				}
				if r.URL.Path == "/legacy-file" {
					fmt.Fprint(w, "legacy file")
					return
				}
				t.Error("unexpected endpoint")
			}))
			defer gateway.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "file")
			_, _, err := executeRootForTest("assets", "download", "--file-path", "key", "-o", file)
			if (err == nil) != tc.fallback || len(paths) != 1+2*btoi(tc.fallback) {
				t.Fatalf("incorrect fallback: %v %v", paths, err)
			}
		})
	}
}
func btoi(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestStorageNotFoundDoesNotFallbackOrLeakSignature(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "synthetic-signed-secret", 404) }))
	defer storage.Close()
	calls := 0
	gateway := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, storage.URL+"/?sig=synthetic-signed-secret", 302)
	}))
	defer gateway.Close()
	trustDownloadServers(t, gateway, storage)
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic-key"}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("assets", "download", "--file-path", "key", "--json")
	if err == nil || calls != 1 || strings.Contains(out+stderr, "synthetic-signed-secret") {
		t.Fatal("storage failure leaked signature or retried gateway")
	}
}

func TestDownloadMultiHopNeverRestoresSensitiveHeaders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var gateway *httptest.Server
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "" || r.Header.Get("Referer") != "" {
			t.Error("storage received sensitive header")
		}
		http.Redirect(w, r, gateway.URL+"/returned?sig=second-secret", 302)
	}))
	defer storage.Close()
	gateway = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/returned" {
			if r.Header.Get("x-api-key") != "" || r.Header.Get("Referer") != "" {
				t.Error("key or signed referer restored after crossing origin")
			}
			fmt.Fprint(w, "multi-hop file")
			return
		}
		if r.Header.Get("x-api-key") == "" {
			t.Error("gateway missing key")
		}
		http.Redirect(w, r, storage.URL+"/object?sig=first-secret", 302)
	}))
	defer gateway.Close()
	trustDownloadServers(t, gateway, storage)
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("assets", "download", "--file-path", "key")
	if err != nil || out != "multi-hop file" || strings.Contains(stderr, "secret") {
		t.Fatal("multi-hop download failed")
	}
}

func TestDownloadBlocksHTTPSDowngradeAndRedactsLocation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	received := 0
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received++ }))
	defer insecure.Close()
	gateway := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL+"/?sig=synthetic-signed-secret", 302)
	}))
	defer gateway.Close()
	trustDownloadServers(t, gateway)
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic-key"}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("assets", "download", "--file-path", "key", "--json")
	if err == nil || received != 0 || strings.Contains(out+stderr, "synthetic-signed-secret") || strings.Contains(out+stderr, "synthetic-key") {
		t.Fatal("downgrade not blocked safely")
	}
}

func TestSameOriginSignedRedirectNeverSendsReferer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	secret := "same-origin-signed-secret"
	gateway := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "" {
			t.Error("redirect sent a Referer")
		}
		switch r.URL.Path {
		case "/openapi/v2/assets/files/download":
			http.Redirect(w, r, "/signed?signature="+secret, 302)
		case "/signed":
			http.Redirect(w, r, "/file", 302)
		case "/file":
			fmt.Fprint(w, "same-origin file")
		default:
			t.Error("unexpected path")
		}
	}))
	defer gateway.Close()
	trustDownloadServers(t, gateway)
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRootForTest("assets", "download", "--file-path", "key")
	if err != nil || out != "same-origin file" || strings.Contains(stderr, secret) {
		t.Fatal("same-origin download failed or exposed signature")
	}
}

func TestGatewayJSONBusinessErrorDoesNotOverwriteFile(t *testing.T) {
	for _, version := range []string{"v2", "v1 fallback"} {
		t.Run(version, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if version == "v1 fallback" && r.URL.Path == "/openapi/v2/assets/files/download" {
					w.WriteHeader(404)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				fmt.Fprint(w, `{"code":400123,"msg":"permission denied"}`)
			}))
			defer gateway.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "existing")
			if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := executeRootForTest("assets", "download", "--file-path", "key", "-o", file, "--json")
			data, readErr := os.ReadFile(file)
			if err == nil || readErr != nil || string(data) != "original" || !strings.Contains(stderr, "gateway did not return a 302") || strings.Contains(out, "permission denied") {
				t.Fatal("gateway business error treated as a file")
			}
		})
	}
}

func TestRedirectedStorageJSONIsDownloaded(t *testing.T) {
	for _, sameOrigin := range []bool{true, false} {
		t.Run(fmt.Sprintf("same_origin_%v", sameOrigin), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			payload := `{"code":400123,"msg":"this is a legitimate JSON file"}`
			storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, payload)
			}))
			defer storage.Close()
			gateway := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/file" {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, payload)
					return
				}
				target := storage.URL + "/file?signature=secret"
				if sameOrigin {
					target = "/file?signature=secret"
				}
				http.Redirect(w, r, target, 302)
			}))
			defer gateway.Close()
			trustDownloadServers(t, gateway, storage)
			if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
				t.Fatal(err)
			}
			out, _, err := executeRootForTest("assets", "download", "--file-path", "key")
			if err != nil || out != payload {
				t.Fatal("legitimate JSON file was not downloaded")
			}
			if runtime.GOOS == "windows" {
				return
			}
			dir := t.TempDir()
			file := filepath.Join(dir, "data.json")
			if err := os.WriteFile(file, []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := executeRootForTest("assets", "download", "--file-path", "key", "-o", file); err != nil {
				t.Fatal(err)
			}
			data, readErr := os.ReadFile(file)
			entries, _ := os.ReadDir(dir)
			if readErr != nil || string(data) != payload || len(entries) != 1 {
				t.Fatal("JSON file replacement failed")
			}
		})
	}
}

func TestGatewayRejectsDirectResponseAndLoginRedirect(t *testing.T) {
	for _, tc := range []struct {
		name              string
		code              int
		location, content string
	}{
		{"html", 200, "", "<html>login</html>"},
		{"binary", 200, "", "not a redirect"},
		{"no location", 302, "", ""},
		{"wrong redirect status", 307, "/file", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			reached := false
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/openapi/v2/assets/files/download" {
					reached = true
					fmt.Fprint(w, "<html>login</html>")
					return
				}
				w.Header().Set("Content-Type", "text/html")
				if tc.location != "" {
					w.Header().Set("Location", tc.location)
				}
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.content)
			}))
			defer gateway.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "existing")
			if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := executeRootForTest("assets", "download", "--file-path", "key", "-o", file, "--json")
			data, _ := os.ReadFile(file)
			if err == nil || string(data) != "original" || reached || strings.Contains(out+stderr, "token=secret") {
				t.Fatal("unexpected gateway response accepted or file changed")
			}
		})
	}
}

func TestDownloadAllowsLoginNamedHTMLFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	payload := `<html><title>login</title><form><input TYPE = "password"></form></html>`
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi/v2/assets/files/download" {
			http.Redirect(w, r, "/login", 302)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, payload)
	}))
	defer gateway.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	out, _, err := executeRootForTest("assets", "download", "--file-path", "login")
	if err != nil || out != payload {
		t.Fatal("legitimate HTML object was rejected")
	}
}

func TestDownloadInterruptedDoesNotReplaceFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi/v2/assets/files/download" {
			http.Redirect(w, r, "/object", 302)
			return
		}
		w.Header().Set("Content-Length", "1000")
		fmt.Fprint(w, "partial")
	}))
	defer gateway.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "existing")
	if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := executeRootForTest("assets", "download", "--file-path", "key", "-o", file)
	data, _ := os.ReadFile(file)
	entries, _ := os.ReadDir(dir)
	if err == nil || string(data) != "original" || len(entries) != 1 {
		t.Fatal("partial download replaced file or left temporary file")
	}
}

func TestDownloadOutputPermissionsAndSymlinkPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics")
	}
	t.Setenv("HOME", t.TempDir())
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi/v2/assets/files/download" {
			http.Redirect(w, r, "/object", 302)
			return
		}
		fmt.Fprint(w, "downloaded")
	}))
	defer gateway.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("original"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeRootForTest("assets", "download", "--file-path", "key", "-o", file); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatal("existing permissions not preserved")
	}
	data, _ := os.ReadFile(file)
	if string(data) != "downloaded" {
		t.Fatal("file not replaced")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeRootForTest("assets", "download", "--file-path", "key", "-o", link); err == nil {
		t.Fatal("symlink output was accepted")
	}
	linkInfo, err := os.Lstat(link)
	if err != nil || linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink changed")
	}
	data, _ = os.ReadFile(file)
	if string(data) != "downloaded" {
		t.Fatal("symlink target changed")
	}
}

func TestWindowsDownloadDestinationPolicy(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if info, err := downloadDestination(file, "windows"); err != nil || info != nil {
		t.Fatal("new Windows destination should be accepted")
	}
	if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := downloadDestination(file, "windows"); err == nil {
		t.Fatal("existing Windows file must be refused to preserve ACL")
	}
	if runtime.GOOS == "windows" {
		t.Setenv("HOME", t.TempDir())
		gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/openapi/v2/assets/files/download" {
				http.Redirect(w, r, "/object", 302)
				return
			}
			fmt.Fprint(w, "replacement")
		}))
		defer gateway.Close()
		if err := config.SaveProfile(config.Profile{BaseURL: gateway.URL, APIKey: "synthetic"}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := executeRootForTest("assets", "download", "--file-path", "key", "-o", file); err == nil {
			t.Fatal("existing Windows output accepted")
		}
		data, _ := os.ReadFile(file)
		if string(data) != "original" {
			t.Fatal("existing Windows file changed")
		}
		newFile := filepath.Join(dir, "new")
		if _, _, err := executeRootForTest("assets", "download", "--file-path", "key", "-o", newFile); err != nil {
			t.Fatalf("new Windows output: %v", err)
		}
		data, _ = os.ReadFile(newFile)
		if string(data) != "replacement" {
			t.Fatal("new Windows output incorrect")
		}
	}
}
