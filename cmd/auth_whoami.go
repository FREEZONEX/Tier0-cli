package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: i18n.T("Show current API key identity", "查看当前 API Key 身份"),
	Long: i18n.T(
		"Show the user, workspace, key name, key type, roles, and permissions for the configured API key.",
		"查看当前配置 API Key 对应的用户、Workspace、Key 名称、Key 类型、角色和权限。",
	),
	RunE: runAuthWhoami,
}

func runAuthWhoami(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/auth/whoami", "POST", "{}", debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	if jsonMode {
		checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
		return nil
	}

	var result struct {
		UserID        int64    `json:"userID"`
		UserName      string   `json:"userName"`
		Email         string   `json:"email"`
		WorkspaceID   int64    `json:"workspaceID"`
		WorkspaceName string   `json:"workspaceName"`
		ApiKeyName    string   `json:"apiKeyName"`
		KeyPrefix     string   `json:"keyPrefix"`
		Permissions   []string `json:"permissions"`
		Roles         []string `json:"roles"`
		KeyType       string   `json:"keyType"`
	}
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}

	fmt.Fprintf(stdout, "%-16s %d\n", i18n.T("UserID:", "用户ID:"), result.UserID)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("UserName:", "用户名:"), result.UserName)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Email:", "邮箱:"), result.Email)
	fmt.Fprintf(stdout, "%-16s %d\n", i18n.T("WorkspaceID:", "工作区ID:"), result.WorkspaceID)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Workspace:", "工作区:"), result.WorkspaceName)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("API Key:", "API Key:"), result.ApiKeyName)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("KeyPrefix:", "Key 前缀:"), result.KeyPrefix)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("KeyType:", "Key 类型:"), result.KeyType)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Roles:", "角色:"), joinOrDash(result.Roles))
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Permissions:", "权限:"), joinOrDash(result.Permissions))
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}
