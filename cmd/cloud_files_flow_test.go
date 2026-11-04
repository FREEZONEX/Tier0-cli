package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
)

func TestAssetsURLVersionSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cloud bool
		path  string
	}{{"enterprise", false, "/openapi/v1/assets/files/url"}, {"cloud", true, "/openapi/v2/assets/files/url"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			var paths []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if r.URL.Path != tc.path || r.URL.Query().Get("filePath") != "owned/test" {
					t.Errorf("unexpected request %s", r.URL.String())
				}
				fmt.Fprint(w, `{"code":200,"data":{"fileUrl":"https://files.example/test","expiresAt":123}}`)
			}))
			defer srv.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: srv.URL, APIKey: "synthetic"}); err != nil {
				t.Fatal(err)
			}
			args := []string{"assets", "url", "--file-path", "owned/test", "--json"}
			if tc.cloud {
				args = append(args, "--cloud")
			}
			out, _, err := executeRootForTest(args...)
			if err != nil || !strings.Contains(out, "files.example") || len(paths) != 1 {
				t.Fatalf("out=%s err=%v paths=%v", out, err, paths)
			}
		})
	}
}

func TestCloudFlowListNativeContract(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/openapi/v2/uns/streams/program/list" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["search"] != "machine" {
			t.Errorf("unexpected body %v", body)
		}
		fmt.Fprint(w, `{"code":200,"data":{"list":[],"total":0,"page":1,"size":20}}`)
	}))
	defer srv.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: srv.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	out, _, err := executeRootForTest("flow", "list", "--cloud", "--keyword", "machine", "--json")
	if err != nil || !strings.Contains(out, `"list":[]`) || calls != 1 {
		t.Fatalf("out=%s err=%v calls=%d", out, err, calls)
	}
}

func TestCloudFlowRejectsEnterpriseTypes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: srv.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := executeRootForTest("flow", "list", "--cloud", "--source", "--json")
	if err == nil || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
