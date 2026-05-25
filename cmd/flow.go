package cmd

import (
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/spf13/cobra"
)

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: i18n.T("Manage Node-RED Flows", "管理 Node-RED Flow"),
	Long: i18n.T(
		"Manage Node-RED Flows in a Workspace (SourceFlow / EventFlow).",
		"管理 Workspace 中的 Node-RED Flow（SourceFlow / EventFlow）。",
	),
}

func init() {
	flowCmd.AddCommand(flowListCmd)
	flowCmd.AddCommand(flowGetCmd)
	flowCmd.AddCommand(flowCreateCmd)
	flowCmd.AddCommand(flowUpdateCmd)
	flowCmd.AddCommand(flowDeleteCmd)
	flowCmd.AddCommand(flowDataCmd)
	flowCmd.AddCommand(flowDeployCmd)
}
