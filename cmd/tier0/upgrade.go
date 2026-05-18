package tier0

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

func runUpgrade(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	opts := upgrade.Options{}
	jsonOutput := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			opts.DryRun = true
		case "--version", "-v":
			if i+1 < len(args) {
				opts.TargetVersion = args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		case "-h", "--help", "help":
			printUpgradeHelp(stdout)
			return nil
		}
	}

	if version.IsDev() {
		fmt.Fprintln(stdout, "当前为开发版本 (dev)，不支持自升级。")
		fmt.Fprintln(stdout, "如需升级，请重新从源码构建或下载 Release 安装包。")
		return nil
	}

	if opts.DryRun {
		result, err := upgrade.Check()
		if err != nil {
			return fmt.Errorf("检查更新失败: %w", err)
		}
		if jsonOutput {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(stdout, string(output))
			return nil
		}
		if result.UpToDate {
			fmt.Fprintf(stdout, "✓ 已是最新版本 %s\n", result.CurrentVersion)
		} else {
			fmt.Fprintf(stdout, "发现新版本: %s → %s\n", result.CurrentVersion, result.LatestVersion)
			fmt.Fprintf(stdout, "下载地址: %s\n", result.DownloadURL)
			fmt.Fprintln(stdout, "运行 tier0 upgrade 执行升级")
		}
		return nil
	}

	fmt.Fprintf(stdout, "正在检查更新（当前版本: %s）...\n", version.BuildVersion)

	result, err := upgrade.Perform(opts)
	if err != nil {
		if jsonOutput {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(stdout, string(output))
			return nil
		}
		if result.ErrorMessage != "" {
			return fmt.Errorf("升级失败: %s", result.ErrorMessage)
		}
		return fmt.Errorf("升级失败: %w", err)
	}

	if jsonOutput {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	if result.UpToDate {
		fmt.Fprintf(stdout, "✓ 已是最新版本 %s\n", result.CurrentVersion)
		return nil
	}

	fmt.Fprintf(stdout, "✓ 升级成功: %s → %s\n", result.CurrentVersion, result.LatestVersion)
	fmt.Fprintln(stdout, "请重新运行 tier0 以使用新版本。")
	fmt.Fprintf(stdout, "旧版本已备份至: %s\n", upgrade.BackupDir())
	return nil
}

func printUpgradeHelp(w io.Writer) {
	fmt.Fprintln(w, "用法: tier0 upgrade [选项]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "选项:")
	fmt.Fprintln(w, "  --dry-run          只检查更新，不安装")
	fmt.Fprintln(w, "  --version <ver>    升级到指定版本（默认最新）")
	fmt.Fprintln(w, "  --json             以 JSON 格式输出")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "示例:")
	fmt.Fprintln(w, "  tier0 upgrade --dry-run         检查是否有新版本")
	fmt.Fprintln(w, "  tier0 upgrade                   升级到最新版本")
	fmt.Fprintln(w, "  tier0 upgrade --version v0.2.0  升级到指定版本")
}
