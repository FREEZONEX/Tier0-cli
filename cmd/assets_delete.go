package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

var assetsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a file from Tier0 object storage",
	Long: `Delete a file from Tier0 object storage by filePath.

Examples:
  tier0 assets delete --file-path workspace/.../report.csv`,
	RunE: runAssetsDelete,
}

func init() {
	assetsDeleteCmd.Flags().String("file-path", "", "File path returned by upload")
	_ = assetsDeleteCmd.MarkFlagRequired("file-path")
}

type assetsDeleteResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Deleted bool `json:"deleted"`
	} `json:"data"`
}

func runAssetsDelete(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	filePath, _ := cmd.Flags().GetString("file-path")

	body := map[string]any{"filePath": filePath}
	bodyJSON, _ := json.Marshal(body)

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/assets/files/delete", "POST", string(bodyJSON), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	var delResp assetsDeleteResponse
	if err := json.Unmarshal([]byte(resp), &delResp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("parse delete response: %w", err), jsonMode)
	}
	if delResp.Code != 0 && delResp.Code != 200 {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), apierr.New(delResp.Code, resp), jsonMode)
	}

	result := map[string]any{"deleted": delResp.Data.Deleted, "filePath": filePath}
	if jsonMode {
		fmt.Fprintln(cmd.OutOrStdout(), cmdutil.JSONString(result))
	} else {
		if delResp.Data.Deleted {
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Deleted %s\n", filePath)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "✗ Delete failed %s\n", filePath)
		}
	}
	return nil
}
