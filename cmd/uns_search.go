package cmd

import (
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: i18n.T("Search UNS topics", "搜索 UNS 点位"),
	Long: i18n.T(
		"Search topics in the UNS by keyword, tag, or path prefix.\n\nExamples:\n  tier0 uns search --keyword temp\n  tier0 uns search --path-prefix /devices --size 50\n  tier0 uns search --keyword temp --include-metadata",
		"按关键字、标签或路径前缀搜索 UNS 中的点位。\n\n示例:\n  tier0 uns search --keyword temp\n  tier0 uns search --path-prefix /devices --size 50\n  tier0 uns search --keyword temp --include-metadata",
	),
	RunE: runUnsSearch,
}

func init() {
	unsSearchCmd.Flags().StringP("keyword", "k", "",
		i18n.T("Search by name keyword", "按名称关键字搜索"))
	unsSearchCmd.Flags().String("path-prefix", "/",
		i18n.T("Filter by path prefix", "按路径前缀过滤"))
	unsSearchCmd.Flags().String("topic-type", "",
		i18n.T("Filter by topic type", "按点位类型过滤"))
	unsSearchCmd.Flags().Int("page", 1,
		i18n.T("Page number", "页码"))
	unsSearchCmd.Flags().IntP("size", "l", 20,
		i18n.T("Page size (max results)", "每页大小（最大结果数）"))
	unsSearchCmd.Flags().Bool("include-metadata", false,
		i18n.T("Include node metadata", "包含节点元数据"))
	unsSearchCmd.Flags().Bool("include-leaf-value", false,
		i18n.T("Include leaf node values", "包含叶子节点值"))
}

func runUnsSearch(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	keyword, _ := cmd.Flags().GetString("keyword")
	pathPrefix, _ := cmd.Flags().GetString("path-prefix")
	topicType, _ := cmd.Flags().GetString("topic-type")
	page, _ := cmd.Flags().GetInt("page")
	size, _ := cmd.Flags().GetInt("size")
	includeMeta, _ := cmd.Flags().GetBool("include-metadata")
	includeLeaf, _ := cmd.Flags().GetBool("include-leaf-value")

	payload := map[string]any{}
	if keyword != "" {
		payload["keyword"] = keyword
	}
	if pathPrefix != "/" {
		payload["path_prefix"] = pathPrefix
	}
	if topicType != "" {
		payload["topicType"] = topicType
	}
	if page != 1 {
		payload["page"] = page
	}
	if size != 20 {
		payload["size"] = size
	}
	if includeMeta {
		payload["include_metadata"] = true
	}
	if includeLeaf {
		payload["include_leaf_value"] = true
	}

	body := cmdutil.JSONString(payload)

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/search", "POST", body, debug)
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
