package cmd

import (
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsReadCmd = &cobra.Command{
	Use:   "read [topic...]",
	Short: "Read current value of UNS topics",
	Long:  "Read the current value of one or more UNS topics.\n\nExamples:\n  tier0 uns read demo\n  tier0 uns read --topic demo\n  tier0 uns read temp humidity\n  tier0 uns read --topic sensor1 --include-metadata",

	RunE: runUnsRead,
}

func init() {
	unsReadCmd.Flags().StringSliceP("topic", "t", nil,
		"Topic name(s) to read (repeatable; positional args are also accepted)")
	unsReadCmd.Flags().Bool("include-metadata", false,
		"Include topic metadata (topicType, fields, description)")
	unsReadCmd.Flags().Bool("include-leaf-value", false,
		"Include leaf node values")
}

func runUnsRead(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topics, _ := cmd.Flags().GetStringSlice("topic")
	includeMeta, _ := cmd.Flags().GetBool("include-metadata")
	includeLeaf, _ := cmd.Flags().GetBool("include-leaf-value")
	topics = append(topics, args...)
	if len(topics) == 0 {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("%s",
			"specify at least one topic via --topic <path> or positional arguments",
		), jsonMode)
	}

	payload := map[string]any{"topics": topics}
	if includeMeta {
		payload["include_metadata"] = true
	}
	if includeLeaf {
		payload["include_leaf_value"] = true
	}

	body := cmdutil.JSONString(payload)

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/read", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		stdout.Write([]byte(resp + "\n"))
	}
	return nil
}
