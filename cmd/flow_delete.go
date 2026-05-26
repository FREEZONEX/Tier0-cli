package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowDeleteCmd = &cobra.Command{
	Use:     "delete",
	Aliases: []string{"del", "rm"},
	Short:   i18n.T("Delete flow(s)", "删除 Flow"),
	RunE:    runFlowDelete,
}

func init() {
	flowDeleteCmd.Flags().Int64Slice("id", nil,
		i18n.T("Flow ID(s) to delete (repeatable)", "要删除的 Flow ID（可重复指定多个）"))
	flowDeleteCmd.Flags().BoolP("yes", "y", false,
		i18n.T("Confirm high-risk operation (required)", "确认高风险操作（必填）"))
}

func runFlowDelete(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	confirmed, _ := cmd.Flags().GetBool("yes")
	ids, _ := cmd.Flags().GetInt64Slice("id")

	// Also parse positional args as comma-separated IDs.
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			n, err := strconv.ParseInt(part, 10, 64)
			if err == nil {
				ids = append(ids, n)
			}
		}
	}

	if len(ids) == 0 {
		return fmt.Errorf(i18n.T(
			"specify at least one Flow ID via --id <id> or as positional arguments (comma-separated)",
			"请通过 --id <id> 或直接传入 ID（支持多个，逗号分隔）指定要删除的 Flow",
		))
	}

	summary := i18n.T(
		fmt.Sprintf("Delete Flow(s) %v — this will STOP the Node-RED container(s) and cannot be undone.", ids),
		fmt.Sprintf("删除 Flow %v — 将停止对应的 Node-RED 容器，操作不可逆。", ids),
	)
	if err := highrisk.Guard(confirmed, "flow delete", summary); err != nil {
		return err
	}

	body, _ := json.Marshal(map[string][]int64{"ids": ids})
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/delete", "POST", string(body), debug)
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
	_ = resp
	if len(ids) == 1 {
		fmt.Fprintf(stdout, i18n.T("✓ Flow %d deleted\n", "✓ Flow %d 已删除\n"), ids[0])
	} else {
		fmt.Fprintf(stdout, i18n.T("✓ %d flows deleted\n", "✓ 已删除 %d 个 Flow\n"), len(ids))
	}
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
