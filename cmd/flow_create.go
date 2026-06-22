package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new flow",
	RunE:  runFlowCreate,
}

func init() {
	flowCreateCmd.Flags().StringP("name", "n", "",
		"Flow name (required)")
	flowCreateCmd.Flags().StringP("type", "t", "",
		"Flow type: SourceFlow | EventFlow (required)")
	flowCreateCmd.Flags().Bool("source", false,
		"Set type to SourceFlow")
	flowCreateCmd.Flags().Bool("event", false,
		"Set type to EventFlow")
	flowCreateCmd.Flags().String("desc", "",
		"Description")
	flowCreateCmd.Flags().String("template", "",
		"Initial template JSON string")
	flowCreateCmd.Flags().String("template-file", "",
		"Read initial template from file")
}

func runFlowCreate(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	flowName, _ := cmd.Flags().GetString("name")
	flowType, _ := cmd.Flags().GetString("type")
	description, _ := cmd.Flags().GetString("desc")
	template, _ := cmd.Flags().GetString("template")
	templateFile, _ := cmd.Flags().GetString("template-file")

	if source, _ := cmd.Flags().GetBool("source"); source {
		flowType = flowTypeSource
	}
	if event, _ := cmd.Flags().GetBool("event"); event {
		flowType = flowTypeEvent
	}

	if templateFile != "" {
		raw, err := os.ReadFile(templateFile)
		if err != nil {
			return fmt.Errorf("failed to read template file: %w", err)
		}
		template = string(raw)
	}

	if flowName == "" {
		return fmt.Errorf(
			"flow name is required (--name)",
		)
	}
	if flowType == "" {
		return fmt.Errorf(
			"flow type is required: use --type SourceFlow|EventFlow, or --source / --event",
		)
	}

	payload := map[string]string{
		"flowName":    flowName,
		"flowType":    flowType,
		"description": description,
		"template":    template,
	}
	body, _ := json.Marshal(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/create", "POST", string(body), debug)
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
		Id int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}
	fmt.Fprintf(stdout, "✓ Flow created, ID: %d\n", result.Id)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
