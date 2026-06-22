package cmd

import (
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: i18n.T("Delete UNS node(s)", "删除 UNS 节点"),
	Long: i18n.T(
		"Delete one or more UNS nodes (path or topic). By default performs a soft delete (recoverable via restore). Use --hard to permanently delete.\n\nExamples:\n  tier0 uns delete --path factory/line1/sensor/temp --yes\n  tier0 uns delete --path factory/line1/sensor/temp --path factory/line1/sensor/humi --yes\n  tier0 uns delete --path factory/line1/sensor/temp --hard --yes",
		"删除一个或多个 UNS 节点（路径或点位）。默认执行软删除（可通过 restore 恢复）。使用 --hard 永久删除。\n\n示例:\n  tier0 uns delete --path factory/line1/sensor/temp --yes\n  tier0 uns delete --path factory/line1/sensor/temp --path factory/line1/sensor/humi --yes\n  tier0 uns delete --path factory/line1/sensor/temp --hard --yes",
	),
	RunE: runUnsDelete,
}

func init() {
	unsDeleteCmd.Flags().StringArrayP("path", "p", nil,
		i18n.T("Node path(s) to delete (repeatable, required)", "要删除的节点路径（可重复指定，必填）"))
	unsDeleteCmd.Flags().StringArray("topic", nil,
		i18n.T("Deprecated alias for --path", "已废弃：请改用 --path"))
	_ = unsDeleteCmd.Flags().MarkHidden("topic")
	unsDeleteCmd.Flags().Bool("hard", false,
		i18n.T("Hard delete (irreversible)", "硬删除（不可逆）"))
	unsDeleteCmd.Flags().BoolP("yes", "y", false,
		i18n.T("Confirm high-risk operation (required)", "确认高风险操作（必填）"))
}

func runUnsDelete(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	confirmed, _ := cmd.Flags().GetBool("yes")
	topics, _ := cmd.Flags().GetStringArray("path")
	legacyTopics, _ := cmd.Flags().GetStringArray("topic")
	hard, _ := cmd.Flags().GetBool("hard")
	if len(legacyTopics) > 0 {
		if !jsonMode {
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.T(
				"warning: --topic is deprecated for uns delete; use --path",
				"警告: uns delete 的 --topic 已废弃，请改用 --path",
			))
		}
		topics = append(topics, legacyTopics...)
	}
	if len(topics) == 0 {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("%s", i18n.T(
			"specify at least one path via --path <path>",
			"请通过 --path <路径> 指定至少一个节点",
		)), jsonMode)
	}

	action := i18n.T("soft delete", "软删除")
	if hard {
		action = i18n.T("HARD DELETE", "硬删除")
	}
	summary := i18n.T(
		fmt.Sprintf("%s UNS topic(s) %v — %s.", action, topics, func() string {
			if hard {
				return "this is IRREVERSIBLE"
			}
			return "soft deleted items can be restored"
		}()),
		fmt.Sprintf("%s UNS 点位 %v — %s。", action, topics, func() string {
			if hard {
				return "操作不可逆"
			}
			return "软删除的项目可以恢复"
		}()),
	)
	if err := highrisk.Guard(confirmed, "uns delete", summary); err != nil {
		return err
	}

	payload := map[string]any{"topics": topics}
	if hard {
		payload["hard_delete"] = true
	}

	body := cmdutil.JSONString(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/delete", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(stdout, i18n.T("Topic(s) deleted: %v\n", "已删除点位: %v\n"), topics)
	}
	return nil
}
