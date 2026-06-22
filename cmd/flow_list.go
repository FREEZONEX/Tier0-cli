package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

const (
	flowTypeSource = "SourceFlow"
	flowTypeEvent  = "EventFlow"
)

var flowListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List flows",
	RunE:    runFlowList,
}

func init() {
	flowListCmd.Flags().StringP("keyword", "k", "",
		"Filter by name keyword")
	flowListCmd.Flags().StringP("type", "t", "",
		"Filter by type (SourceFlow/EventFlow)")
	flowListCmd.Flags().Bool("source", false,
		"Show SourceFlow only")
	flowListCmd.Flags().Bool("event", false,
		"Show EventFlow only")
}

func runFlowList(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	keyword, _ := cmd.Flags().GetString("keyword")
	flowType, _ := cmd.Flags().GetString("type")

	if source, _ := cmd.Flags().GetBool("source"); source {
		flowType = flowTypeSource
	}
	if event, _ := cmd.Flags().GetBool("event"); event {
		flowType = flowTypeEvent
	}

	body, _ := json.Marshal(map[string]string{
		"keyword":  keyword,
		"flowType": flowType,
	})

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/list", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()

	if jsonMode {
		checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
		return nil
	}

	var result struct {
		List []struct {
			Id                 int64  `json:"id"`
			FlowName           string `json:"flowName"`
			FlowType           string `json:"flowType"`
			FlowStatus         string `json:"flowStatus"`
			Description        string `json:"description"`
			IsFavorite         int64  `json:"isFavorite"`
			CurrentVersionName string `json:"currentVersionName"`
		} `json:"list"`
	}
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		return nil
	}
	if len(result.List) == 0 {
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		fmt.Fprintln(stdout, "No flows found.")
		return nil
	}
	fmt.Fprintf(stdout, "%-6s  %-12s  %-26s  %-8s  %s\n",
		"ID",
		"Type",
		"Name",
		"Status",
		"Description",
	)
	fmt.Fprintln(stdout, strings.Repeat("-", 80))
	for _, f := range result.List {
		fav := ""
		if f.IsFavorite == 1 {
			fav = " ★"
		}
		fmt.Fprintf(stdout, "%-6d  %-12s  %-26s  %-8s  %s%s\n",
			f.Id, f.FlowType, f.FlowName, f.FlowStatus, f.Description, fav)
	}
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
