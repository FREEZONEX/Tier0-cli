package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall tier0 CLI and agent skills",
	Long:  "Remove the tier0 binary, bundled skills, and Cursor/Claude agent skills.\nThe config file (~/.tier0/config.json) is kept by default; use --purge to delete it.",

	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().Bool("purge", false,
		"Also delete config file (credentials)")
	uninstallCmd.Flags().Bool("keep-skills", false,
		"Skip agent skills removal")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	purge, _ := cmd.Flags().GetBool("purge")
	keepSkills, _ := cmd.Flags().GetBool("keep-skills")
	stdout := cmd.OutOrStdout()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	tier0Dir := filepath.Join(home, ".tier0")
	binDir := filepath.Join(tier0Dir, "bin")
	skillsDir := filepath.Join(tier0Dir, "skills")
	configFile := filepath.Join(tier0Dir, "config.json")

	fmt.Fprintln(stdout, "Uninstalling tier0 CLI...")
	fmt.Fprintln(stdout)

	removed := 0

	// Remove binary
	binName := "tier0"
	if runtime.GOOS == "windows" {
		binName = "tier0.exe"
	}
	binPath := filepath.Join(binDir, binName)
	if removeFile(stdout, binPath, "binary") {
		removed++
	}
	removeFile(stdout, filepath.Join(binDir, ".version"), "version record")

	// Remove bin dir if empty
	if entries, err := os.ReadDir(binDir); err == nil && len(entries) == 0 {
		os.Remove(binDir)
	}

	// Remove bundled skills docs
	if removeDir(stdout, skillsDir, "bundled skills") {
		removed++
	}

	// Config handling
	if purge {
		if removeFile(stdout, configFile, "config (credentials)") {
			removed++
		}
		if entries, err := os.ReadDir(tier0Dir); err == nil && len(entries) == 0 {
			os.Remove(tier0Dir)
			fmt.Fprintf(stdout, "✓ Removed %s\n", tier0Dir)
		}
	} else {
		fmt.Fprintf(stdout,
			"\n  Config kept: %s\n  Run with --purge to also remove credentials.\n",
			configFile)
	}

	// Remove agent skills
	if !keepSkills {
		fmt.Fprintln(stdout, "\nRemoving agent skills...")
		if err := runNpxSkillsRemove(); err != nil {
			fmt.Fprintf(stdout,
				"⚠ Agent skills removal failed (non-fatal): %s\n  Run manually: npx skills remove FREEZONEX/Tier0-skill\n",
				err)
		} else {
			fmt.Fprintln(stdout, "✓ Agent skills removed.")
		}
	}

	if removed == 0 {
		fmt.Fprintln(stdout, "\ntier0 CLI was not installed (nothing to remove).")
	} else {
		fmt.Fprintln(stdout, "\ntier0 CLI uninstalled successfully.")
	}
	return nil
}

func removeFile(stdout interface{ Write([]byte) (int, error) }, path, label string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(stdout, "⚠ Failed to remove %s: %s\n", label, err)
		return false
	}
	fmt.Fprintf(stdout, "✓ Removed %s: %s\n", label, path)
	return true
}

func removeDir(stdout interface{ Write([]byte) (int, error) }, path, label string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	if err := os.RemoveAll(path); err != nil {
		fmt.Fprintf(stdout, "⚠ Failed to remove %s: %s\n", label, err)
		return false
	}
	fmt.Fprintf(stdout, "✓ Removed %s: %s\n", label, path)
	return true
}

func runNpxSkillsRemove() error {
	cmd := exec.Command("npx", "--yes", "skills", "remove", "FREEZONEX/Tier0-skill")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
