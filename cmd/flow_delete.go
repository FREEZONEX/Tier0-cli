package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowDeleteCmd = &cobra.Command{
	Use:     "delete",
	Aliases: []string{"del", "rm"},
	Short:   "Delete flow(s)",
	RunE:    runFlowDelete,
}

func init() {
	flowDeleteCmd.Flags().Int64Slice("id", nil,
		"Flow ID(s) to delete (repeatable)")
	flowDeleteCmd.Flags().BoolP("yes", "y", false,
		"Confirm high-risk operation (required)")
	addDryRunFlag(flowDeleteCmd)
}

func runFlowDelete(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	confirmed, _ := cmd.Flags().GetBool("yes")
	ids, _ := cmd.Flags().GetInt64Slice("id")
	for _, id := range ids {
		if id <= 0 {
			return invalidArgument(cmd, "--id", "Flow IDs must be positive integers")
		}
	}

	// Also parse positional args as comma-separated IDs.
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			n, err := strconv.ParseInt(part, 10, 64)
			if err != nil || n <= 0 {
				if err == nil {
					err = fmt.Errorf("ID must be positive")
				}
				return invalidArgumentCause(cmd, "flow ID", fmt.Sprintf("invalid Flow ID %q: %v", part, err), err)
			}
			ids = append(ids, n)
		}
	}

	if len(ids) == 0 {
		return invalidArgument(cmd, "--id", "specify at least one Flow ID via --id <id> or as positional arguments (comma-separated)")
	}

	summary :=
		fmt.Sprintf("Delete Flow(s) %v — this will STOP the Node-RED container(s) and cannot be undone.", ids)

	payload := map[string][]int64{"ids": ids}
	if handled, err := writeDryRun(cmd, "POST", "/openapi/v1/flow/delete", payload); handled {
		return err
	}

	if err := highrisk.Guard(confirmed, "flow delete", summary); err != nil {
		return err
	}

	checker := notice.Start()
	body, _ := json.Marshal(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/delete", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	if jsonMode {
		checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
		return nil
	}
	_ = resp
	if len(ids) == 1 {
		fmt.Fprintf(stdout, "✓ Flow %d deleted\n", ids[0])
	} else {
		fmt.Fprintf(stdout, "✓ %d flows deleted\n", len(ids))
	}
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
