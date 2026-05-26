package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: i18n.T("Uninstall tier0 CLI and agent skills", "卸载 tier0 CLI 及 Agent Skills"),
	Long: i18n.T(
		"Remove the tier0 binary, bundled skills, and Cursor/Claude agent skills.\nThe config file (~/.tier0/config.json) is kept by default; use --purge to delete it.",
		"移除 tier0 二进制、本地 skills 文档及 Cursor/Claude Agent Skills。\n默认保留配置文件（~/.tier0/config.json），使用 --purge 彻底清除。",
	),
	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().Bool("purge", false,
		i18n.T("Also delete config file (credentials)", "同时删除配置文件（含登录凭证）"))
	uninstallCmd.Flags().Bool("keep-skills", false,
		i18n.T("Skip agent skills removal", "跳过 Agent Skills 卸载"))
}

func runUninstall(cmd *cobra.Command, args []string) error {
	purge, _ := cmd.Flags().GetBool("purge")
	keepSkills, _ := cmd.Flags().GetBool("keep-skills")
	stdout := cmd.OutOrStdout()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf(i18n.T("cannot determine home directory: %w", "无法获取用户目录: %w"), err)
	}

	tier0Dir := filepath.Join(home, ".tier0")
	binDir := filepath.Join(tier0Dir, "bin")
	skillsDir := filepath.Join(tier0Dir, "skills")
	configFile := filepath.Join(tier0Dir, "config.json")

	fmt.Fprintln(stdout, i18n.T("Uninstalling tier0 CLI...\n", "正在卸载 tier0 CLI...\n"))

	removed := 0

	// Remove binary
	binName := "tier0"
	if runtime.GOOS == "windows" {
		binName = "tier0.exe"
	}
	binPath := filepath.Join(binDir, binName)
	if removeFile(stdout, binPath, i18n.T("binary", "二进制文件")) {
		removed++
	}
	removeFile(stdout, filepath.Join(binDir, ".version"), i18n.T("version record", "版本记录"))

	// Remove bin dir if empty
	if entries, err := os.ReadDir(binDir); err == nil && len(entries) == 0 {
		os.Remove(binDir)
	}

	// Remove bundled skills docs
	if removeDir(stdout, skillsDir, i18n.T("bundled skills", "本地 skills 文档")) {
		removed++
	}

	// Config handling
	if purge {
		if removeFile(stdout, configFile, i18n.T("config (credentials)", "配置文件（含凭证）")) {
			removed++
		}
		if entries, err := os.ReadDir(tier0Dir); err == nil && len(entries) == 0 {
			os.Remove(tier0Dir)
			fmt.Fprintf(stdout, i18n.T("✓ Removed %s\n", "✓ 已删除 %s\n"), tier0Dir)
		}
	} else {
		fmt.Fprintf(stdout, i18n.T(
			"\n  Config kept: %s\n  Run with --purge to also remove credentials.\n",
			"\n  配置文件已保留: %s\n  使用 --purge 可同时删除登录凭证。\n",
		), configFile)
	}

	// Remove agent skills
	if !keepSkills {
		fmt.Fprintln(stdout, i18n.T("\nRemoving agent skills...", "\n正在移除 Agent Skills..."))
		if err := runNpxSkillsRemove(); err != nil {
			fmt.Fprintf(stdout, i18n.T(
				"⚠ Agent skills removal failed (non-fatal): %s\n  Run manually: npx skills remove FREEZONEX/Tier0-skill\n",
				"⚠ Agent Skills 移除失败（非致命）: %s\n  可手动运行: npx skills remove FREEZONEX/Tier0-skill\n",
			), err)
		} else {
			fmt.Fprintln(stdout, i18n.T("✓ Agent skills removed.", "✓ Agent Skills 已移除。"))
		}
	}

	if removed == 0 {
		fmt.Fprintln(stdout, i18n.T("\ntier0 CLI was not installed (nothing to remove).", "\ntier0 CLI 未安装，无需卸载。"))
	} else {
		fmt.Fprintln(stdout, i18n.T("\ntier0 CLI uninstalled successfully.", "\ntier0 CLI 卸载完成。"))
	}
	return nil
}

func removeFile(stdout interface{ Write([]byte) (int, error) }, path, label string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(stdout, i18n.T("⚠ Failed to remove %s: %s\n", "⚠ 删除 %s 失败: %s\n"), label, err)
		return false
	}
	fmt.Fprintf(stdout, i18n.T("✓ Removed %s: %s\n", "✓ 已删除 %s: %s\n"), label, path)
	return true
}

func removeDir(stdout interface{ Write([]byte) (int, error) }, path, label string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	if err := os.RemoveAll(path); err != nil {
		fmt.Fprintf(stdout, i18n.T("⚠ Failed to remove %s: %s\n", "⚠ 删除 %s 失败: %s\n"), label, err)
		return false
	}
	fmt.Fprintf(stdout, i18n.T("✓ Removed %s: %s\n", "✓ 已删除 %s: %s\n"), label, path)
	return true
}

func runNpxSkillsRemove() error {
	cmd := exec.Command("npx", "--yes", "skills", "remove", "FREEZONEX/Tier0-skill")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
