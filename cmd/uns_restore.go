package cmd

import (
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: i18n.T("Restore topic from history", "从历史数据恢复点位"),
	Long: i18n.T(
		"Restore a soft-deleted UNS topic. This is a high-risk operation.\n\nExamples:\n  tier0 uns restore --path Plant/Line1/Metric/Temperature --yes",
		"恢复被软删除的 UNS 点位。高风险操作。\n\n示例:\n  tier0 uns restore --path Plant/Line1/Metric/Temperature --yes",
	),
	RunE: runUnsRestore,
}

func init() {
	unsRestoreCmd.Flags().StringP("path", "p", "",
		i18n.T("Topic path to restore (required)", "要恢复的点位路径（必填）"))
	unsRestoreCmd.Flags().BoolP("yes", "y", false,
		i18n.T("Confirm high-risk operation (required)", "确认高风险操作（必填）"))
	unsRestoreCmd.MarkFlagRequired("path")
}

func runUnsRestore(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	confirmed, _ := cmd.Flags().GetBool("yes")
	path, _ := cmd.Flags().GetString("path")

	summary := i18n.T(
		fmt.Sprintf("Restore topic %q — this will recover the soft-deleted topic.", path),
		fmt.Sprintf("恢复点位 %q — 将恢复被软删除的点位。", path),
	)
	if err := highrisk.Guard(confirmed, "uns restore", summary); err != nil {
		return err
	}

	body := cmdutil.JSONString(map[string]any{
		"path": path,
	})

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/restore", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(stdout, i18n.T("Topic restored: %s\n", "点位已恢复: %s\n"), path)
	}
	return nil
}
