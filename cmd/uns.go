package cmd

import (
	"github.com/spf13/cobra"
)

var unsCmd = &cobra.Command{
	Use:   "uns",
	Short: "UNS namespace operations",
	Long:  "Browse, read, write, and manage topics in the Unified Namespace (UNS).",
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
}
