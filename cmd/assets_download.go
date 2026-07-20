package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/spf13/cobra"
)

var assetsDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a file from Tier0 object storage",
	Long: `Download a file from Tier0 object storage by filePath.

Examples:
  tier0 assets download --file-path workspace/.../report.csv -o ./report.csv
  tier0 assets download --file-path workspace/.../report.csv`,
	RunE: runAssetsDownload,
}

func init() {
	assetsDownloadCmd.Flags().String("file-path", "", "File path returned by upload")
	assetsDownloadCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	assetsDownloadCmd.Flags().String("content-disposition", "", "Custom Content-Disposition")
	_ = assetsDownloadCmd.MarkFlagRequired("file-path")
}

func runAssetsDownload(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	filePath, _ := cmd.Flags().GetString("file-path")
	output, _ := cmd.Flags().GetString("output")
	contentDisposition, _ := cmd.Flags().GetString("content-disposition")

	profile, err := config.LoadProfile()
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if profile.APIKey == "" {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("API Key not found"), jsonMode)
	}

	u := fmt.Sprintf("%s/openapi/v1/assets/files/download?filePath=%s", profile.BaseURL, url.QueryEscape(filePath))
	if contentDisposition != "" {
		u += "&responseContentDisposition=" + url.QueryEscape(contentDisposition)
	}

	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, u, nil)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	req.Header.Set("x-api-key", profile.APIKey)

	if debug {
		fmt.Fprintf(os.Stderr, "[debug] GET %s\n", u)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("download: %w", err), jsonMode)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("download failed: status=%d body=%s", resp.StatusCode, string(body)), jsonMode)
	}

	var out io.Writer = cmd.OutOrStdout()
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("create output: %w", err), jsonMode)
		}
		defer f.Close()
		out = f
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("write output: %w", err), jsonMode)
	}

	if output != "" && !jsonMode {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Downloaded to %s\n", output)
	} else if jsonMode {
		fmt.Fprintln(cmd.OutOrStdout(), cmdutil.JSONString(map[string]any{"output": output, "filePath": filePath}))
	}
	return nil
}
