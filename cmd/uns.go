package cmd

import (
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/spf13/cobra"
)

var unsCmd = &cobra.Command{
	Use:   "uns",
	Short: i18n.T("UNS namespace operations", "UNS 命名空间操作"),
	Long: i18n.T(
		"Browse, read, write, and manage topics in the Unified Namespace (UNS).",
		"浏览、读取、写入和管理统一命名空间 (UNS) 中的点位。",
	),
}

func init() {
	unsCmd.AddCommand(unsBrowseCmd)
	unsCmd.AddCommand(unsReadCmd)
	unsCmd.AddCommand(unsWriteCmd)
	unsCmd.AddCommand(unsCreateCmd)
	unsCmd.AddCommand(unsUpdateCmd)
	unsCmd.AddCommand(unsDeleteCmd)
	unsCmd.AddCommand(unsSearchCmd)
	unsCmd.AddCommand(unsHistoryCmd)
	unsCmd.AddCommand(unsRestoreCmd)
	unsCmd.AddCommand(unsAttachmentsCmd)
	unsCmd.AddCommand(unsBindFlowCmd)
}
