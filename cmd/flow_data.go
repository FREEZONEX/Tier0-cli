package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowDataCmd = &cobra.Command{
	Use:   "data",
	Short: "Get Node-RED canvas JSON",
	RunE:  runFlowData,
}

func init() {
	flowDataCmd.Flags().Int64("id", 0, "Flow ID")
	flowDataCmd.Flags().StringP("out", "o", "",
		"Save output to file")
}

func runFlowData(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	id, _ := cmd.Flags().GetInt64("id")
	outFile, _ := cmd.Flags().GetString("out")

	if id == 0 && len(args) > 0 {
		parsedID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return invalidArgumentCause(cmd, "flow ID", "flow ID must be an integer: "+err.Error(), err)
		}
		id = parsedID
	}
	if id <= 0 {
		return invalidArgument(cmd, "--id", "specify a positive Flow ID via --id <id> or as a positional argument")
	}

	body, _ := json.Marshal(map[string]int64{"id": id})
	checker := notice.Start()
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/flowdata", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckResponse(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()

	if outFile != "" {
		flowsJSON, err := normalizeNodeREDFlowsJSON(resp, true)
		if err != nil {
			return internalCommandError(cmd, "failed to extract Node-RED flows from response: "+err.Error(), err)
		}
		if err := os.WriteFile(outFile, []byte(flowsJSON), 0644); err != nil {
			return fileIOError(cmd, "--out", "write flow data file", outFile, err)
		}
		fmt.Fprintf(stdout, "✓ Flow data saved to %s\n", outFile)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}

	checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
	return nil
}
