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

var unsBindFlowCmd = &cobra.Command{
	Use:   "bind-flow",
	Short: i18n.T("Bind a UNS node to a SourceFlow", "绑定 UNS 节点到 SourceFlow"),
	Long: i18n.T(
		"Bind a UNS node to a SourceFlow using unsId and flow business ID.",
		"使用 unsId 和 Flow 业务主键 ID 关联 UNS 节点与 SourceFlow。",
	),
	RunE: runUnsBindFlow,
}

func init() {
	unsBindFlowCmd.Flags().Int64("uns-id", 0,
		i18n.T("UNS node ID (required)", "UNS 节点 ID（必填）"))
	unsBindFlowCmd.Flags().Int64("flow-id", 0,
		i18n.T("Flow business ID (required)", "Flow 业务主键 ID（必填）"))
	unsBindFlowCmd.MarkFlagRequired("uns-id")
	unsBindFlowCmd.MarkFlagRequired("flow-id")
}

func runUnsBindFlow(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	unsID, _ := cmd.Flags().GetInt64("uns-id")
	flowID, _ := cmd.Flags().GetInt64("flow-id")

	if unsID == 0 && len(args) > 0 {
		unsID, _ = strconv.ParseInt(args[0], 10, 64)
	}
	if flowID == 0 && len(args) > 1 {
		flowID, _ = strconv.ParseInt(args[1], 10, 64)
	}
	if unsID <= 0 || flowID <= 0 {
		return fmt.Errorf(i18n.T(
			"specify --uns-id <id> and --flow-id <id>",
			"请指定 --uns-id <id> 和 --flow-id <id>",
		))
	}

	body, _ := json.Marshal(map[string]int64{
		"unsId":  unsID,
		"flowId": flowID,
	})
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/unsBindFlow", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	if jsonMode {
		checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
		return nil
	}
	fmt.Fprintf(stdout, i18n.T("✓ UNS %d bound to Flow %d\n", "✓ UNS %d 已绑定到 Flow %d\n"), unsID, flowID)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
