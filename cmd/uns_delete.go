package cmd

import (
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete UNS node(s)",
	Long:  "Delete one or more UNS nodes (path or topic). By default performs a soft delete (recoverable via restore). Use --hard to permanently delete.\n\nExamples:\n  tier0 uns delete --path factory/line1/sensor/temp --yes\n  tier0 uns delete --path factory/line1/sensor/temp --path factory/line1/sensor/humi --yes\n  tier0 uns delete --path factory/line1/sensor/temp --hard --yes",

	RunE: runUnsDelete,
}

func init() {
	unsDeleteCmd.Flags().StringArrayP("path", "p", nil,
		"Node path(s) to delete (repeatable, required)")
	unsDeleteCmd.Flags().StringArray("topic", nil,
		"Deprecated alias for --path")
	_ = unsDeleteCmd.Flags().MarkHidden("topic")
	unsDeleteCmd.Flags().Bool("hard", false,
		"Hard delete (irreversible)")
	unsDeleteCmd.Flags().BoolP("yes", "y", false,
		"Confirm high-risk operation (required)")
	addDryRunFlag(unsDeleteCmd)
}

func runUnsDelete(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	confirmed, _ := cmd.Flags().GetBool("yes")
	topics, _ := cmd.Flags().GetStringArray("path")
	legacyTopics, _ := cmd.Flags().GetStringArray("topic")
	hard, _ := cmd.Flags().GetBool("hard")
	if len(legacyTopics) > 0 {
		if !jsonMode {
			fmt.Fprintln(cmd.ErrOrStderr(),
				"warning: --topic is deprecated for uns delete; use --path",
			)
		}
		topics = append(topics, legacyTopics...)
	}
	if len(topics) == 0 {
		return invalidArgument(cmd, "--path", "specify at least one path via --path <path>")
	}

	action := "soft delete"
	if hard {
		action = "HARD DELETE"
	}
	summary :=
		fmt.Sprintf("%s UNS topic(s) %v — %s.", action, topics, func() string {
			if hard {
				return "this is IRREVERSIBLE"
			}
			return "soft deleted items can be restored"
		}())

	payload := map[string]any{"topics": topics}
	if hard {
		payload["hard_delete"] = true
	}
	if handled, err := writeDryRun(cmd, "POST", "/openapi/v1/uns/delete", payload); handled {
		return err
	}
	if err := highrisk.Guard(confirmed, "uns delete", summary); err != nil {
		return err
	}

	checker := notice.Start()
	body := cmdutil.JSONString(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/delete", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(stdout, "Topic(s) deleted: %v\n", topics)
	}
	return nil
}
