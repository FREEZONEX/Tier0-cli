package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade CLI to the latest version",
	Long:  "Check for and install the latest version of the tier0 CLI. Use --dry-run to check without installing.",

	RunE: runUpgrade,
}

func init() {
	upgradeCmd.Flags().Bool("dry-run", false,
		"Check for updates without installing")
	upgradeCmd.Flags().StringP("version", "v", "",
		"Upgrade to a specific version (default: latest)")
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	opts := upgrade.Options{}
	opts.DryRun, _ = cmd.Flags().GetBool("dry-run")
	opts.TargetVersion, _ = cmd.Flags().GetString("version")
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	if version.IsDev() {
		fmt.Fprintln(stdout,
			"Development build (dev) — self-upgrade is not supported.",
		)
		fmt.Fprintln(stdout,
			"Rebuild from source or download a release package to upgrade.",
		)
		return nil
	}

	if opts.DryRun {
		fmt.Fprintf(stdout,
			"Checking for updates (current: %s)...\n",
			version.BuildVersion)
		result, err := upgrade.Perform(opts)
		if err != nil {
			return internalCommandError(cmd, "failed to check for updates: "+err.Error(), err)
		}
		if jsonMode {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(stdout, string(output))
			return nil
		}
		if result.UpToDate {
			fmt.Fprintf(stdout, "✓ Already up to date: %s\n", result.CurrentVersion)
		} else {
			fmt.Fprintf(stdout,
				"New version available: %s → %s\n",
				result.CurrentVersion, result.LatestVersion)
			fmt.Fprintf(stdout, "Download: %s\n", result.DownloadURL)
			fmt.Fprintln(stdout, "Run tier0 upgrade to install.")
		}
		return nil
	}

	fmt.Fprintf(stdout,
		"Checking for updates (current: %s)...\n",
		version.BuildVersion)

	result, err := upgrade.Perform(opts)
	if err != nil {
		if result.ErrorMessage != "" {
			return internalCommandError(cmd, "upgrade failed: "+result.ErrorMessage, err)
		}
		return internalCommandError(cmd, "upgrade failed: "+err.Error(), err)
	}

	if jsonMode {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	if result.UpToDate {
		fmt.Fprintf(stdout, "✓ Already up to date: %s\n", result.CurrentVersion)
		return nil
	}

	via := result.Method
	if via == "" {
		via = "github"
	}
	fmt.Fprintf(stdout,
		"✓ Upgraded: %s → %s (via %s)\n",
		result.CurrentVersion, result.LatestVersion, via)
	fmt.Fprintln(stdout,
		"Please restart tier0 to use the new version.",
	)
	switch result.SkillStatus {
	case "synced":
		fmt.Fprintln(stdout, "Embedded Skill installed and synced to detected agents.")
	case "installed":
		fmt.Fprintf(stdout, "Embedded Skill installed; Agent Skills sync needs a retry: %s\n", result.SkillError)
	case "failed":
		fmt.Fprintf(stdout, "Warning: CLI upgraded, but embedded Skill install failed: %s\n", result.SkillError)
	}
	if via != "npm" {
		fmt.Fprintf(stdout,
			"Old binary backed up to: %s\n",
			upgrade.BackupDir())
	}
	return nil
}
