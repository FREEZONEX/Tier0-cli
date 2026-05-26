// Package cmd contains the tier0 CLI command tree built on Cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tier0",
	Short: "Tier0 Cloud Platform CLI",
	Long: i18n.T(
		"tier0 — Tier0 Cloud Platform CLI\n\nManage UNS topics, Node-RED flows, skills, and more.",
		"tier0 — Tier0 云平台命令行工具\n\n管理 UNS 点位、Node-RED Flow、Skills 等。",
	),
	SilenceUsage: true,
	Example: i18n.T(
		`  tier0 config --base-url https://tier0.dev
  tier0 login
  tier0 uns browse --path /
  tier0 uns read --topic demo
  tier0 flow list --source
  tier0 api /openapi/v1/uns/browse --body '{"path":"/"}'`,
		`  tier0 config --base-url https://tier0.dev
  tier0 config --lang zh
  tier0 login
  tier0 uns browse --path /
  tier0 uns read --topic demo
  tier0 flow list --source
  tier0 api /openapi/v1/uns/browse --body '{"path":"/"}'`,
	),
}

// Execute runs the root command.
func Execute() error {
	initLang()
	if rootCmd.PersistentPreRun != nil || rootCmd.PersistentPostRun != nil {
		_ = 0 // compiled but no-op for now; hooks ready for future use
	}
	return rootCmd.Execute()
}

// initLang loads the stored language preference.
// Priority: TIER0_LANG env > config file > default (en).
func initLang() {
	if envLang := os.Getenv("TIER0_LANG"); envLang != "" {
		i18n.SetLang(envLang)
		return
	}
	profile, err := config.LoadProfile()
	if err == nil && profile.Lang != "" {
		i18n.SetLang(profile.Lang)
	}
}

func init() {
	// Global persistent flags inherited by all subcommands.
	rootCmd.PersistentFlags().Bool("json", false,
		i18n.T("Output raw JSON", "输出原始 JSON"))
	rootCmd.PersistentFlags().Bool("debug", false,
		i18n.T("Print HTTP request/response details", "打印 HTTP 请求/响应详情"))

	// Top-level commands.
	rootCmd.AddCommand(apiCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(flowCmd)
	rootCmd.AddCommand(unsCmd)

	// Version is handled inline since it's trivial.
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: i18n.T("Show version", "显示版本"),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "tier0 version %s\n", version.BuildVersion)
		},
	})
}
