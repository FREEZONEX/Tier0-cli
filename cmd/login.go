package cmd

import (
	"fmt"
	"io"

	"github.com/FREEZONEX/Tier0-cli/internal/auth"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: i18n.T("Authenticate via Device Flow", "Device Flow 登录授权"),
	Long: i18n.T(
		"Authenticate to the Tier0 platform using Device Flow. Opens a browser for authorization and retrieves a Personal API Key.",
		"通过 Device Flow 登录 Tier0 平台。打开浏览器完成授权后自动获取 Personal API Key。",
	),
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().Bool("no-wait", false,
		i18n.T("Print the authorization URL and exit without polling", "仅打印授权地址，不等待完成"))
	loginCmd.Flags().String("setup-code", "",
		i18n.T("Poll for an existing setup code instead of creating a new one", "使用已有的 setup code 轮询结果"))
	loginCmd.Flags().String("base-url", "",
		i18n.T("Override the platform base URL", "覆盖平台地址"))
}

func runLogin(cmd *cobra.Command, args []string) error {
	noWait, _ := cmd.Flags().GetBool("no-wait")
	setupCode, _ := cmd.Flags().GetString("setup-code")
	jsonMode, _ := cmd.Flags().GetBool("json")
	baseURLArg, _ := cmd.Flags().GetString("base-url")
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	baseURL := cmdutil.ResolveBaseURL(baseURLArg)

	if setupCode != "" {
		return loginPoll(cmd, baseURL, setupCode, jsonMode, stdout, stderr)
	}

	setupCode = auth.GenerateSetupCode()
	consoleURL := auth.BuildConsoleURL(baseURL, setupCode)

	if noWait {
		if jsonMode {
			fmt.Fprintf(stdout, `{"status":"authorization_required","verification_url":"%s","setup_code":"%s","expires_in":600}`+"\n", consoleURL, setupCode)
		} else {
			fmt.Fprintf(stdout, i18n.T(
				"Please complete authorization in your browser: %s\n",
				"请在浏览器中完成授权：%s\n",
			), consoleURL)
			fmt.Fprintf(stdout, i18n.T(
				"After authorization, run: tier0 login --setup-code %s\n",
				"授权完成后执行: tier0 login --setup-code %s\n",
			), setupCode)
		}
		return nil
	}

	fmt.Fprintln(stdout, i18n.T(
		"Please complete authorization in your browser:",
		"请在浏览器中完成授权：",
	))
	fmt.Fprintln(stdout, consoleURL)
	fmt.Fprintln(stdout, i18n.T(
		"\nWaiting for authorization... (polling every 5s, up to 10 minutes)",
		"\n正在等待授权...（每5秒检测一次，最多10分钟）",
	))

	return loginPoll(cmd, baseURL, setupCode, jsonMode, stdout, stderr)
}

func loginPoll(cmd *cobra.Command, baseURL, setupCode string, jsonMode bool, stdout, stderr io.Writer) error {
	result, err := auth.PollSetupCheck(cmd.Context(), baseURL, setupCode, func(current, total int, done bool, pollErr error) {
		if jsonMode || done {
			return
		}
		if pollErr != nil {
			if current == 0 {
				fmt.Fprintf(stdout, i18n.T(
					"\r  Polling... (%d/%d) Network hiccup, retrying...\n",
					"\r  正在检测...（第 %d/%d 次）网络暂时不稳定，继续等待...\n",
				), current+1, total)
			}
			return
		}
		if current%6 == 0 && current > 0 {
			remainingMin := (total - current) * 5 / 60
			fmt.Fprintf(stdout, i18n.T(
				"\r  Waiting for authorization... (check %d/%d, ~%d min remaining)",
				"\r  正在等待授权...（第 %d/%d 次检测，剩余约 %d 分钟）",
			), current+1, total, remainingMin)
		}
	})
	if err != nil {
		return cmdutil.HandleCommandError(stderr, err, jsonMode)
	}

	profile := config.Profile{
		BaseURL: baseURL,
		APIKey:  result.APIKey,
	}
	if err := config.SaveProfile(profile); err != nil {
		return cmdutil.HandleCommandError(stderr, fmt.Errorf(i18n.T(
			"failed to save config: %w",
			"保存配置失败: %w",
		), err), jsonMode)
	}

	if jsonMode {
		fmt.Fprintf(stdout, `{"event":"authorization_complete","api_key":"%s"}`+"\n", result.APIKey)
	} else {
		fmt.Fprintln(stdout, i18n.T("\n✓ Authorization successful!", "\n✓ 授权成功！"))
		fmt.Fprintf(stdout, i18n.T(
			"API Key: %s... (saved)\n",
			"API Key: %s...（已保存）\n",
		), result.APIKey[:8])
		fmt.Fprintln(stdout, i18n.T(
			"✓ Setup complete. You can now use tier0 api commands.",
			"✓ 初始化完成，您现在可以使用 tier0 api 命令了。",
		))
	}
	return nil
}
