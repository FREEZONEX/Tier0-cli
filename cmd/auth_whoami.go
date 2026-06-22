package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current API key identity",
	Long:  "Show the user, workspace, key name, key type, roles, and permissions for the configured API key.",

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

	fmt.Fprintf(stdout, "%-16s %d\n", "UserID:", result.UserID)
	fmt.Fprintf(stdout, "%-16s %s\n", "UserName:", result.UserName)
	fmt.Fprintf(stdout, "%-16s %s\n", "Email:", result.Email)
	fmt.Fprintf(stdout, "%-16s %d\n", "WorkspaceID:", result.WorkspaceID)
	fmt.Fprintf(stdout, "%-16s %s\n", "Workspace:", result.WorkspaceName)
	fmt.Fprintf(stdout, "%-16s %s\n", "API Key:", result.ApiKeyName)
	fmt.Fprintf(stdout, "%-16s %s\n", "KeyPrefix:", result.KeyPrefix)
	fmt.Fprintf(stdout, "%-16s %s\n", "KeyType:", result.KeyType)
	fmt.Fprintf(stdout, "%-16s %s\n", "Roles:", joinOrDash(result.Roles))
	fmt.Fprintf(stdout, "%-16s %s\n", "Permissions:", joinOrDash(result.Permissions))
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}
