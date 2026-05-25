package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: i18n.T("Manage Skills (list/update/version)", "管理 Skills（list/update/version）"),
	Long: i18n.T(
		"Manage locally installed AI agent skills. List, update, or check version.",
		"管理本地安装的 AI Agent Skills。支持列出、升级和查看版本。",
	),
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsUpdateCmd)
	skillsCmd.AddCommand(skillsVersionCmd)
}

var skillsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   i18n.T("List installed skills", "列出已安装的 skills"),
	RunE:    runSkillsList,
}

var skillsUpdateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"upgrade"},
	Short:   i18n.T("Upgrade skills to the latest version", "升级 skills 到最新版本"),
	RunE:    runSkillsUpdate,
}

var skillsVersionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"ver"},
	Short:   i18n.T("Show skills version info", "查看 skills 版本信息"),
	RunE:    runSkillsVersion,
}

func init() {
	skillsUpdateCmd.Flags().Bool("dry-run", false,
		i18n.T("Check for updates without installing", "只检查更新，不安装"))
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	skillsDir := upgrade.GetDefaultSkillsDir()
	result, err := upgrade.ListSkills(skillsDir)
	if err != nil {
		return fmt.Errorf(i18n.T("failed to list skills: %w", "列出 skills 失败: %w"), err)
	}

	if jsonMode {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	fmt.Fprintf(stdout, i18n.T("Skills version: %s\n", "Skills 版本: %s\n"), result.Version)
	if result.Skills == nil || len(result.Skills) == 0 {
		fmt.Fprintln(stdout, i18n.T("  (no skills installed)", "  (未安装 skills)"))
		fmt.Fprintln(stdout, i18n.T(
			"\nTip: download the full CLI package to get skills, or copy skill files manually",
			"\n提示: 下载 CLI 完整安装包可获取 skills，或手动复制 skill 文件到目录",
		))
		return nil
	}

	fmt.Fprintf(stdout, i18n.T("Installed %d skill(s):\n\n", "已安装 %d 个 skill:\n\n"), len(result.Skills))
	for _, s := range result.Skills {
		if s.Description != "" {
			fmt.Fprintf(stdout, "  %-26s %-12s %s\n", s.Name, s.Version, s.Description)
		} else {
			fmt.Fprintf(stdout, "  %-26s %s\n", s.Name, s.Version)
		}
	}
	return nil
}

func runSkillsUpdate(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	if version.IsDev() {
		fmt.Fprintln(stdout, i18n.T(
			"Development build (dev) — skills are updated with the source code, no separate upgrade needed.",
			"当前为开发版本 (dev)，skills 随源码更新，无需单独升级。",
		))
		return nil
	}

	skillsDir := upgrade.GetDefaultSkillsDir()
	if skillsDir == "" {
		skillsDir = upgrade.FallbackSkillsDir()
	}

	result, err := upgrade.UpdateSkills(skillsDir, dryRun)
	if err != nil {
		if jsonMode {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(stdout, string(output))
		} else {
			return fmt.Errorf(i18n.T("failed to upgrade skills: %w", "升级 skills 失败: %w"), err)
		}
		return err
	}

	if jsonMode {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	if result.UpToDate {
		fmt.Fprintf(stdout, i18n.T("Skills are up to date: %s\n", "Skills 已是最新版本 %s\n"), result.CurrentVersion)
		return nil
	}

	if dryRun {
		fmt.Fprintf(stdout, i18n.T(
			"New skills version available: %s → %s\n",
			"Skills 有新版本可用: %s → %s\n",
		), result.CurrentVersion, result.LatestVersion)
		fmt.Fprintln(stdout, i18n.T(
			"Run tier0 skills update to apply the upgrade",
			"运行 tier0 skills update 执行升级",
		))
		return nil
	}

	fmt.Fprintf(stdout, i18n.T(
		"Skills upgraded: %s → %s\n",
		"Skills 升级成功: %s → %s\n",
	), result.CurrentVersion, result.LatestVersion)
	fmt.Fprintf(stdout, i18n.T("Updated %d skill(s)\n", "已更新 %d 个 skill\n"), result.UpdatedCount)
	return nil
}

func runSkillsVersion(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	skillsDir := upgrade.GetDefaultSkillsDir()
	skillsVersion := upgrade.GetSkillsVersion(skillsDir)
	lastUpdated := upgrade.SkillsLastUpdated(skillsDir)

	if jsonMode {
		output := fmt.Sprintf(`{"version": %q, "updatedAt": %q}`, skillsVersion, lastUpdated)
		fmt.Fprintln(stdout, output)
		return nil
	}

	if skillsDir == "" {
		fmt.Fprintln(stdout, i18n.T("Skills directory not found", "未找到 skills 目录"))
		return nil
	}

	fmt.Fprintf(stdout, i18n.T("Skills version: %s\n", "Skills 版本: %s\n"), skillsVersion)
	if lastUpdated != "" {
		fmt.Fprintf(stdout, i18n.T("Last updated:   %s\n", "最后更新: %s\n"), lastUpdated)
	}
	fmt.Fprintf(stdout, i18n.T("Path:           %s\n", "路径: %s\n"), skillsDir)
	return nil
}
