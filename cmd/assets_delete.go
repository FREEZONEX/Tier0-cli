package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/spf13/cobra"
)

var assetsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a file from Tier0 object storage",
	Long: `Delete a file from Tier0 object storage by filePath.

Examples:
  tier0 assets delete --file-path workspace/.../report.csv --dry-run --json
  tier0 assets delete --file-path workspace/.../report.csv --yes`,
	RunE: runAssetsDelete,
}

func init() {
	assetsDeleteCmd.Flags().String("file-path", "", "File path returned by upload")
	assetsDeleteCmd.Flags().BoolP("yes", "y", false,
		"Confirm high-risk operation (required)")
	addDryRunFlag(assetsDeleteCmd)
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
	confirmed, _ := cmd.Flags().GetBool("yes")

	body := map[string]any{"filePath": filePath}
	if handled, err := writeDryRun(cmd, "POST", "/openapi/v1/assets/files/delete", body); handled {
		return err
	}
	if err := highrisk.Guard(confirmed, "assets delete",
		fmt.Sprintf("Delete object-storage file %q; this cannot be undone.", filePath)); err != nil {
		return err
	}
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
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), apierr.NewBusiness(delResp.Code, resp), jsonMode)
	}
	if !delResp.Data.Deleted {
		return apiCommandError(cmd, "the server did not delete the requested file", nil)
	}

	result := map[string]any{"deleted": delResp.Data.Deleted, "filePath": filePath}
	if jsonMode {
		fmt.Fprintln(cmd.OutOrStdout(), cmdutil.JSONString(result))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Deleted %s\n", filePath)
	}
	return nil
}
