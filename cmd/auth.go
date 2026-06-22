package cmd

import (
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Inspect API key authentication",
	Long:  "Inspect the current API key, workspace binding, and permissions.",
}

func init() {
	authCmd.AddCommand(authWhoamiCmd)
}
