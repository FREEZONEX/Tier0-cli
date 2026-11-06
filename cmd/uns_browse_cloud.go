package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/client"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
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

func cloudBrowse(ctx context.Context, c *client.Client, path, parentID string, page, size, maxPages int, debug bool) (string, error) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		segments = nil
	}
	parent := parentID
	query := func(parent, search string, page, querySize int) (string, cloudNamespace, error) {
		q := map[string]any{"queryId": "browse", "page": page, "size": querySize}
		if search != "" {
			q["search"] = search
		}
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
			return "", result, errs.New(errs.CategoryAPI, 0, "invalid Cloud namespace response")
		}
		item := result.Data.Results[0]
		if item.QueryID != "browse" || item.List == nil || item.Page != page || item.Size != querySize || item.HasMore == nil {
			return "", result, errs.New(errs.CategoryAPI, 0, "invalid Cloud namespace pagination")
		}
		return raw, result, nil
	}
	for _, segment := range segments {
		if segment == "" || !utf8.ValidString(segment) {
			return "", errs.InvalidArgument("--path", "invalid Cloud namespace path")
		}
		found := ""
		// ILIKE is substring matching and also searches IDs. Use a valid UTF-8
		// prefix bounded by the server's 128-byte limit; wildcard and escape
		// characters terminate the optimization, never the exact comparison.
		search := cloudSearchPrefix(segment)
		querySize := 200
		for {
			found = "" // a smaller page size changes offsets: rescan from page 1.
			shrink := false
			for p := 1; p <= maxPages; p++ {
				_, v, err := query(parent, search, p, querySize)
				if err != nil {
					var apiError *apierr.APIError
					if errors.As(err, &apiError) && apiError.HTTPStatus == http.StatusRequestEntityTooLarge && apiError.BackendCode == "RESPONSE_TOO_LARGE" && querySize > 1 {
						querySize /= 2
						shrink = true
						break
					}
					return "", err
				}
				r := v.Data.Results[0]
				for _, node := range r.List {
					if node.Kind == "directory" && node.Data.Name == segment {
						if found != "" {
							return "", errs.New(errs.CategoryAPI, 0, "ambiguous Cloud namespace path")
						}
						found = node.ID
					}
				}
				if !*r.HasMore {
					break
				}
				if p == maxPages {
					return "", errs.New(errs.CategoryAPI, 0, "Cloud namespace path cannot be resolved uniquely: directory listing exceeds path-max-pages")
				}
			}
			if !shrink {
				break
			}
		}

		if found == "" {
			return "", errs.New(errs.CategoryAPI, 404, fmt.Sprintf("Cloud namespace directory not found: %s", segment))
		}
		parent = found
	}
	raw, _, err := query(parent, "", page, size)
	return raw, err
}

// cloudSearchPrefix cannot narrow out a literal match under PostgreSQL ILIKE.
func cloudSearchPrefix(name string) string {
	end := 0
	for offset, r := range name {
		if r == '%' || r == '_' || r == '\\' || offset+utf8.RuneLen(r) > 128 {
			break
		}
		end = offset + utf8.RuneLen(r)
	}
	return name[:end]
}
