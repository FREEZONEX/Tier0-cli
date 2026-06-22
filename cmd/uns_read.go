package cmd

import (
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsReadCmd = &cobra.Command{
	Use:   "read [topic...]",
	Short: i18n.T("Read current value of UNS topics", "读取 UNS 点位当前值"),
	Long: i18n.T(
		"Read the current value of one or more UNS topics.\n\nExamples:\n  tier0 uns read demo\n  tier0 uns read --topic demo\n  tier0 uns read temp humidity\n  tier0 uns read --topic sensor1 --include-metadata",
		"读取一个或多个 UNS 点位的当前值。\n\n示例:\n  tier0 uns read demo\n  tier0 uns read --topic demo\n  tier0 uns read temp humidity\n  tier0 uns read --topic sensor1 --include-metadata",
	),
	RunE: runUnsRead,
}

func init() {
	unsReadCmd.Flags().StringSliceP("topic", "t", nil,
		i18n.T("Topic name(s) to read (repeatable; positional args are also accepted)", "要读取的点位名称（可重复指定；也支持位置参数）"))
	unsReadCmd.Flags().Bool("include-metadata", false,
		i18n.T("Include topic metadata (topicType, fields, description)", "包含点位元数据（topicType、fields、description）"))
	unsReadCmd.Flags().Bool("include-leaf-value", false,
		i18n.T("Include leaf node values", "包含叶子节点值"))
}

func runUnsRead(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topics, _ := cmd.Flags().GetStringSlice("topic")
	includeMeta, _ := cmd.Flags().GetBool("include-metadata")
	includeLeaf, _ := cmd.Flags().GetBool("include-leaf-value")
	topics = append(topics, args...)
	if len(topics) == 0 {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("%s", i18n.T(
			"specify at least one topic via --topic <path> or positional arguments",
			"请通过 --topic <路径> 或位置参数指定至少一个点位",
		)), jsonMode)
	}

	payload := map[string]any{"topics": topics}
	if includeMeta {
		payload["include_metadata"] = true
	}
	if includeLeaf {
		payload["include_leaf_value"] = true
	}

	body := cmdutil.JSONString(payload)

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/read", "POST", body, debug)
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
