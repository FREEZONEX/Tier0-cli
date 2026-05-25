package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: i18n.T("Upgrade CLI to the latest version", "升级 CLI 到最新版本"),
	Long: i18n.T(
		"Check for and install the latest version of the tier0 CLI. Use --dry-run to check without installing.",
		"检查并安装最新版本的 tier0 CLI。使用 --dry-run 仅检查更新。",
	),
	RunE: runUpgrade,
}

func init() {
	upgradeCmd.Flags().Bool("dry-run", false,
		i18n.T("Check for updates without installing", "只检查更新，不安装"))
	upgradeCmd.Flags().StringP("version", "v", "",
		i18n.T("Upgrade to a specific version (default: latest)", "升级到指定版本（默认最新）"))
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	opts := upgrade.Options{}
	opts.DryRun, _ = cmd.Flags().GetBool("dry-run")
	opts.TargetVersion, _ = cmd.Flags().GetString("version")
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	if version.IsDev() {
		fmt.Fprintln(stdout, i18n.T(
			"Development build (dev) — self-upgrade is not supported.",
			"当前为开发版本 (dev)，不支持自升级。",
		))
		fmt.Fprintln(stdout, i18n.T(
			"Rebuild from source or download a release package to upgrade.",
			"如需升级，请重新从源码构建或下载 Release 安装包。",
		))
		return nil
	}

	if opts.DryRun {
		fmt.Fprintf(stdout, i18n.T(
			"Checking for updates (current: %s)...\n",
			"正在检查更新（当前版本: %s）...\n",
		), version.BuildVersion)
		result, err := upgrade.Perform(opts)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to check for updates: %w", "检查更新失败: %w"), err)
		}
		if jsonMode {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(stdout, string(output))
			return nil
		}
		if result.UpToDate {
			fmt.Fprintf(stdout, i18n.T("✓ Already up to date: %s\n", "✓ 已是最新版本 %s\n"), result.CurrentVersion)
		} else {
			fmt.Fprintf(stdout, i18n.T(
				"New version available: %s → %s\n",
				"发现新版本: %s → %s\n",
			), result.CurrentVersion, result.LatestVersion)
			fmt.Fprintf(stdout, i18n.T("Download: %s\n", "下载地址: %s\n"), result.DownloadURL)
			fmt.Fprintln(stdout, i18n.T("Run tier0 upgrade to install.", "运行 tier0 upgrade 执行升级"))
		}
		return nil
	}

	fmt.Fprintf(stdout, i18n.T(
		"Checking for updates (current: %s)...\n",
		"正在检查更新（当前版本: %s）...\n",
	), version.BuildVersion)

	result, err := upgrade.Perform(opts)
	if err != nil {
		if jsonMode {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(stdout, string(output))
			return nil
		}
		if result.ErrorMessage != "" {
			return fmt.Errorf(i18n.T("upgrade failed: %s", "升级失败: %s"), result.ErrorMessage)
		}
		return fmt.Errorf(i18n.T("upgrade failed: %w", "升级失败: %w"), err)
	}

	if jsonMode {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	if result.UpToDate {
		fmt.Fprintf(stdout, i18n.T("✓ Already up to date: %s\n", "✓ 已是最新版本 %s\n"), result.CurrentVersion)
		return nil
	}

	fmt.Fprintf(stdout, i18n.T(
		"✓ Upgraded: %s → %s\n",
		"✓ 升级成功: %s → %s\n",
	), result.CurrentVersion, result.LatestVersion)
	fmt.Fprintln(stdout, i18n.T(
		"Please restart tier0 to use the new version.",
		"请重新运行 tier0 以使用新版本。",
	))
	fmt.Fprintf(stdout, i18n.T(
		"Old binary backed up to: %s\n",
		"旧版本已备份至: %s\n",
	), upgrade.BackupDir())
	return nil
}
