package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get flow details",
	RunE:  runFlowGet,
}

func init() {
	flowGetCmd.Flags().Int64("id", 0, "Flow ID")
}

func runFlowGet(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	id, _ := cmd.Flags().GetInt64("id")

	// Accept positional arg as ID fallback.
	if id == 0 && len(args) > 0 {
		parsedID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return invalidArgumentCause(cmd, "flow ID", "flow ID must be an integer: "+err.Error(), err)
		}
		id = parsedID
	}
	if id <= 0 {
		return invalidArgument(cmd, "--id", "specify a positive Flow ID via --id <id> or as a positional argument")
	}

	body, _ := json.Marshal(map[string]int64{"id": id})
	checker := notice.Start()
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
	fav := "no"
	if f.IsFavorite == 1 {
		fav = "yes"
	}
	fmt.Fprintf(stdout, "%-16s %d\n", "ID:", f.Id)
	fmt.Fprintf(stdout, "%-16s %s\n", "FlowId:", f.FlowId)
	fmt.Fprintf(stdout, "%-16s %s\n", "Name:", f.FlowName)
	fmt.Fprintf(stdout, "%-16s %s\n", "Type:", f.FlowType)
	fmt.Fprintf(stdout, "%-16s %s\n", "Status:", f.FlowStatus)
	fmt.Fprintf(stdout, "%-16s %s\n", "Description:", f.Description)
	fmt.Fprintf(stdout, "%-16s %s\n", "Favorite:", fav)
	fmt.Fprintf(stdout, "%-16s %s (%s)\n",
		"Version:", f.CurrentVersionName, f.CurrentVersionType)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
