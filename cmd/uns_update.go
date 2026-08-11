package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update UNS topic metadata",
	Long:  "Update metadata of an existing UNS topic.\n\nExamples:\n  tier0 uns update --path Plant/Line1/Metric/Temperature --display-name 'Line 1 Temp'\n  tier0 uns update --path Plant/Line1 --description 'Production line 1' --update-mask description\n  tier0 uns update --path Plant/Line1/Metric/Temperature --fields '[{\"name\":\"temp\",\"type\":\"float\",\"unit\":\"C\"}]' --update-mask fields",

	RunE: runUnsUpdate,
}

func init() {
	unsUpdateCmd.Flags().StringP("path", "p", "",
		"Topic path to update (required)")
	unsUpdateCmd.Flags().StringP("name", "n", "",
		"New name")
	unsUpdateCmd.Flags().String("alias", "",
		"New alias")
	unsUpdateCmd.Flags().String("description", "",
		"New description")
	unsUpdateCmd.Flags().Bool("clear-description", false,
		"Clear the description without relying on an empty shell argument")
	unsUpdateCmd.Flags().StringP("display-name", "d", "",
		"New display name")
	unsUpdateCmd.Flags().String("extend-properties", "",
		"Extended properties JSON object")
	unsUpdateCmd.Flags().String("fields", "",
		"Schema fields JSON array")
	unsUpdateCmd.Flags().String("fields-file", "",
		"Read schema fields JSON array from file")
	unsUpdateCmd.Flags().StringSlice("update-mask", nil,
		"Fields to update (repeatable, e.g. name,description,fields)")
	unsUpdateCmd.MarkFlagRequired("path")
	addDryRunFlag(unsUpdateCmd)
}

func runUnsUpdate(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	path, _ := cmd.Flags().GetString("path")
	name, _ := cmd.Flags().GetString("name")
	alias, _ := cmd.Flags().GetString("alias")
	description, _ := cmd.Flags().GetString("description")
	clearDescription, _ := cmd.Flags().GetBool("clear-description")
	displayName, _ := cmd.Flags().GetString("display-name")
	extendProps, _ := cmd.Flags().GetString("extend-properties")
	fields, _ := cmd.Flags().GetString("fields")
	fieldsFile, _ := cmd.Flags().GetString("fields-file")
	updateMask, _ := cmd.Flags().GetStringSlice("update-mask")

	payload := map[string]any{"path": path}
	hasUpdate := false
	if cmd.Flags().Changed("name") {
		payload["name"] = name
		hasUpdate = true
	}
	if cmd.Flags().Changed("alias") {
		payload["alias"] = alias
		hasUpdate = true
	}
	if cmd.Flags().Changed("description") && clearDescription {
		return invalidArgument(cmd, "--description/--clear-description", "--description and --clear-description are mutually exclusive")
	}
	if cmd.Flags().Changed("fields") && cmd.Flags().Changed("fields-file") {
		return invalidArgument(cmd, "--fields/--fields-file", "--fields and --fields-file are mutually exclusive")
	}
	if cmd.Flags().Changed("description") || clearDescription {
		payload["description"] = description
		hasUpdate = true
	}
	if cmd.Flags().Changed("display-name") {
		payload["displayName"] = displayName
		hasUpdate = true
	}
	if cmd.Flags().Changed("extend-properties") {
		var props map[string]any
		if err := json.Unmarshal([]byte(extendProps), &props); err != nil {
			return invalidArgumentCause(cmd, "--extend-properties", "--extend-properties must be a JSON object: "+err.Error(), err)
		}
		payload["extendProperties"] = props
		hasUpdate = true
	}
	if cmd.Flags().Changed("fields-file") {
		raw, err := os.ReadFile(fieldsFile)
		if err != nil {
			return fileIOError(cmd, "--fields-file", "read fields file", fieldsFile, err)
		}
		fields = string(raw)
	}
	if cmd.Flags().Changed("fields") || cmd.Flags().Changed("fields-file") {
		var fieldList []any
		if err := json.Unmarshal([]byte(fields), &fieldList); err != nil {
			param := "--fields"
			if cmd.Flags().Changed("fields-file") {
				param = "--fields-file"
			}
			return invalidArgumentCause(cmd, param, param+" must contain a JSON array: "+err.Error(), err)
		}
		payload["fields"] = fieldList
		hasUpdate = true
	}
	if len(updateMask) > 0 {
		payload["updateMask"] = updateMask
	}
	if !hasUpdate {
		return invalidArgument(cmd, "update fields", "specify at least one field to update")
	}
	if handled, err := writeDryRun(cmd, "POST", "/openapi/v1/uns/update", payload); handled {
		return err
	}

	checker := notice.Start()
	body := cmdutil.JSONString(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/update", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(stdout, "Topic updated: %s\n", path)
	}
	return nil
}
