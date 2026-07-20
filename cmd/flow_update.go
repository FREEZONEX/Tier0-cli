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

var flowUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update flow metadata",
	RunE:  runFlowUpdate,
}

func init() {
	flowUpdateCmd.Flags().Int64("id", 0, "Flow ID (required)")
	flowUpdateCmd.Flags().StringP("name", "n", "",
		"New name")
	flowUpdateCmd.Flags().String("desc", "",
		"New description")
	flowUpdateCmd.Flags().String("template", "",
		"New template JSON string")
	flowUpdateCmd.Flags().String("template-file", "",
		"Read new template from file")
	flowUpdateCmd.Flags().Bool("favorite", false,
		"Mark as favorite")
	flowUpdateCmd.Flags().Bool("unfavorite", false,
		"Remove from favorites")
	addDryRunFlag(flowUpdateCmd)
}

func runFlowUpdate(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	id, _ := cmd.Flags().GetInt64("id")
	flowName, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("desc")
	template, _ := cmd.Flags().GetString("template")
	templateFile, _ := cmd.Flags().GetString("template-file")
	favorite, _ := cmd.Flags().GetBool("favorite")
	unfavorite, _ := cmd.Flags().GetBool("unfavorite")

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
	if cmd.Flags().Changed("template") && cmd.Flags().Changed("template-file") {
		return invalidArgument(cmd, "--template/--template-file", "--template and --template-file are mutually exclusive")
	}
	if favorite && unfavorite {
		return invalidArgument(cmd, "--favorite/--unfavorite", "--favorite and --unfavorite are mutually exclusive")
	}

	if templateFile != "" {
		raw, err := os.ReadFile(templateFile)
		if err != nil {
			return fileIOError(cmd, "--template-file", "read template file", templateFile, err)
		}
		template = string(raw)
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

	payload := map[string]interface{}{"id": id}
	if cmd.Flags().Changed("name") {
		payload["flowName"] = flowName
	}
	if cmd.Flags().Changed("desc") {
		payload["description"] = description
	}
	if cmd.Flags().Changed("template") || cmd.Flags().Changed("template-file") {
		payload["template"] = template
	}
	var isFavorite int64 = -1
	if favorite {
		isFavorite = 1
	}
	if unfavorite {
		isFavorite = 0
	}
	if isFavorite >= 0 {
		payload["isFavorite"] = isFavorite
	}
	if len(payload) == 1 {
		return invalidArgument(cmd, "update fields", "specify at least one flow field to update")
	}
	if handled, err := writeDryRun(cmd, "POST", "/openapi/v1/flow/update", payload); handled {
		return err
	}

	checker := notice.Start()
	body, _ := json.Marshal(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/update", "POST", string(body), debug)
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
	fmt.Fprintf(stdout, "✓ Flow %d updated\n", id)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
