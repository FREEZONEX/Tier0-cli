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

func TestBrowseEnterpriseAndCloudContracts(t *testing.T) {
	for _, scenario := range []string{"enterprise", "cloud-root", "cloud-directory", "denied", "invalid-response"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TIER0_BASE_URL", "")
			calls := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.URL.Path)
				if r.URL.Path == "/openapi/v1/uns/browse" {
					if scenario == "enterprise" {
						fmt.Fprint(w, `{"code":200,"data":{"tree":[]}}`)
						return
					}
					if scenario == "denied" {
						w.WriteHeader(403)
						fmt.Fprint(w, `{"code":403,"msg":"denied"}`)
						return
					}
					w.WriteHeader(404)
					return
				}
				if r.URL.Path != "/openapi/v2/uns/namespace/query" {
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(500)
					return
				}
				if scenario == "invalid-response" {
					fmt.Fprint(w, `{"code":200,"data":{}}`)
					return
				}
				var req struct {
					Queries []struct {
						Parent string `json:"parentId"`
						Page   int    `json:"page"`
						Size   int    `json:"size"`
					} `json:"queries"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Queries) != 1 {
					t.Errorf("invalid query: %v", err)
					w.WriteHeader(400)
					return
				}
				q := req.Queries[0]
				if q.Page != 1 || q.Size != 100 {
					t.Errorf("bad pagination %+v", q)
				}
				list := `[]`
				if scenario == "cloud-directory" && q.Parent == "" {
					list = `[{"id":"folder-id","kind":"directory","data":{"name":"Plant"}}]`
				}
				if scenario == "cloud-directory" && q.Parent == "folder-id" {
					list = `[{"id":"topic-id","kind":"topic","data":{"name":"Temp"}}]`
				}
				fmt.Fprintf(w, `{"code":200,"data":{"results":[{"queryId":"browse","list":%s,"page":1,"size":100,"hasMore":false}]}}`, list)
			}))
			defer server.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
				t.Fatal(err)
			}
			args := []string{"uns", "browse", "--json"}
			if scenario == "cloud-directory" {
				args = append(args, "--path", "/Plant")
			}
			out, _, err := executeRootForTest(args...)
			switch scenario {
			case "enterprise":
				if err != nil || !strings.Contains(out, `"tree"`) || len(calls) != 1 {
					t.Fatalf("enterprise result %q %v calls=%v", out, err, calls)
				}
			case "cloud-root":
				if err != nil || !strings.Contains(out, `"results"`) || len(calls) != 2 {
					t.Fatalf("cloud root %q %v calls=%v", out, err, calls)
				}
			case "cloud-directory":
				if err != nil || !strings.Contains(out, "topic-id") || len(calls) != 3 {
					t.Fatalf("cloud directory %q %v calls=%v", out, err, calls)
				}
			case "denied":
				if err == nil || len(calls) != 1 {
					t.Fatalf("denial retried: %v %v", err, calls)
				}
			case "invalid-response":
				if err == nil {
					t.Fatal("accepted invalid namespace response")
				}
			}
		})
	}
}

func TestCloudBrowsePathResolutionAcrossPages(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var req struct {
			Queries []struct {
				Parent string `json:"parentId"`
				Page   int    `json:"page"`
				Size   int    `json:"size"`
			} `json:"queries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Queries) != 1 {
			t.Errorf("bad query %v", err)
			return
		}
		q := req.Queries[0]
		list := `[]`
		more := false
		switch {
		case q.Parent == "" && q.Page == 1:
			more = true
		case q.Parent == "" && q.Page == 2:
			list = `[{"id":"p","kind":"directory","data":{"name":"Plant"}}]`
		case q.Parent == "p" && q.Page == 1:
			list = `[{"id":"t","kind":"topic","data":{"name":"Temp"}}]`
		default:
			t.Errorf("unexpected query %+v", q)
		}
		fmt.Fprintf(w, `{"code":200,"data":{"results":[{"queryId":"browse","list":%s,"page":%d,"size":%d,"hasMore":%t}]}}`, list, q.Page, q.Size, more)
	}))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	out, _, err := executeRootForTest("uns", "browse", "--cloud", "--path", "Plant", "--json")
	if err != nil || !strings.Contains(out, `"id":"t"`) || requests != 3 {
		t.Fatalf("unexpected result %q err=%v requests=%d", out, err, requests)
	}
}

func TestCloudBrowseDoesNotPretendToSupportEnterpriseFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(404) }))
	defer server.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := executeRootForTest("uns", "browse", "--cloud", "--include-leaf-value", "--json")
	if err == nil || calls != 0 {
		t.Fatalf("unsupported option sent request: err=%v calls=%d", err, calls)
	}
}
