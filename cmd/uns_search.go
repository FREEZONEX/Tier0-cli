package cmd

import (
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search UNS topics",
	Long:  "Search topics in the UNS by keyword, tag, or path prefix.\n\nExamples:\n  tier0 uns search --keyword temp\n  tier0 uns search --path-prefix /devices --size 50\n  tier0 uns search --keyword temp --include-metadata",

	RunE: runUnsSearch,
}

func init() {
	unsSearchCmd.Flags().StringP("keyword", "k", "",
		"Search by name keyword")
	unsSearchCmd.Flags().String("path-prefix", "/",
		"Filter by path prefix")
	unsSearchCmd.Flags().String("topic-type", "",
		"Filter by topic type")
	unsSearchCmd.Flags().Int("page", 1,
		"Page number")
	unsSearchCmd.Flags().IntP("size", "l", 20,
		"Page size (max results)")
	unsSearchCmd.Flags().Bool("include-metadata", false,
		"Include node metadata")
	unsSearchCmd.Flags().Bool("include-leaf-value", false,
		"Include leaf node values")
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
