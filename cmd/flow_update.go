package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var flowUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update flow metadata",
	RunE:  runFlowUpdate,
}

func init() {
	flowUpdateCmd.Flags().Int64("id", 0, "Flow ID (required)")
	flowUpdateCmd.Flags().StringP("name", "n", "",
		"New name")
	flowUpdateCmd.Flags().String("desc", "",
		"New description")
	flowUpdateCmd.Flags().String("template", "",
		"New template JSON string")
	flowUpdateCmd.Flags().String("template-file", "",
		"Read new template from file")
	flowUpdateCmd.Flags().Bool("favorite", false,
		"Mark as favorite")
	flowUpdateCmd.Flags().Bool("unfavorite", false,
		"Remove from favorites")
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
		return fmt.Errorf(
			"specify a Flow ID via --id <id> or as a positional argument",
		)
	}

	if templateFile != "" {
		raw, err := os.ReadFile(templateFile)
		if err != nil {
			return fmt.Errorf("failed to read template file: %w", err)
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
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	if jsonMode {
		checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
		return nil
	}
	_ = resp
	fmt.Fprintf(stdout, "✓ Flow %d updated\n", id)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
