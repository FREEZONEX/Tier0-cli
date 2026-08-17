package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const npmPackageName = "@tier0/cli"

var uninstallLookPath = exec.LookPath
var uninstallUserHomeDir = os.UserHomeDir

var uninstallRunCommand = func(name string, args []string, env []string) ([]byte, error) {
	command := exec.Command(name, args...)
	if env != nil {
		command.Env = env
	}
	return command.CombinedOutput()
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall tier0 CLI",
	Long:  "Remove the tier0 binary, bundled Skill baseline, and global npm package.\nAgent Skills and config (~/.tier0/config.json) are kept by default. Use --remove-skills and --purge to delete them.",

	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().Bool("purge", false,
		"Also delete config file (credentials)")
	uninstallCmd.Flags().Bool("remove-skills", false,
		"Also remove the Tier0 Skill from detected AI agents")
	uninstallCmd.Flags().Bool("keep-skills", false,
		"Keep agent skills (deprecated; this is now the default)")
	_ = uninstallCmd.Flags().MarkDeprecated("keep-skills",
		"agent skills are now kept by default; use --remove-skills to delete them")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	purge, _ := cmd.Flags().GetBool("purge")
	removeSkills, _ := cmd.Flags().GetBool("remove-skills")
	stdout := cmd.OutOrStdout()

	home, err := uninstallUserHomeDir()
	if err != nil {
		return internalCommandError(cmd, "cannot determine home directory: "+err.Error(), err)
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
	if removeBinary(stdout, binPath) {
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

	// An npm-based upgrade leaves a global wrapper package behind. Remove it
	// with the lifecycle cleanup disabled so this command does not recurse.
	npmRemoved, npmErr := removeGlobalNpmPackage(stdout)
	if npmRemoved {
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

	// Agent Skills have their own lifecycle and are preserved by default. This
	// mirrors other CLIs that let users update/reuse Skills independently.
	var skillsErr error
	if removeSkills {
		fmt.Fprintln(stdout, "\nRemoving agent skills...")
		if err := runNpxSkillsRemove(home); err != nil {
			skillsErr = err
			fmt.Fprintf(stdout,
				"⚠ Agent skills removal failed: %s\n  Run manually: npx -y --package=skills -- skills remove tier0 -y -g\n",
				err)
		} else {
			fmt.Fprintln(stdout, "✓ Agent skills removed.")
		}
	} else {
		fmt.Fprintln(stdout, "\n  Agent Skill kept. Use --remove-skills to delete it.")
	}

	if removed == 0 {
		fmt.Fprintln(stdout, "\ntier0 CLI was not installed (nothing to remove).")
	}
	if npmErr != nil || skillsErr != nil {
		parts := make([]string, 0, 2)
		if npmErr != nil {
			parts = append(parts, npmErr.Error())
		}
		if skillsErr != nil {
			parts = append(parts, skillsErr.Error())
		}
		return internalCommandError(cmd, "uninstall incomplete: "+strings.Join(parts, "; "), nil)
	}
	if removed > 0 {
		fmt.Fprintln(stdout, "\ntier0 CLI uninstalled successfully.")
	}
	return nil
}

func removeBinary(stdout interface{ Write([]byte) (int, error) }, path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	if err := os.Remove(path); err == nil {
		fmt.Fprintf(stdout, "✓ Removed binary: %s\n", path)
		return true
	} else if runtime.GOOS != "windows" {
		fmt.Fprintf(stdout, "⚠ Failed to remove binary: %s\n", err)
		return false
	}

	// Windows keeps the running executable locked. Only fall back to delayed
	// removal when this process is uninstalling itself; other access errors
	// should still be reported to the user.
	currentExe, err := os.Executable()
	if err != nil || !strings.EqualFold(filepath.Clean(currentExe), filepath.Clean(path)) {
		fmt.Fprintf(stdout, "⚠ Failed to remove binary: access denied: %s\n", path)
		return false
	}

	quotedPath := strings.ReplaceAll(path, "'", "''")
	script := fmt.Sprintf(
		"Start-Sleep -Milliseconds 500; Remove-Item -LiteralPath '%s' -Force -ErrorAction SilentlyContinue",
		quotedPath,
	)
	cleanup := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-WindowStyle", "Hidden",
		"-Command", script,
	)
	if err := cleanup.Start(); err != nil {
		fmt.Fprintf(stdout, "⚠ Failed to schedule binary removal: %s\n", err)
		return false
	}
	_ = cleanup.Process.Release()
	fmt.Fprintf(stdout, "✓ Scheduled binary removal: %s\n", path)
	return true
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

func skillsRemoveArgs() []string {
	return []string{"-y", "--package=skills", "--", "skills", "remove", "tier0", "-y", "-g"}
}

func runNpxSkillsRemove(home string) error {
	agentSkillDir := filepath.Join(home, ".agents", "skills", "tier0")
	if _, err := os.Stat(agentSkillDir); os.IsNotExist(err) {
		return nil
	}

	npxPath, err := uninstallLookPath("npx")
	if err != nil {
		return fmt.Errorf("npx is required to remove Agent Skills: %w", err)
	}
	output, err := uninstallRunCommand(npxPath, skillsRemoveArgs(), nil)
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("skills remove failed: %s", detail)
	}
	if _, err := os.Stat(agentSkillDir); err == nil {
		return fmt.Errorf("skills remove reported success but %s still exists", agentSkillDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot verify Agent Skill removal: %w", err)
	}
	return nil
}

func removeGlobalNpmPackage(stdout interface{ Write([]byte) (int, error) }) (bool, error) {
	npmPath, err := uninstallLookPath("npm")
	if err != nil {
		return false, nil
	}

	output, err := uninstallRunCommand(npmPath, []string{"root", "-g", "--json"}, nil)
	if err != nil {
		// Older npm versions do not support JSON for `npm root`; retry with its
		// stable text output before deciding that no global package is present.
		output, err = uninstallRunCommand(npmPath, []string{"root", "-g"}, nil)
		if err != nil {
			return false, fmt.Errorf("cannot locate global npm packages: %s", commandFailure(output, err))
		}
	}

	npmRoot := strings.TrimSpace(string(output))
	if strings.HasPrefix(npmRoot, "\"") {
		var decoded string
		if json.Unmarshal(output, &decoded) == nil {
			npmRoot = decoded
		}
	}
	packageFile := filepath.Join(npmRoot, "@tier0", "cli", "package.json")
	if _, err := os.Stat(packageFile); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("cannot inspect global npm package: %w", err)
	}

	env := append(os.Environ(), "TIER0_SKIP_UNINSTALL=1")
	output, err = uninstallRunCommand(npmPath, []string{"uninstall", "-g", npmPackageName}, env)
	if err != nil {
		return false, fmt.Errorf("global npm package removal failed: %s", commandFailure(output, err))
	}
	if _, err := os.Stat(packageFile); err == nil {
		return false, fmt.Errorf("npm reported success but global package still exists at %s", filepath.Dir(packageFile))
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("cannot verify global npm package removal: %w", err)
	}
	fmt.Fprintf(stdout, "✓ Removed global npm package: %s\n", npmPackageName)
	return true, nil
}

func commandFailure(output []byte, err error) string {
	detail := strings.TrimSpace(string(output))
	if detail != "" {
		return detail
	}
	return err.Error()
}
