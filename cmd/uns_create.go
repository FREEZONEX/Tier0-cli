package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: i18n.T("Create UNS namespace nodes", "创建 UNS 命名空间节点"),
	Long: i18n.T(
		"Create new UNS namespace nodes.\n\nExamples:\n  tier0 uns create --topic Plant/Line1/Metric/Temperature --type METRIC\n  tier0 uns create --topic Plant/Line1 --type FOLDER --display-name 'Line 1'\n  tier0 uns create --file namespace.json",
		"创建新的 UNS 命名空间节点。\n\n示例:\n  tier0 uns create --topic Plant/Line1/Metric/Temperature --type METRIC\n  tier0 uns create --topic Plant/Line1 --type FOLDER --display-name 'Line 1'\n  tier0 uns create --file namespace.json",
	),
	RunE: runUnsCreate,
}

func init() {
	unsCreateCmd.Flags().StringP("topic", "t", "",
		i18n.T("New topic path/name (required if not using --file)", "新点位路径/名称（不使用 --file 时必填）"))
	unsCreateCmd.Flags().String("type", "",
		i18n.T("Node type (required if not using --file, e.g. METRIC, FOLDER, THING)", "节点类型（不使用 --file 时必填，如 METRIC, FOLDER, THING）"))
	unsCreateCmd.Flags().StringP("display-name", "d", "",
		i18n.T("Display name", "显示名称"))
	unsCreateCmd.Flags().String("description", "",
		i18n.T("Description", "描述"))
	unsCreateCmd.Flags().String("alias", "",
		i18n.T("Alias", "别名"))
	unsCreateCmd.Flags().StringP("file", "f", "",
		i18n.T("Read namespace definition from JSON file", "从 JSON 文件读取命名空间定义"))
	unsCreateCmd.Flags().String("topic-type", "",
		i18n.T("Topic type", "Topic 类型"))
	unsCreateCmd.Flags().String("fields", "",
		i18n.T("Schema fields JSON array (e.g. '[{\"name\":\"temp\",\"type\":\"float\"}]')", "Schema 字段 JSON 数组"))
}

func runUnsCreate(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topic, _ := cmd.Flags().GetString("topic")
	nodeType, _ := cmd.Flags().GetString("type")
	displayName, _ := cmd.Flags().GetString("display-name")
	description, _ := cmd.Flags().GetString("description")
	alias, _ := cmd.Flags().GetString("alias")
	file, _ := cmd.Flags().GetString("file")
	topicType, _ := cmd.Flags().GetString("topic-type")
	fields, _ := cmd.Flags().GetString("fields")

	var namespace []any

	if file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to read file: %w", "读取文件失败: %w"), err)
		}
		if err := json.Unmarshal(raw, &namespace); err != nil {
			return fmt.Errorf(i18n.T("invalid JSON in file: %w", "文件中 JSON 无效: %w"), err)
		}
	} else {
		if topic == "" || nodeType == "" {
			return fmt.Errorf(i18n.T(
				"--topic and --type are required (or use --file)",
				"--topic 和 --type 为必填（或使用 --file）",
			))
		}
		node := map[string]any{
			"name": topic,
			"type": nodeType,
		}
		if displayName != "" {
			node["displayName"] = displayName
		}
		if description != "" {
			node["description"] = description
		}
		if alias != "" {
			node["alias"] = alias
		}
		if topicType != "" {
			node["topicType"] = topicType
		}
		if fields != "" {
			var fieldList []any
			if err := json.Unmarshal([]byte(fields), &fieldList); err != nil {
				return fmt.Errorf(i18n.T("invalid fields JSON: %w", "fields JSON 无效: %w"), err)
			}
			node["fields"] = fieldList
		}
		namespace = []any{node}
	}

	body := cmdutil.JSONString(map[string]any{"namespace": namespace})
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/create", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(stdout, i18n.T("Topic created: %s\n", "点位创建成功: %s\n"), topic)
	}
	return nil
}
