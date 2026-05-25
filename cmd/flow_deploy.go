package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: i18n.T("Deploy Node-RED canvas JSON", "部署 Node-RED 画布 JSON"),
	RunE:  runFlowDeploy,
}

func init() {
	flowDeployCmd.Flags().Int64("id", 0, i18n.T("Flow ID (required)", "Flow ID（必填）"))
	flowDeployCmd.Flags().String("flows-json", "",
		i18n.T("Node-RED canvas JSON string", "Node-RED 画布 JSON 字符串"))
	flowDeployCmd.Flags().StringP("flows-file", "f", "",
		i18n.T("Read Node-RED canvas JSON from file (recommended)", "从文件读取 Node-RED 画布 JSON（推荐）"))
	flowDeployCmd.Flags().BoolP("yes", "y", false,
		i18n.T("Confirm high-risk operation (required)", "确认高风险操作（必填）"))
}

func runFlowDeploy(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	confirmed, _ := cmd.Flags().GetBool("yes")
	id, _ := cmd.Flags().GetInt64("id")
	flowsJSON, _ := cmd.Flags().GetString("flows-json")
	flowsFile, _ := cmd.Flags().GetString("flows-file")

	if id == 0 && len(args) > 0 {
		id, _ = strconv.ParseInt(args[0], 10, 64)
	}
	if id == 0 {
		return fmt.Errorf(i18n.T(
			"specify a Flow ID via --id <id> or as a positional argument",
			"请通过 --id <id> 或直接传入 ID 指定 Flow",
		))
	}

	if flowsFile != "" {
		raw, err := os.ReadFile(flowsFile)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to read flows file: %w", "读取 flows 文件失败: %w"), err)
		}
		flowsJSON = string(raw)
	}
	if flowsJSON == "" {
		return fmt.Errorf(i18n.T(
			"provide Node-RED canvas JSON via --flows-json '<json>' or --flows-file <file>",
			"请通过 --flows-json '<json>' 或 --flows-file <file> 提供 Node-RED 画布数据",
		))
	}

	summary := i18n.T(
		fmt.Sprintf("Deploy canvas to Flow %d — ALL existing Node-RED nodes will be REPLACED. Back up with 'tier0 flow data --id %d --out backup.json' first.", id, id),
		fmt.Sprintf("部署画布到 Flow %d — 将替换该 Node-RED 实例的所有节点配置。建议先执行 'tier0 flow data --id %d --out backup.json' 备份。", id, id),
	)
	if err := highrisk.Guard(confirmed, "flow deploy", summary); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"id":        id,
		"flowsJson": flowsJSON,
	}
	body, _ := json.Marshal(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/deploy", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	if jsonMode {
		checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
		return nil
	}

	var result struct {
		FlowId string `json:"flowId"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}
	fmt.Fprintf(stdout, i18n.T(
		"✓ Flow %d deployed, Node-RED FlowId: %s\n",
		"✓ Flow %d 部署成功，Node-RED FlowId: %s\n",
	), id, result.FlowId)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
