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

var flowDataCmd = &cobra.Command{
	Use:   "data",
	Short: i18n.T("Get Node-RED canvas JSON", "获取 Node-RED 画布 JSON 数据"),
	RunE:  runFlowData,
}

func init() {
	flowDataCmd.Flags().Int64("id", 0, i18n.T("Flow ID", "Flow ID"))
	flowDataCmd.Flags().StringP("out", "o", "",
		i18n.T("Save output to file", "将结果保存到文件"))
}

func runFlowData(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	debug, _ := cmd.Flags().GetBool("debug")
	id, _ := cmd.Flags().GetInt64("id")
	outFile, _ := cmd.Flags().GetString("out")

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
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/flowdata", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, true) // data always JSON error
	}

	stdout := cmd.OutOrStdout()

	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(resp), 0644); err != nil {
			return fmt.Errorf(i18n.T("failed to write file: %w", "写入文件失败: %w"), err)
		}
		fmt.Fprintf(stdout, i18n.T("✓ Flow data saved to %s\n", "✓ Flow 数据已保存到 %s\n"), outFile)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}

	checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
	return nil
}
