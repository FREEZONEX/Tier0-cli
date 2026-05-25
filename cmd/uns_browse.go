package cmd

import (
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsBrowseCmd = &cobra.Command{
	Use:   "browse",
	Short: i18n.T("Browse UNS namespace tree", "浏览 UNS 命名空间树"),
	Long: i18n.T(
		"Browse the UNS namespace tree at a given path.\n\nExamples:\n  tier0 uns browse\n  tier0 uns browse --path /devices\n  tier0 uns browse --path / --max-depth 2",
		"浏览指定路径的 UNS 命名空间树。\n\n示例:\n  tier0 uns browse\n  tier0 uns browse --path /devices\n  tier0 uns browse --path / --max-depth 2",
	),
	RunE: runUnsBrowse,
}

func init() {
	unsBrowseCmd.Flags().StringP("path", "p", "/",
		i18n.T("Path to browse in the UNS tree", "UNS 树中要浏览的路径"))
	unsBrowseCmd.Flags().IntP("max-depth", "d", 1,
		i18n.T("Max recursion depth (0 = unlimited)", "最大递归深度（0 = 不限制）"))
	unsBrowseCmd.Flags().Bool("include-metadata", false,
		i18n.T("Include node metadata", "包含节点元数据"))
	unsBrowseCmd.Flags().Bool("include-leaf-value", false,
		i18n.T("Include leaf node values", "包含叶子节点值"))
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

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		stdout.Write([]byte(resp + "\n"))
	}
	return nil
}
