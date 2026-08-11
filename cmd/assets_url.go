package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

var assetsUrlCmd = &cobra.Command{
	Use:   "url",
	Short: "Get file access URL from Tier0 object storage",
	Long: `Get a file access URL from Tier0 object storage.

Examples:
  tier0 assets url --file-path workspace/.../report.csv
  tier0 assets url --file-path workspace/.../report.csv --expired-sec 300`,
	RunE: runAssetsUrl,
}

func init() {
	assetsUrlCmd.Flags().String("file-path", "", "File path returned by upload")
	assetsUrlCmd.Flags().Int("expired-sec", 3600, "Presigned URL expiration seconds")
	assetsUrlCmd.Flags().String("content-disposition", "", "Custom Content-Disposition")
	_ = assetsUrlCmd.MarkFlagRequired("file-path")
}

type assetsUrlResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		FileURL   string `json:"fileUrl"`
		ExpiresAt int64  `json:"expiresAt"`
	} `json:"data"`
}

func runAssetsUrl(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	filePath, _ := cmd.Flags().GetString("file-path")
	expiredSec, _ := cmd.Flags().GetInt("expired-sec")
	contentDisposition, _ := cmd.Flags().GetString("content-disposition")

	u := fmt.Sprintf("/openapi/v1/assets/files/url?filePath=%s&expiredSec=%d", url.QueryEscape(filePath), expiredSec)
	if contentDisposition != "" {
		u += "&responseContentDisposition=" + url.QueryEscape(contentDisposition)
	}

	resp, err := cmdutil.DoAPI(cmd.Context(), u, "GET", "", debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	var urlResp assetsUrlResponse
	if err := json.Unmarshal([]byte(resp), &urlResp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("parse url response: %w", err), jsonMode)
	}
	if urlResp.Code != 0 && urlResp.Code != 200 {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), apierr.NewBusiness(urlResp.Code, resp), jsonMode)
	}

	result := map[string]any{
		"fileUrl":   urlResp.Data.FileURL,
		"expiresAt": urlResp.Data.ExpiresAt,
	}
	if jsonMode {
		fmt.Fprintln(cmd.OutOrStdout(), cmdutil.JSONString(result))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", urlResp.Data.FileURL)
		if urlResp.Data.ExpiresAt > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "expiresAt: %d\n", urlResp.Data.ExpiresAt)
		}
	}
	return nil
}
