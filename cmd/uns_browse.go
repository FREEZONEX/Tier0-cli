package cmd

import (
	"errors"
	"net/http"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsBrowseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse UNS namespaces",
	Long:  "Browse Enterprise v1 trees or Cloud v2 namespace children. Cloud returns a native paginated response.\n\nExamples:\n  tier0 uns browse\n  tier0 uns browse --path /devices\n  tier0 uns browse --path / --max-depth 2",

	RunE: runUnsBrowse,
}

func init() {
	unsBrowseCmd.Flags().Bool("cloud", false, "Use Cloud v2 namespace query and return its native paginated response")
	unsBrowseCmd.Flags().Int("page", 1, "Cloud page number (requires --cloud)")
	unsBrowseCmd.Flags().Int("size", 100, "Cloud page size, 1-200 (requires --cloud)")
	unsBrowseCmd.Flags().StringP("path", "p", "/",
		"Path to browse in the UNS tree")
	unsBrowseCmd.Flags().IntP("max-depth", "d", 1,
		"Max recursion depth (0 = unlimited)")
	unsBrowseCmd.Flags().Bool("include-metadata", false,
		"Include node metadata")
	unsBrowseCmd.Flags().Bool("include-leaf-value", false,
		"Include leaf node values")
}

func runUnsBrowse(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	path, _ := cmd.Flags().GetString("path")
	depth, _ := cmd.Flags().GetInt("max-depth")

	includeMeta, _ := cmd.Flags().GetBool("include-metadata")
	includeLeaf, _ := cmd.Flags().GetBool("include-leaf-value")

	payload := map[string]any{"path": path}
	if depth != 1 {
		payload["max_depth"] = depth
	}
	if includeMeta {
		payload["include_metadata"] = true
	}
	if includeLeaf {
		payload["include_leaf_value"] = true
	}

	cloud, _ := cmd.Flags().GetBool("cloud")
	page, _ := cmd.Flags().GetInt("page")
	size, _ := cmd.Flags().GetInt("size")
	if (cmd.Flags().Changed("page") || cmd.Flags().Changed("size")) && !cloud {
		return invalidArgument(cmd, "--page/--size", "requires --cloud")
	}
	if page < 1 || size < 1 || size > 200 {
		return invalidArgument(cmd, "--page/--size", "page must be positive and size must be between 1 and 200")
	}
	c, err := cmdutil.NewConfiguredClient()
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	var resp string
	if !cloud {
		resp, err = c.DoAPI(cmd.Context(), "/openapi/v1/uns/browse", "POST", cmdutil.JSONString(payload), debug)
		if err == nil {
			err = cmdutil.CheckResponse(resp)
			if err != nil {
				return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
			}
		} else {
			var apiError *apierr.APIError
			if !errors.As(err, &apiError) || apiError.HTTPStatus != http.StatusNotFound {
				return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
			}
			cloud = true
		}
	}
	if cloud {
		if depth != 1 || includeMeta || includeLeaf {
			return invalidArgument(cmd, "uns browse", "Cloud v2 returns native metadata for one directory; --max-depth, --include-metadata and --include-leaf-value are not supported")
		}
		if strings.TrimSpace(path) != path {
			return invalidArgument(cmd, "--path", "Cloud path cannot have leading or trailing whitespace")
		}
		resp, err = cloudBrowse(cmd.Context(), c, path, page, size, debug)
		if err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
		}
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		stdout.Write([]byte(resp + "\n"))
	}
	return nil
}
