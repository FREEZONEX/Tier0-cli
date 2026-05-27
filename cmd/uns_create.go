package cmd

import (
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
		"Create new UNS namespace nodes.\n\n"+
			"Single-node mode (--topic): path segments before the last one are auto-created as folders.\n"+
			"Use --parent to create under an existing path.\n\n"+
			"Examples:\n"+
			"  tier0 uns create --topic Plant/Line1/Metric/Temperature --type METRIC\n"+
			"  tier0 uns create --parent Plant --topic Line1 --type FOLDER --display-name 'Line 1'\n"+
			"  tier0 uns create --file namespace.json",
		"创建新的 UNS 命名空间节点。\n\n"+
			"单节点模式（--topic）：除最后一段外，路径中的中间段会自动建为 folder。\n"+
			"可用 --parent 在已有路径下创建。\n\n"+
			"示例:\n"+
			"  tier0 uns create --topic Plant/Line1/Metric/Temperature --type METRIC\n"+
			"  tier0 uns create --parent Plant --topic Line1 --type FOLDER --display-name 'Line 1'\n"+
			"  tier0 uns create --file namespace.json",
	),
	RunE: runUnsCreate,
}

func init() {
	unsCreateCmd.Flags().StringP("topic", "t", "",
		i18n.T("Topic path or leaf name (required if not using --file)", "点位路径或叶子名称（不使用 --file 时必填）"))
	unsCreateCmd.Flags().String("parent", "",
		i18n.T("Parent path prefix (optional, combined with --topic)", "父路径前缀（可选，与 --topic 拼接）"))
	unsCreateCmd.Flags().String("type", "",
		i18n.T("Node type (required if not using --file): FOLDER, METRIC/ACTION/STATE, file, folder", "节点类型（不使用 --file 时必填）：FOLDER、METRIC/ACTION/STATE、file、folder"))
	unsCreateCmd.Flags().StringP("display-name", "d", "",
		i18n.T("Display name", "显示名称"))
	unsCreateCmd.Flags().String("description", "",
		i18n.T("Description", "描述"))
	unsCreateCmd.Flags().String("alias", "",
		i18n.T("Alias", "别名"))
	unsCreateCmd.Flags().StringP("file", "f", "",
		i18n.T("Read namespace definition from JSON file ({\"namespace\":[]} or bare array)", "从 JSON 文件读取命名空间定义（支持 {\"namespace\":[]} 或裸数组）"))
	unsCreateCmd.Flags().String("topic-type", "",
		i18n.T("Topic type for file nodes (metric, action, state)", "文件节点的 Topic 类型（metric、action、state）"))
	unsCreateCmd.Flags().String("fields", "",
		i18n.T("Schema fields JSON array (e.g. '[{\"name\":\"temp\",\"type\":\"float\"}]')", "Schema 字段 JSON 数组"))
}

func runUnsCreate(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topic, _ := cmd.Flags().GetString("topic")
	parent, _ := cmd.Flags().GetString("parent")
	nodeType, _ := cmd.Flags().GetString("type")
	displayName, _ := cmd.Flags().GetString("display-name")
	description, _ := cmd.Flags().GetString("description")
	alias, _ := cmd.Flags().GetString("alias")
	file, _ := cmd.Flags().GetString("file")
	topicType, _ := cmd.Flags().GetString("topic-type")
	fields, _ := cmd.Flags().GetString("fields")

	var namespace []any
	createdPath := ""

	if file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to read file: %w", "读取文件失败: %w"), err)
		}
		namespace, err = parseNamespaceFile(raw)
		if err != nil {
			return fmt.Errorf(i18n.T("invalid JSON in file: %w", "文件中 JSON 无效: %w"), err)
		}
	} else {
		if topic == "" || nodeType == "" {
			return fmt.Errorf(i18n.T(
				"--topic and --type are required (or use --file)",
				"--topic 和 --type 为必填（或使用 --file）",
			))
		}
		var err error
		namespace, createdPath, err = buildNamespaceFromFlags(parent, topic, nodeType, topicType, displayName, description, alias, fields)
		if err != nil {
			return err
		}
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
	if !jsonMode && createdPath != "" {
		fmt.Fprintf(stdout, i18n.T("Topic created: %s\n", "点位创建成功: %s\n"), createdPath)
	} else if !jsonMode {
		fmt.Fprintln(stdout, i18n.T("Namespace created.", "命名空间创建完成。"))
	}
	return nil
}
