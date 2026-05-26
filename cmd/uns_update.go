package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: i18n.T("Update UNS topic metadata", "更新 UNS 点位元数据"),
	Long: i18n.T(
		"Update metadata of an existing UNS topic.\n\nExamples:\n  tier0 uns update --path Plant/Line1/Metric/Temperature --display-name 'Line 1 Temp'\n  tier0 uns update --path Plant/Line1 --description 'Production line 1' --update-mask description\n  tier0 uns update --path Plant/Line1/Metric/Temperature --fields '[{\"name\":\"temp\",\"type\":\"float\",\"unit\":\"C\"}]' --update-mask fields",
		"更新现有 UNS 点位的元数据。\n\n示例:\n  tier0 uns update --path Plant/Line1/Metric/Temperature --display-name 'Line 1 Temp'\n  tier0 uns update --path Plant/Line1 --description 'Production line 1' --update-mask description\n  tier0 uns update --path Plant/Line1/Metric/Temperature --fields '[{\"name\":\"temp\",\"type\":\"float\",\"unit\":\"C\"}]' --update-mask fields",
	),
	RunE: runUnsUpdate,
}

func init() {
	unsUpdateCmd.Flags().StringP("path", "p", "",
		i18n.T("Topic path to update (required)", "要更新的点位路径（必填）"))
	unsUpdateCmd.Flags().StringP("name", "n", "",
		i18n.T("New name", "新名称"))
	unsUpdateCmd.Flags().String("alias", "",
		i18n.T("New alias", "新别名"))
	unsUpdateCmd.Flags().String("description", "",
		i18n.T("New description", "新描述"))
	unsUpdateCmd.Flags().StringP("display-name", "d", "",
		i18n.T("New display name", "新显示名称"))
	unsUpdateCmd.Flags().String("extend-properties", "",
		i18n.T("Extended properties JSON object", "扩展属性 JSON 对象"))
	unsUpdateCmd.Flags().String("fields", "",
		i18n.T("Schema fields JSON array", "Schema 字段 JSON 数组"))
	unsUpdateCmd.Flags().StringSlice("update-mask", nil,
		i18n.T("Fields to update (repeatable, e.g. name,description,fields)", "要更新的字段（可重复指定，如 name,description,fields）"))
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
			return fmt.Errorf(i18n.T("invalid extend-properties JSON: %w", "extend-properties JSON 无效: %w"), err)
		}
		payload["extendProperties"] = props
	}
	if fields != "" {
		var fieldList []any
		if err := json.Unmarshal([]byte(fields), &fieldList); err != nil {
			return fmt.Errorf(i18n.T("invalid fields JSON: %w", "fields JSON 无效: %w"), err)
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
		fmt.Fprintf(stdout, i18n.T("Topic updated: %s\n", "点位更新成功: %s\n"), path)
	}
	return nil
}
