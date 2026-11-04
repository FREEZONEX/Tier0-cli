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
				expectedSize := 100
				if scenario == "cloud-directory" && q.Parent == "" {
					expectedSize = 200
				}
				if q.Page != 1 || q.Size != expectedSize {
					t.Errorf("bad pagination %+v", q)
				}
				list := `[]`
				if scenario == "cloud-directory" && q.Parent == "" {
					list = `[{"id":"folder-id","kind":"directory","data":{"name":"Plant"}}]`
				}
				if scenario == "cloud-directory" && q.Parent == "folder-id" {
					list = `[{"id":"topic-id","kind":"topic","data":{"name":"Temp"}}]`
				}
				fmt.Fprintf(w, `{"code":200,"data":{"results":[{"queryId":"browse","list":%s,"page":1,"size":%d,"hasMore":false}]}}`, list, q.Size)
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

func TestBrowseUsesOneProfileAcrossFallbackAndPages(t *testing.T) {
	for _, changeAt := range []string{"fallback", "pagination"} {
		t.Run(changeAt, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TIER0_BASE_URL", "")
			otherCalls := 0
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { otherCalls++; w.WriteHeader(500) }))
			defer other.Close()
			requests := 0
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Header.Get("x-api-key") != "original-key" {
					t.Error("original request did not use the original key")
				}
				if r.URL.Path == "/openapi/v1/uns/browse" {
					if changeAt == "fallback" {
						if err := config.ReplaceProfile(config.Profile{BaseURL: other.URL, APIKey: "replacement-key"}); err != nil {
							t.Error(err)
						}
					}
					w.WriteHeader(404)
					return
				}
				var q struct {
					Queries []struct {
						Parent     string `json:"parentId"`
						Page, Size int
					} `json:"queries"`
				}
				if err := json.NewDecoder(r.Body).Decode(&q); err != nil || len(q.Queries) != 1 {
					t.Errorf("bad request: %v", err)
					return
				}
				v := q.Queries[0]
				list := `[]`
				more := false
				if v.Parent == "" && v.Page == 1 {
					list = `[{"id":"folder","kind":"directory","data":{"name":"Plant"}}]`
					more = true
					if changeAt == "pagination" {
						if err := config.ReplaceProfile(config.Profile{BaseURL: other.URL, APIKey: "replacement-key"}); err != nil {
							t.Error(err)
						}
					}
				}
				if v.Parent == "folder" {
					list = `[{"id":"owned","kind":"topic","data":{"name":"Temp"}}]`
				}
				fmt.Fprintf(w, `{"code":200,"data":{"results":[{"queryId":"browse","list":%s,"page":%d,"size":%d,"hasMore":%t}]}}`, list, v.Page, v.Size, more)
			}))
			defer origin.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: origin.URL, APIKey: "original-key"}); err != nil {
				t.Fatal(err)
			}
			out, _, err := executeRootForTest("uns", "browse", "--path", "Plant", "--json")
			if err != nil || !strings.Contains(out, "owned") || requests != 4 || otherCalls != 0 {
				t.Fatalf("out=%q err=%v original=%d replacement=%d", out, err, requests, otherCalls)
			}
		})
	}
}

func TestCloudPathUsesIndependentPageSize(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TIER0_BASE_URL", "")
	requests := 0
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var q struct {
			Queries []struct {
				Parent string `json:"parentId"`
				Page   int    `json:"page"`
				Size   int    `json:"size"`
			} `json:"queries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil || len(q.Queries) != 1 {
			t.Errorf("bad request %v", err)
			return
		}
		v := q.Queries[0]
		list := []map[string]any{}
		more := false
		if v.Parent == "" && v.Page == 1 {
			if v.Size != 200 {
				t.Errorf("path page size = %d", v.Size)
			}
			list = append(list, map[string]any{"id": "target", "kind": "directory", "data": map[string]string{"name": "Plant"}})
			for i := 0; i < 199; i++ {
				list = append(list, map[string]any{"id": fmt.Sprint(i), "kind": "directory", "data": map[string]string{"name": fmt.Sprintf("sibling-%d", i)}})
			}
			more = true
		} else if v.Parent == "" && v.Page == 2 {
			if v.Size != 200 {
				t.Errorf("path page size = %d", v.Size)
			}
			list = append(list, map[string]any{"id": "later", "kind": "directory", "data": map[string]string{"name": "Later"}})
		} else if v.Parent == "target" {
			if v.Size != 1 {
				t.Errorf("result page size = %d", v.Size)
			}
			list = append(list, map[string]any{"id": "child", "kind": "topic", "data": map[string]string{"name": "Child"}})
		} else {
			t.Errorf("unexpected query %+v", v)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": map[string]any{"results": []any{map[string]any{"queryId": "browse", "list": list, "page": v.Page, "size": v.Size, "hasMore": more}}}})
	}))
	defer origin.Close()
	if err := config.SaveProfile(config.Profile{BaseURL: origin.URL, APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	out, _, err := executeRootForTest("uns", "browse", "--cloud", "--path", "Plant", "--size", "1", "--json")
	if err != nil || !strings.Contains(out, "child") || requests != 3 {
		t.Fatalf("out=%q err=%v requests=%d", out, err, requests)
	}
}

func TestCloudPathDoesNotAssumeUniquenessOnIncompleteListing(t *testing.T) {
	for _, mode := range []string{"duplicate", "limit"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TIER0_BASE_URL", "")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var req struct {
					Queries []struct {
						Page int `json:"page"`
						Size int `json:"size"`
					} `json:"queries"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				q := req.Queries[0]
				list := `[]`
				if q.Page == 1 || (mode == "duplicate" && q.Page == 2) {
					list = fmt.Sprintf(`[{"id":"id-%d","kind":"directory","data":{"name":"Plant"}}]`, q.Page)
				}
				more := mode == "limit" || q.Page == 1
				fmt.Fprintf(w, `{"code":200,"data":{"results":[{"queryId":"browse","list":%s,"page":%d,"size":%d,"hasMore":%t}]}}`, list, q.Page, q.Size, more)
			}))
			defer server.Close()
			if err := config.SaveProfile(config.Profile{BaseURL: server.URL, APIKey: "synthetic"}); err != nil {
				t.Fatal(err)
			}
			_, stderr, err := executeRootForTest("uns", "browse", "--cloud", "--path", "Plant", "--json")
			if err == nil {
				t.Fatal("accepted an ambiguous or incomplete listing")
			}
			if mode == "duplicate" && calls != 2 {
				t.Fatalf("duplicate calls=%d", calls)
			}
			if mode == "limit" && (calls != 100 || !strings.Contains(stderr, "cannot be resolved uniquely")) {
				t.Fatalf("limit calls=%d stderr=%q", calls, stderr)
			}
		})
	}
}
