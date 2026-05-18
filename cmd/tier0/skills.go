package tier0

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

func runSkills(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printSkillsHelp(stdout)
		return nil
	}

	switch args[0] {
	case "list", "ls":
		return runSkillsList(ctx, args[1:], stdout, stderr)
	case "update", "upgrade":
		return runSkillsUpdate(ctx, args[1:], stdout, stderr)
	case "version", "ver":
		return runSkillsVersion(ctx, args[1:], stdout, stderr)
	case "-h", "--help", "help":
		printSkillsHelp(stdout)
		return nil
	default:
		fmt.Fprintf(stderr, "未知 skills 子命令: %s\n", args[0])
		printSkillsHelp(stderr)
		return fmt.Errorf("unknown skills subcommand: %s", args[0])
	}
}

func runSkillsList(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		}
	}

	skillsDir := upgrade.GetDefaultSkillsDir()
	result, err := upgrade.ListSkills(skillsDir)
	if err != nil {
		return fmt.Errorf("列出 skills 失败: %w", err)
	}

	if jsonOutput {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	fmt.Fprintf(stdout, "Skills 版本: %s\n", result.Version)
	if result.Skills == nil || len(result.Skills) == 0 {
		fmt.Fprintln(stdout, "  (未安装 skills)")
		fmt.Fprintln(stdout, "\n提示: 下载 CLI 完整安装包可获取 skills，或手动复制 skill 文件到目录")
		return nil
	}

	fmt.Fprintf(stdout, "已安装 %d 个 skill:\n\n", len(result.Skills))
	for _, s := range result.Skills {
		if s.Description != "" {
			fmt.Fprintf(stdout, "  %-26s %-12s %s\n", s.Name, s.Version, s.Description)
		} else {
			fmt.Fprintf(stdout, "  %-26s %s\n", s.Name, s.Version)
		}
	}
	return nil
}

func runSkillsUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	dryRun := false
	jsonOutput := false
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--json":
			jsonOutput = true
		}
	}

	if version.IsDev() {
		fmt.Fprintln(stdout, "当前为开发版本 (dev)，skills 随源码更新，无需单独升级。")
		return nil
	}

	skillsDir := upgrade.GetDefaultSkillsDir()
	if skillsDir == "" {
		return fmt.Errorf("未找到 skills 目录，请先安装 CLI 完整包")
	}

	result, err := upgrade.UpdateSkills(skillsDir, dryRun)
	if err != nil {
		if jsonOutput {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(stdout, string(output))
		} else {
			return fmt.Errorf("升级 skills 失败: %w", err)
		}
		return err
	}

	if jsonOutput {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	if result.UpToDate {
		fmt.Fprintf(stdout, "Skills 已是最新版本 %s\n", result.CurrentVersion)
		return nil
	}

	if dryRun {
		fmt.Fprintf(stdout, "Skills 有新版本可用: %s → %s\n", result.CurrentVersion, result.LatestVersion)
		fmt.Fprintln(stdout, "运行 tier0 skills update 执行升级")
		return nil
	}

	fmt.Fprintf(stdout, "Skills 升级成功: %s → %s\n", result.CurrentVersion, result.LatestVersion)
	fmt.Fprintf(stdout, "已更新 %d 个 skill\n", result.UpdatedCount)
	return nil
}

func runSkillsVersion(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		}
	}

	skillsDir := upgrade.GetDefaultSkillsDir()
	skillsVersion := upgrade.GetSkillsVersion(skillsDir)
	lastUpdated := upgrade.SkillsLastUpdated(skillsDir)

	if jsonOutput {
		output := fmt.Sprintf(`{"version": %q, "updatedAt": %q}`, skillsVersion, lastUpdated)
		fmt.Fprintln(stdout, output)
		return nil
	}

	if skillsDir == "" {
		fmt.Fprintln(stdout, "未找到 skills 目录")
		return nil
	}

	parts := []string{fmt.Sprintf("Skills 版本: %s", skillsVersion)}
	if lastUpdated != "" {
		parts = append(parts, fmt.Sprintf("最后更新: %s", lastUpdated))
	}
	parts = append(parts, fmt.Sprintf("路径: %s", skillsDir))
	fmt.Fprintln(stdout, parts[0])
	for _, p := range parts[1:] {
		fmt.Fprintln(stdout, p)
	}
	return nil
}

func printSkillsHelp(w io.Writer) {
	fmt.Fprintln(w, "用法: tier0 skills <子命令> [选项]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "子命令:")
	fmt.Fprintln(w, "  list, ls        列出已安装的 skills")
	fmt.Fprintln(w, "  update, upgrade  升级 skills 到最新版本")
	fmt.Fprintln(w, "  version, ver     查看 skills 版本信息")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "选项:")
	fmt.Fprintln(w, "  --json           以 JSON 格式输出")
	fmt.Fprintln(w, "  --dry-run        只检查更新，不安装（仅 update）")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "示例:")
	fmt.Fprintln(w, "  tier0 skills list              列出所有已安装的 skills")
	fmt.Fprintln(w, "  tier0 skills update --dry-run  检查 skills 是否有更新")
	fmt.Fprintln(w, "  tier0 skills update            升级 skills 到最新版本")
	fmt.Fprintln(w, "  tier0 skills version           查看 skills 版本")
}
