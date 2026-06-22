package cmd

import (
	"github.com/spf13/cobra"
)

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Manage Node-RED Flows",
	Long:  "Manage Node-RED Flows in a Workspace (SourceFlow / EventFlow).",
}

func init() {
	flowCmd.AddCommand(flowListCmd)
	flowCmd.AddCommand(flowGetCmd)
	flowCmd.AddCommand(flowCreateCmd)
	flowCmd.AddCommand(flowUpdateCmd)
	flowCmd.AddCommand(flowDeleteCmd)
	flowCmd.AddCommand(flowDataCmd)
	flowCmd.AddCommand(flowDeployCmd)
	flowCmd.AddCommand(flowNodesCmd)
}
