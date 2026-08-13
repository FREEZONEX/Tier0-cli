package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/FREEZONEX/Tier0-cli/internal/upgrade"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage Skills (install/list/update/status/sync)",
	Long:  "Manage locally installed AI agent skills. Install the embedded baseline, update it independently, inspect status, or sync it to detected agents.",
}

func init() {
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsUpdateCmd)
	skillsCmd.AddCommand(skillsVersionCmd)
	skillsCmd.AddCommand(skillsStatusCmd)
	skillsCmd.AddCommand(skillsSyncCmd)
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install or repair the Skill embedded in this CLI",
	RunE:  runSkillsInstall,
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

var skillsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active and embedded Skill status",
	RunE:  runSkillsStatus,
}

var skillsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync the active Skill to detected AI agents",
	RunE:  runSkillsSync,
}

func init() {
	skillsInstallCmd.Flags().Bool("force", false,
		"Replace the active Skill with the embedded baseline")
	skillsInstallCmd.Flags().Bool("no-sync", false,
		"Install locally without syncing detected AI agents")
	skillsUpdateCmd.Flags().Bool("dry-run", false,
		"Check for updates without installing")
}

func runSkillsInstall(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	noSync, _ := cmd.Flags().GetBool("no-sync")
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	result, err := upgrade.EnsureEmbeddedSkills(upgrade.FallbackSkillsDir(), force)
	if err != nil {
		return internalCommandError(cmd, "failed to install embedded Skill: "+err.Error(), err)
	}

	syncStatus := "skipped"
	syncError := ""
	if !noSync {
		if err := upgrade.SyncAgentSkills(result.Path); err != nil {
			syncStatus = "failed"
			syncError = err.Error()
		} else {
			syncStatus = "synced"
		}
	}

	if jsonMode {
		output := struct {
			*upgrade.EmbeddedSkillsResult
			AgentSyncStatus string `json:"agentSyncStatus"`
			AgentSyncError  string `json:"agentSyncError,omitempty"`
		}{result, syncStatus, syncError}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Fprintln(stdout, string(data))
	} else {
		fmt.Fprintf(stdout, "Skill %s: %s (%s)\n", result.Action, result.Version, result.Source)
		fmt.Fprintf(stdout, "Path: %s\n", result.Path)
		if syncStatus == "synced" {
			fmt.Fprintln(stdout, "Agent Skills synced to detected global agents")
		} else if syncStatus == "failed" {
			fmt.Fprintf(stdout, "Warning: local Skill is ready, but Agent Skills sync failed: %s\n", syncError)
		}
	}
	if syncStatus == "failed" {
		return fmt.Errorf("Agent Skills sync failed: %s", syncError)
	}
	return nil
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	skillsDir := upgrade.GetDefaultSkillsDir()
	result, err := upgrade.ListSkills(skillsDir)
	if err != nil {
		return internalCommandError(cmd, "failed to list skills: "+err.Error(), err)
	}

	if jsonMode {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	fmt.Fprintf(stdout, "Skills version: %s\n", result.Version)
	if result.Skills == nil || len(result.Skills) == 0 {
		fmt.Fprintln(stdout, "  (no skills installed)")
		fmt.Fprintln(stdout, "\nRun tier0 skills install to restore the embedded baseline")
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
		return internalCommandError(cmd, "failed to upgrade skills: "+err.Error(), err)
	}
	if !dryRun {
		if err := upgrade.SyncAgentSkills(skillsDir); err != nil {
			result.AgentSyncStatus = "failed"
			result.AgentSyncError = err.Error()
		} else {
			result.AgentSyncStatus = "synced"
		}
	}

	if jsonMode {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	if result.UpToDate {
		fmt.Fprintf(stdout, "Skills are up to date: %s\n", result.CurrentVersion)
		printAgentSkillsSync(stdout, result)
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
	printAgentSkillsSync(stdout, result)
	return nil
}

func printAgentSkillsSync(stdout io.Writer, result *upgrade.SkillsUpdateResult) {
	switch result.AgentSyncStatus {
	case "synced":
		fmt.Fprintln(stdout, "Agent Skills synced to detected global agents")
	case "failed":
		fmt.Fprintf(stdout, "Warning: local Skills updated, but Agent Skills sync failed: %s\n", result.AgentSyncError)
	}
}

func runSkillsVersion(cmd *cobra.Command, args []string) error {
	return runSkillsStatus(cmd, args)
}

func runSkillsStatus(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	skillsDir := upgrade.GetDefaultSkillsDir()
	if skillsDir == "" {
		skillsDir = upgrade.FallbackSkillsDir()
	}
	status := upgrade.GetSkillsStatus(skillsDir)

	if jsonMode {
		output, _ := json.MarshalIndent(status, "", "  ")
		fmt.Fprintln(stdout, string(output))
		return nil
	}

	if !status.Installed {
		fmt.Fprintln(stdout, "Skills are not installed")
		fmt.Fprintf(stdout, "Embedded baseline: %s\n", status.EmbeddedVersion)
		fmt.Fprintln(stdout, "Run tier0 skills install")
		return nil
	}

	fmt.Fprintf(stdout, "Skills version:   %s\n", status.Version)
	fmt.Fprintf(stdout, "Source:           %s\n", status.Source)
	fmt.Fprintf(stdout, "Healthy:          %t\n", status.Healthy)
	fmt.Fprintf(stdout, "Embedded version: %s\n", status.EmbeddedVersion)
	if status.UpdatedAt != "" {
		fmt.Fprintf(stdout, "Last updated:     %s\n", status.UpdatedAt)
	}
	fmt.Fprintf(stdout, "Path:             %s\n", status.Path)
	return nil
}

func runSkillsSync(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	stdout := cmd.OutOrStdout()

	result, err := upgrade.EnsureEmbeddedSkills(upgrade.FallbackSkillsDir(), false)
	if err != nil {
		return internalCommandError(cmd, "failed to prepare local Skill: "+err.Error(), err)
	}
	if err := upgrade.SyncAgentSkills(result.Path); err != nil {
		return internalCommandError(cmd, "failed to sync Agent Skills: "+err.Error(), err)
	}
	if jsonMode {
		fmt.Fprintf(stdout, "{\"status\":\"synced\",\"path\":%q}\n", result.Path)
	} else {
		fmt.Fprintln(stdout, "Agent Skills synced to detected global agents")
		fmt.Fprintf(stdout, "Source: %s\n", result.Path)
	}
	return nil
}
