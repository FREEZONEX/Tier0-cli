package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowNodesCmd = &cobra.Command{
	Use:   "nodes [source|event]",
	Short: "List available Node-RED node types",
	Long:  "List Node-RED node types available for a SourceFlow or EventFlow.",

	RunE: runFlowNodes,
}

func init() {
	flowNodesCmd.Flags().StringP("type", "t", "",
		"Flow type (SourceFlow/EventFlow)")
	flowNodesCmd.Flags().Bool("source", false,
		"Show SourceFlow nodes")
	flowNodesCmd.Flags().Bool("event", false,
		"Show EventFlow nodes")
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
		fmt.Fprintf(stdout, "No Node-RED nodes found for %s.\n", flowType)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}

	fmt.Fprintf(stdout, "%-28s  %-8s  %-18s  %s\n",
		"Name",
		"Enabled",
		"Module",
		"Types",
	)
	fmt.Fprintln(stdout, strings.Repeat("-", 100))
	for _, item := range result.Nodes {
		enabled := "no"
		if item.Enabled {
			enabled = "yes"
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
		return "", errors.New(
			"specify a Flow type via --source, --event, --type SourceFlow|EventFlow, or positional source|event",
		)
	default:
		return "", errors.New(
			"flow type must be SourceFlow or EventFlow",
		)
	}
}
