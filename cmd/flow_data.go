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
	checker := notice.Start()
	debug, _ := cmd.Flags().GetBool("debug")
	id, _ := cmd.Flags().GetInt64("id")
	outFile, _ := cmd.Flags().GetString("out")

	if id == 0 && len(args) > 0 {
		id, _ = strconv.ParseInt(args[0], 10, 64)
	}
	if id == 0 {
		return fmt.Errorf(
			"specify a Flow ID via --id <id> or as a positional argument",
		)
	}

	body, _ := json.Marshal(map[string]int64{"id": id})
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/flowdata", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, true) // data always JSON error
	}

	stdout := cmd.OutOrStdout()

	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(resp), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Fprintf(stdout, "✓ Flow data saved to %s\n", outFile)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}

	checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
	return nil
}
