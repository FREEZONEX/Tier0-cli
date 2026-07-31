package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

var assetsUploadCmd = &cobra.Command{
	Use:   "upload <local-file>",
	Short: "Upload a file to Tier0 object storage",
	Long: `Upload a local file to Tier0 object storage.

The command first requests a presigned PUT URL from the backend, then uploads the file content directly to S3/RustFS.

Examples:
  tier0 assets upload ./report.csv
  tier0 assets upload ./report.csv --visibility public --business attachment
  tier0 assets upload ./report.csv --use-by workspace --visibility private`,
	Args: cobra.ExactArgs(1),
	RunE: runAssetsUpload,
}

func init() {
	assetsUploadCmd.Flags().String("business", "attachment", "Business scene")
	assetsUploadCmd.Flags().String("use-by", "workspace", "Usage scope: user|workspace|platform")
	assetsUploadCmd.Flags().String("visibility", "private", "Visibility: public|private")
	assetsUploadCmd.Flags().String("app-instance-id", "", "AI app instance ID")
	assetsUploadCmd.Flags().String("session-id", "", "AI session ID")
}

type assetsUploadResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		FileID    int64  `json:"fileId"`
		FilePath  string `json:"filePath"`
		FileURL   string `json:"fileUrl"`
		UploadURL string `json:"uploadUrl"`
		ExpiresAt int64  `json:"expiresAt"`
	} `json:"data"`
}

func runAssetsUpload(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	localPath := args[0]

	info, err := os.Stat(localPath)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("stat file: %w", err), jsonMode)
	}
	if info.IsDir() {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("path is a directory"), jsonMode)
	}
	// 文件大小上限与配额由服务端按套餐裁定，CLI 不做大小硬校验；仅保留空文件友好预检
	if info.Size() <= 0 {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("file is empty"), jsonMode)
	}

	business, _ := cmd.Flags().GetString("business")
	useBy, _ := cmd.Flags().GetString("use-by")
	visibility, _ := cmd.Flags().GetString("visibility")
	appInstanceID, _ := cmd.Flags().GetString("app-instance-id")
	sessionID, _ := cmd.Flags().GetString("session-id")

	fileName := filepath.Base(localPath)
	contentType := mimeTypeByExtension(filepath.Ext(fileName))

	body := map[string]any{
		"fileName":      fileName,
		"contentType":   contentType,
		"size":          info.Size(),
		"business":      business,
		"useBy":         useBy,
		"visibility":    visibility,
		"appInstanceId": appInstanceID,
		"sessionId":     sessionID,
	}
	bodyJSON, _ := json.Marshal(body)

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/assets/files", "POST", string(bodyJSON), debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	var uploadResp assetsUploadResponse
	if err := json.Unmarshal([]byte(resp), &uploadResp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("parse upload response: %w", err), jsonMode)
	}
	if uploadResp.Code != 0 && uploadResp.Code != 200 {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), apierr.New(uploadResp.Code, resp), jsonMode)
	}
	if uploadResp.Data.UploadURL == "" || uploadResp.Data.FilePath == "" {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("invalid upload response: missing uploadUrl or filePath"), jsonMode)
	}

	if err := putFileToURL(cmd.Context(), uploadResp.Data.UploadURL, localPath, contentType, debug); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	result := map[string]any{
		"filePath": uploadResp.Data.FilePath,
		"fileUrl":  uploadResp.Data.FileURL,
		"fileId":   uploadResp.Data.FileID,
	}
	if jsonMode {
		fmt.Fprintln(cmd.OutOrStdout(), cmdutil.JSONString(result))
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Uploaded")
		fmt.Fprintf(cmd.OutOrStdout(), "  filePath: %s\n", uploadResp.Data.FilePath)
		if uploadResp.Data.FileURL != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  fileUrl:  %s\n", uploadResp.Data.FileURL)
		}
	}
	return nil
}

func putFileToURL(ctx context.Context, uploadURL, localPath, contentType string, debug bool) error {
	// 流式上传：以文件为请求体，避免整文件读入内存，支持大文件
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, f)
	if err != nil {
		return fmt.Errorf("build put request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = info.Size()

	if debug {
		fmt.Fprintf(os.Stderr, "[debug] PUT %s (body %d bytes)\n", uploadURL, info.Size())
	}
	// 大文件流式上传不设置客户端总超时：固定超时会中断慢速但正常的大文件上传。
	// 取消通过请求 context（cmd.Context()，响应 Ctrl-C）传递，与仓库其他 HTTP 调用一致。
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("put file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("put file failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	return nil
}

func mimeTypeByExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".txt":
		return "text/plain"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}
