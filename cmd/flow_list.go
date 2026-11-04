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
	flowListCmd.Flags().Bool("cloud", false, "List Cloud stream programs using the native Cloud contract")
	flowListCmd.Flags().Int("page", 1, "Cloud result page (requires --cloud)")
	flowListCmd.Flags().Int("size", 20, "Cloud page size, 1–200 (requires --cloud)")
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
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	keyword, _ := cmd.Flags().GetString("keyword")
	flowType, _ := cmd.Flags().GetString("type")
	source, _ := cmd.Flags().GetBool("source")
	event, _ := cmd.Flags().GetBool("event")

	if source && event {
		return invalidArgument(cmd, "--source/--event", "--source and --event are mutually exclusive")
	}
	if cmd.Flags().Changed("type") && (source || event) {
		return invalidArgument(cmd, "--type", "--type cannot be combined with --source or --event")
	}
	if source {
		flowType = flowTypeSource
	}
	if event {
		flowType = flowTypeEvent
	}
	if flowType != "" && flowType != flowTypeSource && flowType != flowTypeEvent {
		return invalidArgument(cmd, "--type", "--type must be SourceFlow or EventFlow")
	}

	cloud, _ := cmd.Flags().GetBool("cloud")
	if cloud {
		if flowType != "" {
			return invalidArgument(cmd, "--type/--source/--event", "Cloud streams do not use Enterprise Flow types")
		}
		page, _ := cmd.Flags().GetInt("page")
		size, _ := cmd.Flags().GetInt("size")
		if page < 1 {
			return invalidArgument(cmd, "--page", "Cloud page must be at least 1")
		}
		if size < 1 || size > 200 {
			return invalidArgument(cmd, "--size", "Cloud size must be between 1 and 200")
		}
		body, _ := json.Marshal(map[string]any{"search": keyword, "page": page, "size": size})
		resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v2/uns/streams/program/list", "POST", string(body), debug)
		if err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
		}
		if err := cmdutil.CheckResponse(resp); err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
		}
		var envelope struct {
			Code *int `json:"code"`
			Data *struct {
				List []json.RawMessage `json:"list"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(resp), &envelope) != nil || envelope.Code == nil || (*envelope.Code != 0 && *envelope.Code != 200) || envelope.Data == nil || envelope.Data.List == nil {
			return apiCommandError(cmd, "invalid Cloud stream list response", nil)
		}
		checker := notice.Start()
		checker.Emit(resp, jsonMode, cmd.OutOrStdout(), cmd.ErrOrStderr())
		if !jsonMode {
			fmt.Fprintln(cmd.OutOrStdout(), resp)
		}
		return nil
	}
	if cmd.Flags().Changed("page") || cmd.Flags().Changed("size") {
		return invalidArgument(cmd, "--page/--size", "pagination flags require --cloud")
	}

	body, _ := json.Marshal(map[string]string{
		"keyword":  keyword,
		"flowType": flowType,
	})

	checker := notice.Start()
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/list", "POST", string(body), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckResponse(resp); err != nil {
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
