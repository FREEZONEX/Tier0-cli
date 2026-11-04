package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/client"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
)

// cloudNamespace preserves Cloud's native, paginated result instead of inventing
// the numeric IDs, topic types, or leaf values in Enterprise's v1 tree.
type cloudNamespace struct {
	Code *int `json:"code"`
	Data *struct {
		Results []struct {
			QueryID string `json:"queryId"`
			List    []struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
				Data struct {
					Name string `json:"name"`
				} `json:"data"`
			} `json:"list"`
			Page    int   `json:"page"`
			Size    int   `json:"size"`
			HasMore *bool `json:"hasMore"`
		} `json:"results"`
	} `json:"data"`
}

func cloudBrowse(ctx context.Context, c *client.Client, path string, page, size int, debug bool) (string, error) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		segments = nil
	}
	parent := ""
	query := func(parent string, page, querySize int) (string, cloudNamespace, error) {
		q := map[string]any{"queryId": "browse", "page": page, "size": querySize}
		if parent != "" {
			q["parentId"] = parent
		}
		body := cmdutil.JSONString(map[string]any{"queries": []any{q}})
		raw, err := c.DoAPI(ctx, "/openapi/v2/uns/namespace/query", "POST", body, debug)
		if err != nil {
			return "", cloudNamespace{}, err
		}
		if err = cmdutil.CheckResponse(raw); err != nil {
			return "", cloudNamespace{}, err
		}
		var result cloudNamespace
		if json.Unmarshal([]byte(raw), &result) != nil || result.Code == nil || (*result.Code != 0 && *result.Code != 200) || result.Data == nil || len(result.Data.Results) != 1 {
			return "", result, fmt.Errorf("invalid Cloud namespace response")
		}
		item := result.Data.Results[0]
		if item.QueryID != "browse" || item.List == nil || item.Page != page || item.Size != querySize || item.HasMore == nil {
			return "", result, fmt.Errorf("invalid Cloud namespace pagination")
		}
		return raw, result, nil
	}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("invalid Cloud namespace path")
		}
		found := ""
		for p := 1; p <= 100; p++ {
			_, v, err := query(parent, p, 200)
			if err != nil {
				return "", err
			}
			r := v.Data.Results[0]
			for _, node := range r.List {
				if node.Kind == "directory" && node.Data.Name == segment {
					if found != "" {
						return "", fmt.Errorf("ambiguous Cloud namespace path")
					}
					found = node.ID
				}
			}
			if !*r.HasMore {
				break
			}
			if p == 100 {
				return "", fmt.Errorf("Cloud namespace path cannot be resolved uniquely: directory listing exceeds 100 pages")
			}
		}
		if found == "" {
			return "", fmt.Errorf("Cloud namespace directory not found: %s", segment)
		}
		parent = found
	}
	raw, _, err := query(parent, page, size)
	return raw, err
}
