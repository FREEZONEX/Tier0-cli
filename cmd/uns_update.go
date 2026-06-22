package cmd

import (
	"encoding/json"
	"fmt"

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
	unsUpdateCmd.Flags().StringP("display-name", "d", "",
		"New display name")
	unsUpdateCmd.Flags().String("extend-properties", "",
		"Extended properties JSON object")
	unsUpdateCmd.Flags().String("fields", "",
		"Schema fields JSON array")
	unsUpdateCmd.Flags().StringSlice("update-mask", nil,
		"Fields to update (repeatable, e.g. name,description,fields)")
	unsUpdateCmd.MarkFlagRequired("path")
}

func runUnsUpdate(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	path, _ := cmd.Flags().GetString("path")
	name, _ := cmd.Flags().GetString("name")
	alias, _ := cmd.Flags().GetString("alias")
	description, _ := cmd.Flags().GetString("description")
	displayName, _ := cmd.Flags().GetString("display-name")
	extendProps, _ := cmd.Flags().GetString("extend-properties")
	fields, _ := cmd.Flags().GetString("fields")
	updateMask, _ := cmd.Flags().GetStringSlice("update-mask")

	payload := map[string]any{"path": path}
	if name != "" {
		payload["name"] = name
	}
	if alias != "" {
		payload["alias"] = alias
	}
	if description != "" {
		payload["description"] = description
	}
	if displayName != "" {
		payload["displayName"] = displayName
	}
	if extendProps != "" {
		var props map[string]any
		if err := json.Unmarshal([]byte(extendProps), &props); err != nil {
			return fmt.Errorf("invalid extend-properties JSON: %w", err)
		}
		payload["extendProperties"] = props
	}
	if fields != "" {
		var fieldList []any
		if err := json.Unmarshal([]byte(fields), &fieldList); err != nil {
			return fmt.Errorf("invalid fields JSON: %w", err)
		}
		payload["fields"] = fieldList
	}
	if len(updateMask) > 0 {
		payload["updateMask"] = updateMask
	}

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
