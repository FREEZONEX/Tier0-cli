package cmd

import (
	"context"
	"os"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api <endpoint>",
	Short: "Call an API endpoint directly",
	Long:  "Call a backend API endpoint directly. Useful for debugging and advanced use.\n\nExamples:\n  tier0 api /openapi/v1/uns/browse --body '{\"path\":\"/\"}'\n  tier0 api /openapi/v1/uns/read --body '{\"topics\":[\"demo\"]}'\n  tier0 api /openapi/v1/uns/write --body-file body.json",

	Args: cobra.MinimumNArgs(1),
	RunE: runAPI,
}

func init() {
	apiCmd.Flags().String("body", "", "Request body JSON string")
	apiCmd.Flags().String("body-file", "", "Read request body from file")
	apiCmd.Flags().String("method", "POST", "HTTP method (GET|POST|PUT|DELETE)")
}

func runAPI(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	endpoint := args[0]
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	body, _ := cmd.Flags().GetString("body")
	bodyFile, _ := cmd.Flags().GetString("body-file")
	method, _ := cmd.Flags().GetString("method")

	if bodyFile != "" {
		raw, err := os.ReadFile(bodyFile)
		if err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
		}
		body = string(raw)
	}

	resp, err := cmdutil.DoAPI(cmd.Context(), endpoint, method, body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	checker.Emit(resp, jsonMode, cmd.OutOrStdout(), cmd.ErrOrStderr())
	if !jsonMode {
		cmd.OutOrStdout().Write([]byte(resp + "\n"))
	}
	return nil
}

var _ = context.Background // ensure context import is used
