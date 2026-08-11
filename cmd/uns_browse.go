package cmd

import (
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsBrowseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse UNS namespace tree",
	Long:  "Browse the UNS namespace tree at a given path.\n\nExamples:\n  tier0 uns browse\n  tier0 uns browse --path /devices\n  tier0 uns browse --path / --max-depth 2",

	RunE: runUnsBrowse,
}

func init() {
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

	body := cmdutil.JSONString(payload)

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/browse", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckResponse(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		stdout.Write([]byte(resp + "\n"))
	}
	return nil
}
