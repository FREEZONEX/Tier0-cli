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

var flowCreateCmd = &cobra.Command{
	Use:   "create",
	Short: i18n.T("Create a new flow", "创建新 Flow"),
	RunE:  runFlowCreate,
}

func init() {
	flowCreateCmd.Flags().StringP("name", "n", "",
		i18n.T("Flow name (required)", "Flow 名称（必填）"))
	flowCreateCmd.Flags().StringP("type", "t", "",
		i18n.T("Flow type: SourceFlow | EventFlow (required)", "Flow 类型（必填）"))
	flowCreateCmd.Flags().Bool("source", false,
		i18n.T("Set type to SourceFlow", "类型设为 SourceFlow"))
	flowCreateCmd.Flags().Bool("event", false,
		i18n.T("Set type to EventFlow", "类型设为 EventFlow"))
	flowCreateCmd.Flags().String("desc", "",
		i18n.T("Description", "描述"))
	flowCreateCmd.Flags().String("template", "",
		i18n.T("Initial template JSON string", "初始模板 JSON 字符串"))
	flowCreateCmd.Flags().String("template-file", "",
		i18n.T("Read initial template from file", "从文件读取初始模板 JSON"))
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
			return fmt.Errorf(i18n.T("failed to read template file: %w", "读取模板文件失败: %w"), err)
		}
		template = string(raw)
	}

	if flowName == "" {
		return fmt.Errorf(i18n.T(
			"flow name is required (--name)",
			"请通过 --name 指定 Flow 名称",
		))
	}
	if flowType == "" {
		return fmt.Errorf(i18n.T(
			"flow type is required: use --type SourceFlow|EventFlow, or --source / --event",
			"请通过 --type SourceFlow|EventFlow（或 --source / --event）指定 Flow 类型",
		))
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
	fmt.Fprintf(stdout, i18n.T("✓ Flow created, ID: %d\n", "✓ Flow 创建成功，ID: %d\n"), result.Id)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
