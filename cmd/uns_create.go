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
		"Create UNS namespace nodes from a path or a JSON file.\n\n"+
			"PATH RULE for topic (file) nodes:\n"+
			"  The segment immediately before the leaf must be a type folder: Metric, Action, or State.\n"+
			"  The topicType is derived from that segment automatically — nothing is inserted.\n\n"+
			"  Valid:   Plant/Line1/Metric/Temperature\n"+
			"  Valid:   Machine/Action/Start\n"+
			"  Invalid: Plant/Line1/Temperature  (no type folder before leaf)\n\n"+
			"Use --parent to prepend a common path prefix to --topic.\n"+
			"Use --file for batch or complex structures.\n\n"+
			"Examples:\n"+
			"  tier0 uns create --topic Plant/Line1/Metric/Temperature --type topic\n"+
			"  tier0 uns create --parent Factory1/Line1/Station1 --topic Metric/ProductionCount --type topic\n"+
			"  tier0 uns create --topic Plant/Line1 --type path --display-name 'Line 1'\n"+
			"  tier0 uns create --file namespace.json",
		"创建 UNS 命名空间节点（路径模式或 JSON 文件模式）。\n\n"+
			"topic 节点的路径规则：\n"+
			"  叶子名前一段必须是类型目录：Metric、Action 或 State。\n"+
			"  topicType 从该段自动推导——不会自动插入任何目录。\n\n"+
			"  正确：Plant/Line1/Metric/Temperature\n"+
			"  正确：Machine/Action/Start\n"+
			"  错误：Plant/Line1/Temperature（叶子名前缺少类型目录）\n\n"+
			"用 --parent 为 --topic 拼接公共前缀路径。\n"+
			"批量或复杂结构请用 --file。\n\n"+
			"示例：\n"+
			"  tier0 uns create --topic Plant/Line1/Metric/Temperature --type topic\n"+
			"  tier0 uns create --parent Factory1/Line1/Station1 --topic Metric/ProductionCount --type topic\n"+
			"  tier0 uns create --topic Plant/Line1 --type path --display-name 'Line 1'\n"+
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
		i18n.T("Node type: 'path' (folder) or 'topic' (data point)", "节点类型：path（文件夹）或 topic（数据点）"))
	unsCreateCmd.Flags().StringP("display-name", "d", "",
		i18n.T("Display name", "显示名称"))
	unsCreateCmd.Flags().String("description", "",
		i18n.T("Description", "描述"))
	unsCreateCmd.Flags().String("alias", "",
		i18n.T("Alias", "别名"))
	unsCreateCmd.Flags().StringP("file", "f", "",
		i18n.T("Read namespace definition from JSON file ({\"namespace\":[]} or bare array)", "从 JSON 文件读取命名空间定义（支持 {\"namespace\":[]} 或裸数组）"))
	unsCreateCmd.Flags().String("topic-type", "",
		i18n.T("Deprecated: topic type is now derived from the path (Metric/Action/State folder before leaf)", "已废弃：topic 类型现在从路径中自动推导（叶子名前的 Metric/Action/State 目录）"))
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

	errOut := cmd.ErrOrStderr()
	if file != "" {
		if topic != "" || nodeType != "" || parent != "" {
			return fmt.Errorf(i18n.T(
				"--topic, --type, and --parent cannot be used together with --file",
				"--topic、--type、--parent 不能与 --file 同时使用",
			))
		}
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
		namespace, createdPath, err = buildNamespaceFromFlags(parent, topic, nodeType, topicType, displayName, description, alias, fields, errOut)
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
