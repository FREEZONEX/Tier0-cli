package cmd

import (
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: i18n.T("Inspect API key authentication", "查看 API Key 认证信息"),
	Long: i18n.T(
		"Inspect the current API key, workspace binding, and permissions.",
		"查看当前 API Key、Workspace 绑定和权限信息。",
	),
}

func init() {
	authCmd.AddCommand(authWhoamiCmd)
}
