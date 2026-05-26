package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowNodesCmd = &cobra.Command{
	Use:   "nodes [source|event]",
	Short: i18n.T("List available Node-RED node types", "列出可用 Node-RED 节点类型"),
	Long: i18n.T(
		"List Node-RED node types available for a SourceFlow or EventFlow.",
		"列出 SourceFlow 或 EventFlow 当前可用的 Node-RED 节点类型。",
	),
	RunE: runFlowNodes,
}

func init() {
	flowNodesCmd.Flags().StringP("type", "t", "",
		i18n.T("Flow type (SourceFlow/EventFlow)", "Flow 类型 (SourceFlow/EventFlow)"))
	flowNodesCmd.Flags().Bool("source", false,
		i18n.T("Show SourceFlow nodes", "查看 SourceFlow 可用节点"))
	flowNodesCmd.Flags().Bool("event", false,
		i18n.T("Show EventFlow nodes", "查看 EventFlow 可用节点"))
}

func runFlowNodes(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	flowType, _ := cmd.Flags().GetString("type")

	if flowType == "" && len(args) > 0 {
		flowType = args[0]
	}
	if source, _ := cmd.Flags().GetBool("source"); source {
		flowType = flowTypeSource
	}
	if event, _ := cmd.Flags().GetBool("event"); event {
		flowType = flowTypeEvent
	}

	flowType, err := normalizeFlowNodesType(flowType)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]string{"flowType": flowType})
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/flow/nodes", "POST", string(body), debug)
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

	var result struct {
		Nodes []struct {
			Id      string   `json:"id"`
			Name    string   `json:"name"`
			Types   []string `json:"types"`
			Enabled bool     `json:"enabled"`
			Module  string   `json:"module"`
			Version string   `json:"version"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}
	if len(result.Nodes) == 0 {
		fmt.Fprintf(stdout, i18n.T("No Node-RED nodes found for %s.\n", "%s 暂无可用 Node-RED 节点。\n"), flowType)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}

	fmt.Fprintf(stdout, "%-28s  %-8s  %-18s  %s\n",
		i18n.T("Name", "名称"),
		i18n.T("Enabled", "启用"),
		i18n.T("Module", "模块"),
		i18n.T("Types", "类型"),
	)
	fmt.Fprintln(stdout, strings.Repeat("-", 100))
	for _, item := range result.Nodes {
		enabled := i18n.T("no", "否")
		if item.Enabled {
			enabled = i18n.T("yes", "是")
		}
		module := item.Module
		if module == "" {
			module = "-"
		}
		if item.Module != "" && item.Version != "" {
			module = fmt.Sprintf("%s@%s", item.Module, item.Version)
		}
		name := item.Name
		if name == "" {
			name = item.Id
		}
		fmt.Fprintf(stdout, "%-28s  %-8s  %-18s  %s\n",
			name,
			enabled,
			module,
			strings.Join(item.Types, ", "),
		)
	}
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}

func normalizeFlowNodesType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "source", "sourceflow", "flowtypesource":
		return flowTypeSource, nil
	case "event", "eventflow", "flowtypeevent":
		return flowTypeEvent, nil
	case "":
		return "", errors.New(i18n.T(
			"specify a Flow type via --source, --event, --type SourceFlow|EventFlow, or positional source|event",
			"请通过 --source、--event、--type SourceFlow|EventFlow 或位置参数 source|event 指定 Flow 类型",
		))
	default:
		return "", errors.New(i18n.T(
			"flow type must be SourceFlow or EventFlow",
			"Flow 类型必须是 SourceFlow 或 EventFlow",
		))
	}
}
