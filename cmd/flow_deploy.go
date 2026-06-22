package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy Node-RED canvas JSON",
	RunE:  runFlowDeploy,
}

func init() {
	flowDeployCmd.Flags().Int64("id", 0, "Flow ID (required)")
	flowDeployCmd.Flags().String("flows-json", "",
		"Node-RED canvas JSON string")
	flowDeployCmd.Flags().StringP("flows-file", "f", "",
		"Read Node-RED canvas JSON from file (recommended)")
	flowDeployCmd.Flags().BoolP("yes", "y", false,
		"Confirm high-risk operation (required)")
}

func runFlowDeploy(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	confirmed, _ := cmd.Flags().GetBool("yes")
	id, _ := cmd.Flags().GetInt64("id")
	flowsJSON, _ := cmd.Flags().GetString("flows-json")
	flowsFile, _ := cmd.Flags().GetString("flows-file")

	if id == 0 && len(args) > 0 {
		id, _ = strconv.ParseInt(args[0], 10, 64)
	}
	if id == 0 {
		return fmt.Errorf(
			"specify a Flow ID via --id <id> or as a positional argument",
		)
	}

	if flowsFile != "" {
		raw, err := os.ReadFile(flowsFile)
		if err != nil {
			return fmt.Errorf("failed to read flows file: %w", err)
		}
		flowsJSON = string(raw)
	}
	if flowsJSON == "" {
		return fmt.Errorf(
			"provide Node-RED canvas JSON via --flows-json '<json>' or --flows-file <file>",
		)
	}

	summary :=
		fmt.Sprintf("Deploy canvas to Flow %d — ALL existing Node-RED nodes will be REPLACED. Back up with 'tier0 flow data --id %d --out backup.json' first.", id, id)

	if err := highrisk.Guard(confirmed, "flow deploy", summary); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"id":        id,
		"flowsJson": flowsJSON,
	}
	body, _ := json.Marshal(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/deploy", "POST", string(body), debug)
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

	var result struct {
		FlowId string `json:"flowId"`
	}
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}
	fmt.Fprintf(stdout,
		"✓ Flow %d deployed, Node-RED FlowId: %s\n",
		id, result.FlowId)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
