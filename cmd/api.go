package cmd

import (
	"context"
	"os"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api <endpoint>",
	Short: i18n.T("Call an API endpoint directly", "直接调用 API 接口"),
	Long: i18n.T(
		"Call a backend API endpoint directly. Useful for debugging and advanced use.\n\nExamples:\n  tier0 api /openapi/v1/uns/browse --body '{\"path\":\"/\"}'\n  tier0 api /openapi/v1/uns/read --body '{\"topics\":[\"demo\"]}'\n  tier0 api /openapi/v1/uns/write --body-file body.json",
		"直接调用后端 API 接口。用于调试和高级场景。\n\n示例:\n  tier0 api /openapi/v1/uns/browse --body '{\"path\":\"/\"}'\n  tier0 api /openapi/v1/uns/read --body '{\"topics\":[\"demo\"]}'\n  tier0 api /openapi/v1/uns/write --body-file body.json",
	),
	Args: cobra.MinimumNArgs(1),
	RunE: runAPI,
}

func init() {
	apiCmd.Flags().String("body", "", i18n.T("Request body JSON string", "请求体 JSON 字符串"))
	apiCmd.Flags().String("body-file", "", i18n.T("Read request body from file", "从文件读取请求体"))
	apiCmd.Flags().String("method", "POST", i18n.T("HTTP method (GET|POST|PUT|DELETE)", "HTTP 方法"))
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
