package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage Skills (list/update/version)",
	Long:  "Manage locally installed AI agent skills. List, update, or check version.",
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsUpdateCmd)
	skillsCmd.AddCommand(skillsVersionCmd)
}

var skillsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installed skills",
	RunE:    runSkillsList,
}

var skillsUpdateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"upgrade"},
	Short:   "Upgrade skills to the latest version",
	RunE:    runSkillsUpdate,
}

var skillsVersionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"ver"},
	Short:   "Show skills version info",
	RunE:    runSkillsVersion,
}

func init() {
	skillsUpdateCmd.Flags().Bool("dry-run", false,
		"Check for updates without installing")
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	skillsDir := upgrade.GetDefaultSkillsDir()
	result, err := upgrade.ListSkills(skillsDir)
	if err != nil {
		return fmt.Errorf("failed to list skills: %w", err)
	}

	if jsonMode {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	fmt.Fprintf(stdout, "Skills version: %s\n", result.Version)
	if result.Skills == nil || len(result.Skills) == 0 {
		fmt.Fprintln(stdout, "  (no skills installed)")
		fmt.Fprintln(stdout,
			"\nTip: download the full CLI package to get skills, or copy skill files manually",
		)
		return nil
	}

	fmt.Fprintf(stdout, "Installed %d skill(s):\n\n", len(result.Skills))
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
		fmt.Fprintln(stdout,
			"Development build (dev) — skills are updated with the source code, no separate upgrade needed.",
		)
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
			return fmt.Errorf("failed to upgrade skills: %w", err)
		}
		return err
	}

	if jsonMode {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	if result.UpToDate {
		fmt.Fprintf(stdout, "Skills are up to date: %s\n", result.CurrentVersion)
		return nil
	}

	if dryRun {
		fmt.Fprintf(stdout,
			"New skills version available: %s → %s\n",
			result.CurrentVersion, result.LatestVersion)
		fmt.Fprintln(stdout,
			"Run tier0 skills update to apply the upgrade",
		)
		return nil
	}

	fmt.Fprintf(stdout,
		"Skills upgraded: %s → %s\n",
		result.CurrentVersion, result.LatestVersion)
	fmt.Fprintf(stdout, "Updated %d skill(s)\n", result.UpdatedCount)
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
		fmt.Fprintln(stdout, "Skills directory not found")
		return nil
	}

	fmt.Fprintf(stdout, "Skills version: %s\n", skillsVersion)
	if lastUpdated != "" {
		fmt.Fprintf(stdout, "Last updated:   %s\n", lastUpdated)
	}
	fmt.Fprintf(stdout, "Path:           %s\n", skillsDir)
	return nil
}
