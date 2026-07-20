package cmd

import (
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore topic from history",
	Long:  "Restore a soft-deleted UNS topic. This is a high-risk operation.\n\nExamples:\n  tier0 uns restore --path Plant/Line1/Metric/Temperature --yes",

	RunE: runUnsRestore,
}

func init() {
	unsRestoreCmd.Flags().StringP("path", "p", "",
		"Topic path to restore (required)")
	unsRestoreCmd.Flags().BoolP("yes", "y", false,
		"Confirm high-risk operation (required)")
	unsRestoreCmd.MarkFlagRequired("path")
	addDryRunFlag(unsRestoreCmd)
}

func runUnsRestore(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	confirmed, _ := cmd.Flags().GetBool("yes")
	path, _ := cmd.Flags().GetString("path")

	summary :=
		fmt.Sprintf("Restore topic %q — this will recover the soft-deleted topic.", path)

	payload := map[string]any{
		"path": path,
	}
	if handled, err := writeDryRun(cmd, "POST", "/openapi/v1/uns/restore", payload); handled {
		return err
	}

	if err := highrisk.Guard(confirmed, "uns restore", summary); err != nil {
		return err
	}

	checker := notice.Start()
	body := cmdutil.JSONString(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/restore", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(stdout, "Topic restored: %s\n", path)
	}
	return nil
}
