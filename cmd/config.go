package cmd

import (
	"fmt"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: i18n.T("View or update configuration", "查看/管理配置"),
	Long: i18n.T(
		"View or update CLI configuration (base URL, language). Without flags, displays the current settings.",
		"查看或修改 CLI 配置（平台地址、语言）。不带参数时显示当前设置。",
	),
	RunE: runConfig,
}

func init() {
	configCmd.Flags().String("base-url", "",
		i18n.T("Set the platform base URL", "设置平台地址"))
	configCmd.Flags().String("lang", "",
		i18n.T("Set UI language: en | zh", "设置界面语言: en | zh"))
}

func runConfig(cmd *cobra.Command, args []string) error {
	setBaseURL, _ := cmd.Flags().GetString("base-url")
	setLang, _ := cmd.Flags().GetString("lang")
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	if setBaseURL != "" || setLang != "" {
		if setLang != "" {
			setLang = strings.ToLower(strings.TrimSpace(setLang))
			if setLang != "en" && setLang != "zh" {
				return fmt.Errorf(i18n.T(
					"unsupported language %q, use: en | zh",
					"不支持的语言 %q，可选: en | zh",
				), setLang)
			}
		}

		profile, _ := config.LoadProfile()
		if setBaseURL != "" {
			profile.BaseURL = strings.TrimRight(setBaseURL, "/")
		}
		if setLang != "" {
			profile.Lang = setLang
		}
		if err := config.SaveProfile(profile); err != nil {
			return fmt.Errorf(i18n.T("failed to save config: %w", "保存配置失败: %w"), err)
		}
		if setBaseURL != "" {
			fmt.Fprintf(stdout, i18n.T("✓ BaseURL set to: %s\n", "✓ BaseURL 已设置为: %s\n"), strings.TrimRight(setBaseURL, "/"))
		}
		if setLang != "" {
			i18n.SetLang(setLang)
			fmt.Fprintf(stdout, i18n.T("✓ Language set to: %s\n", "✓ 语言已设置为: %s\n"), setLang)
		}
		return nil
	}

	// Read mode
	profile, err := config.LoadProfile()
	if err != nil {
		return fmt.Errorf(i18n.T("failed to load config: %w", "加载配置失败: %w"), err)
	}

	baseURL := cmdutil.ResolveBaseURL("")
	if profile.BaseURL != "" {
		baseURL = profile.BaseURL
	}
	lang := profile.Lang
	if lang == "" {
		lang = "en"
	}

	fmt.Fprintf(stdout, i18n.T("BaseURL:  %s\n", "BaseURL:  %s\n"), baseURL)
	fmt.Fprintf(stdout, i18n.T("Language: %s\n", "语言:     %s\n"), lang)
	if profile.APIKey != "" {
		fmt.Fprintf(stdout, i18n.T("API Key:  %s...\n", "API Key:  %s...\n"), profile.APIKey[:8])
	} else {
		fmt.Fprintln(stdout, i18n.T("API Key:  (not set)", "API Key:  (未设置)"))
	}
	_ = stderr
	return nil
}
