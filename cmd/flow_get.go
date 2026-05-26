package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowGetCmd = &cobra.Command{
	Use:   "get",
	Short: i18n.T("Get flow details", "获取 Flow 详情"),
	RunE:  runFlowGet,
}

func init() {
	flowGetCmd.Flags().Int64("id", 0, i18n.T("Flow ID", "Flow ID"))
}

func runFlowGet(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	id, _ := cmd.Flags().GetInt64("id")

	// Accept positional arg as ID fallback.
	if id == 0 && len(args) > 0 {
		id, _ = strconv.ParseInt(args[0], 10, 64)
	}
	if id == 0 {
		return fmt.Errorf(i18n.T(
			"specify a Flow ID via --id <id> or as a positional argument",
			"请通过 --id <id> 或直接传入 ID 指定 Flow",
		))
	}

	body, _ := json.Marshal(map[string]int64{"id": id})
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/get", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()

	if jsonMode {
		checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
		return nil
	}

	var f struct {
		Id                 int64  `json:"id"`
		FlowId             string `json:"flowId"`
		FlowName           string `json:"flowName"`
		FlowType           string `json:"flowType"`
		FlowStatus         string `json:"flowStatus"`
		Description        string `json:"description"`
		IsFavorite         int64  `json:"isFavorite"`
		CurrentVersionName string `json:"currentVersionName"`
		CurrentVersionType string `json:"currentVersionType"`
	}
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &f); err != nil {
		fmt.Fprintln(stdout, resp)
		return nil
	}
	fav := i18n.T("no", "否")
	if f.IsFavorite == 1 {
		fav = i18n.T("yes", "是")
	}
	fmt.Fprintf(stdout, "%-16s %d\n", i18n.T("ID:", "ID:"), f.Id)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("FlowId:", "FlowId:"), f.FlowId)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Name:", "名称:"), f.FlowName)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Type:", "类型:"), f.FlowType)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Status:", "状态:"), f.FlowStatus)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Description:", "说明:"), f.Description)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Favorite:", "收藏:"), fav)
	fmt.Fprintf(stdout, "%-16s %s (%s)\n",
		i18n.T("Version:", "当前版本:"), f.CurrentVersionName, f.CurrentVersionType)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
