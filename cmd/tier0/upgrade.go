package tier0

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

func runUpgrade(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	opts := upgrade.Options{}
	jsonOutput := false
	skipSkills := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			opts.DryRun = true
		case "--version", "-v":
			if i+1 < len(args) {
				opts.TargetVersion = args[i+1]
				i++
			}
		case "--skip-skills":
			skipSkills = true
		case "--json":
			jsonOutput = true
		case "-h", "--help", "help":
			printUpgradeHelp(stdout)
			return nil
		}
	}

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
		// --dry-run uses Perform() (same live GitHub query as real upgrade)
		// so it always reflects the actual latest release, not a stale cache.
		fmt.Fprintf(stdout, i18n.T(
			"Checking for updates (current: %s)...\n",
			"正在检查更新（当前版本: %s）...\n",
		), version.BuildVersion)
		result, err := upgrade.Perform(opts)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to check for updates: %w", "检查更新失败: %w"), err)
		}
		if jsonOutput {
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

		// Also check skills update status in dry-run mode.
		if !skipSkills {
			skillsDir := upgrade.GetDefaultSkillsDir()
			if skillsDir == "" {
				skillsDir = upgrade.FallbackSkillsDir()
			}
			skillsResult, serr := upgrade.CheckSkillsUpdate(skillsDir)
			if serr == nil && skillsResult != nil && !skillsResult.UpToDate {
				fmt.Fprintf(stdout, i18n.T(
					"Skills update available: %s → %s (run 'tier0 skills update' or 'tier0 upgrade' to apply)\n",
					"Skills 有更新可用: %s → %s（运行 'tier0 skills update' 或 'tier0 upgrade' 应用）\n",
				), skillsResult.CurrentVersion, skillsResult.LatestVersion)
			}
		}
		return nil
	}

	fmt.Fprintf(stdout, i18n.T(
		"Checking for updates (current: %s)...\n",
		"正在检查更新（当前版本: %s）...\n",
	), version.BuildVersion)

	result, err := upgrade.Perform(opts)
	if err != nil {
		if jsonOutput {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(stdout, string(output))
			return nil
		}
		if result.ErrorMessage != "" {
			return fmt.Errorf(i18n.T("upgrade failed: %s", "升级失败: %s"), result.ErrorMessage)
		}
		return fmt.Errorf(i18n.T("upgrade failed: %w", "升级失败: %w"), err)
	}

	if jsonOutput {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	if result.UpToDate {
		fmt.Fprintf(stdout, i18n.T("✓ Already up to date: %s\n", "✓ 已是最新版本 %s\n"), result.CurrentVersion)
	} else {
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
	}

	// Always update skills alongside the CLI upgrade (unless --skip-skills).
	if !skipSkills {
		skillsDir := upgrade.GetDefaultSkillsDir()
		if skillsDir == "" {
			skillsDir = upgrade.FallbackSkillsDir()
		}
		fmt.Fprintln(stdout, i18n.T(
			"\nUpdating skills...",
			"\n正在更新 skills...",
		))
		skillsResult, serr := upgrade.UpdateSkills(skillsDir, false)
		if serr != nil {
			fmt.Fprintf(stderr, i18n.T(
				"Warning: skills update failed: %v (run 'tier0 skills update' to retry)\n",
				"警告：skills 更新失败: %v（运行 'tier0 skills update' 重试）\n",
			), serr)
		} else if skillsResult.UpToDate {
			fmt.Fprintf(stdout, i18n.T(
				"✓ Skills already up to date: %s\n",
				"✓ Skills 已是最新版本 %s\n",
			), skillsResult.CurrentVersion)
		} else {
			fmt.Fprintf(stdout, i18n.T(
				"✓ Skills updated: %s → %s (%d skill(s))\n",
				"✓ Skills 更新成功: %s → %s（共 %d 个 skill）\n",
			), skillsResult.CurrentVersion, skillsResult.LatestVersion, skillsResult.UpdatedCount)
		}
	}

	return nil
}

func printUpgradeHelp(w io.Writer) {
	fmt.Fprintln(w, i18n.T("Usage: tier0 upgrade [flags]", "用法: tier0 upgrade [选项]"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T(
		"Upgrades the tier0 CLI binary and skills documentation together.",
		"同时升级 tier0 CLI 二进制和 skills 文档。",
	))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags:", "选项:"))
	fmt.Fprintln(w, i18n.T("  --dry-run          Check for updates without installing", "  --dry-run          只检查更新，不安装"))
	fmt.Fprintln(w, i18n.T("  --version <ver>    Upgrade to a specific version (default: latest)", "  --version <ver>    升级到指定版本（默认最新）"))
	fmt.Fprintln(w, i18n.T("  --skip-skills      Upgrade CLI only, skip skills update", "  --skip-skills      只升级 CLI，跳过 skills 更新"))
	fmt.Fprintln(w, i18n.T("  --json             Output as JSON", "  --json             以 JSON 格式输出"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Examples:", "示例:"))
	fmt.Fprintln(w, i18n.T("  tier0 upgrade --dry-run         check CLI and skills for updates", "  tier0 upgrade --dry-run         检查 CLI 和 skills 是否有更新"))
	fmt.Fprintln(w, i18n.T("  tier0 upgrade                   upgrade CLI and skills to latest", "  tier0 upgrade                   升级 CLI 和 skills 到最新版本"))
	fmt.Fprintln(w, i18n.T("  tier0 upgrade --skip-skills     upgrade CLI only", "  tier0 upgrade --skip-skills     只升级 CLI"))
	fmt.Fprintln(w, i18n.T("  tier0 upgrade --version v0.2.0  upgrade to a specific version", "  tier0 upgrade --version v0.2.0  升级到指定版本"))
}
