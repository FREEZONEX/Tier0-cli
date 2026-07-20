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
	addDryRunFlag(flowCreateCmd)
}

func runFlowCreate(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	flowName, _ := cmd.Flags().GetString("name")
	flowType, _ := cmd.Flags().GetString("type")
	description, _ := cmd.Flags().GetString("desc")
	template, _ := cmd.Flags().GetString("template")
	templateFile, _ := cmd.Flags().GetString("template-file")
	source, _ := cmd.Flags().GetBool("source")
	event, _ := cmd.Flags().GetBool("event")

	if source && event {
		return invalidArgument(cmd, "--source/--event", "--source and --event are mutually exclusive")
	}
	if cmd.Flags().Changed("type") && (source || event) {
		return invalidArgument(cmd, "--type", "--type cannot be combined with --source or --event")
	}
	if source {
		flowType = flowTypeSource
	}
	if event {
		flowType = flowTypeEvent
	}
	if cmd.Flags().Changed("template") && cmd.Flags().Changed("template-file") {
		return invalidArgument(cmd, "--template/--template-file", "--template and --template-file are mutually exclusive")
	}

	if templateFile != "" {
		raw, err := os.ReadFile(templateFile)
		if err != nil {
			return fileIOError(cmd, "--template-file", "read template file", templateFile, err)
		}
		template = string(raw)
	}

	if flowName == "" {
		return invalidArgument(cmd, "--name", "flow name is required (--name)")
	}
	if flowType == "" {
		return invalidArgument(cmd, "--type", "flow type is required: use --type SourceFlow|EventFlow, or --source / --event")
	}
	if flowType != flowTypeSource && flowType != flowTypeEvent {
		return invalidArgument(cmd, "--type", "--type must be SourceFlow or EventFlow")
	}
	if template != "" {
		if _, err := decodeJSONInput(template); err != nil {
			param := "--template"
			if templateFile != "" {
				param = "--template-file"
			}
			return invalidArgumentCause(cmd, param, "flow template must be valid JSON: "+err.Error(), err)
		}
	}

	payload := map[string]string{
		"flowName":    flowName,
		"flowType":    flowType,
		"description": description,
		"template":    template,
	}
	if handled, err := writeDryRun(cmd, "POST", "/openapi/v1/flow/create", payload); handled {
		return err
	}

	checker := notice.Start()
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
