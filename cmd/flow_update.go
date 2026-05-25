package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: i18n.T("Update flow metadata", "更新 Flow 元数据"),
	RunE:  runFlowUpdate,
}

func init() {
	flowUpdateCmd.Flags().Int64("id", 0, i18n.T("Flow ID (required)", "Flow ID（必填）"))
	flowUpdateCmd.Flags().StringP("name", "n", "",
		i18n.T("New name", "新名称"))
	flowUpdateCmd.Flags().String("desc", "",
		i18n.T("New description", "新描述"))
	flowUpdateCmd.Flags().String("template", "",
		i18n.T("New template JSON string", "更新模板 JSON 字符串"))
	flowUpdateCmd.Flags().String("template-file", "",
		i18n.T("Read new template from file", "从文件读取模板 JSON"))
	flowUpdateCmd.Flags().Bool("favorite", false,
		i18n.T("Mark as favorite", "标记为收藏"))
	flowUpdateCmd.Flags().Bool("unfavorite", false,
		i18n.T("Remove from favorites", "取消收藏"))
}

func runFlowUpdate(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
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
		id, _ = strconv.ParseInt(args[0], 10, 64)
	}
	if id == 0 {
		return fmt.Errorf(i18n.T(
			"specify a Flow ID via --id <id> or as a positional argument",
			"请通过 --id <id> 或直接传入 ID 指定 Flow",
		))
	}

	if templateFile != "" {
		raw, err := os.ReadFile(templateFile)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to read template file: %w", "读取模板文件失败: %w"), err)
		}
		template = string(raw)
	}

	payload := map[string]interface{}{"id": id}
	if flowName != "" {
		payload["flowName"] = flowName
	}
	if description != "" {
		payload["description"] = description
	}
	if template != "" {
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

	body, _ := json.Marshal(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/update", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	if jsonMode {
		checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
		return nil
	}
	_ = resp
	fmt.Fprintf(stdout, i18n.T("✓ Flow %d updated\n", "✓ Flow %d 更新成功\n"), id)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
