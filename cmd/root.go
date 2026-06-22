// Package cmd contains the tier0 CLI command tree built on Cobra.
package cmd

import (
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tier0",
	Short: "Tier0 Cloud Platform CLI",
	Long:  "tier0 — Tier0 Cloud Platform CLI\n\nManage UNS topics, Node-RED flows, skills, and more.",

	SilenceUsage:  true,
	SilenceErrors: true, // main() owns all stderr output; prevent Cobra double-printing
	Example: `  tier0 config --base-url https://tier0.dev
  tier0 login
  tier0 uns browse --path /
  tier0 uns read --topic demo
  tier0 flow list --source
  tier0 api /openapi/v1/uns/browse --body '{"path":"/"}'`,
}

// Execute runs the root command.
func Execute() error {
	if rootCmd.PersistentPreRun != nil || rootCmd.PersistentPostRun != nil {
		_ = 0 // compiled but no-op for now; hooks ready for future use
	}
	return rootCmd.Execute()
}

func init() {
	// Global persistent flags inherited by all subcommands.
	rootCmd.PersistentFlags().Bool("json", false,
		"Output raw JSON")
	rootCmd.PersistentFlags().Bool("debug", false,
		"Print HTTP request/response details")

	// Top-level commands.
	rootCmd.AddCommand(apiCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(flowCmd)
	rootCmd.AddCommand(unsCmd)

	// Version is handled inline since it's trivial.
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "tier0 version %s\n", version.BuildVersion)
		},
	})

}
