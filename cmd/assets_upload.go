package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

var assetsUploadCmd = &cobra.Command{
	Use:   "upload <local-file>",
	Short: "Upload a file to Tier0 object storage",
	Long: `Upload a local file to Tier0 object storage.

Files larger than 100MB are uploaded with multipart chunked upload: the CLI
requests a presigned upload URL per chunk, uploads chunks directly to the
object store with configurable concurrency, then completes the upload.
Interrupted uploads keep a local resume state file and can be continued with
--resume, or discarded with --abort.

Examples:
  tier0 assets upload ./report.csv
  tier0 assets upload ./report.csv --visibility public --business attachment
  tier0 assets upload ./report.csv --use-by workspace --visibility private
  tier0 assets upload ./large-backup.tar.gz
  tier0 assets upload ./large-backup.tar.gz --resume
  tier0 assets upload ./large-backup.tar.gz --abort
  tier0 assets upload ./large-backup.tar.gz --multipart-size 20MB --concurrency 8`,
	Args: cobra.ExactArgs(1),
	RunE: runAssetsUpload,
}

func init() {
	assetsUploadCmd.Flags().String("business", "attachment", "Business scene")
	assetsUploadCmd.Flags().String("use-by", "workspace", "Usage scope: user|workspace|platform")
	assetsUploadCmd.Flags().String("visibility", "private", "Visibility: public|private")
	assetsUploadCmd.Flags().String("app-instance-id", "", "AI app instance ID")
	assetsUploadCmd.Flags().String("session-id", "", "AI session ID")
	// multipart 分片直传相关（>100MB 自动启用）
	assetsUploadCmd.Flags().String("multipart-size", fmt.Sprintf("%dMB", defaultMultipartSize/(1<<20)),
		"Multipart part size (e.g. 10MB, min 5MB)")
	assetsUploadCmd.Flags().Int("concurrency", 4, "Multipart upload concurrency (number of parallel parts)")
	assetsUploadCmd.Flags().Bool("resume", false, "Resume an interrupted multipart upload")
	assetsUploadCmd.Flags().Bool("abort", false, "Abort an interrupted multipart upload and clean up its state")
}

type assetsUploadResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		FileID     int64             `json:"fileId"`
		FilePath   string            `json:"filePath"`
		FileURL    string            `json:"fileUrl"`
		PostURL    string            `json:"postUrl"`
		PostFields map[string]string `json:"postFields"`
		ExpiresAt  int64             `json:"expiresAt"`
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

	resume, _ := cmd.Flags().GetBool("resume")
	abort, _ := cmd.Flags().GetBool("abort")

	// 大文件（>100MB）自动分片直传；存在断点状态或显式 --resume/--abort 也走 multipart 流程
	if info.Size() > multipartThreshold || resume || abort || multipartStateExists(localPath) {
		return runMultipartUpload(cmd, localPath, info, multipartOptions{
			business:      business,
			useBy:         useBy,
			visibility:    visibility,
			appInstanceID: appInstanceID,
			sessionID:     sessionID,
			resume:        resume,
			abort:         abort,
		})
	}

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
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), apierr.NewBusiness(uploadResp.Code, resp), jsonMode)
	}
	if uploadResp.Data.PostURL == "" || uploadResp.Data.FilePath == "" {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("invalid upload response: missing postUrl or filePath"), jsonMode)
	}

	if err := postFileToURL(cmd.Context(), uploadResp.Data.PostURL, uploadResp.Data.PostFields, localPath, contentType, debug); err != nil {
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

func postFileToURL(ctx context.Context, postURL string, postFields map[string]string, localPath, contentType string, debug bool) error {
	// 流式 multipart/form-data：postFields 与 file part header 拼进头部缓冲，
	// 文件内容以 io.MultiReader 拼接，避免整文件读入内存，支持大文件
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	// 1) postFields 全部先写入表单（按键排序，输出确定）；
	// 2) file 字段必须最后 appended：对象存储签名校验对字段顺序敏感，file 之后不得再有字段。
	var head bytes.Buffer
	mw := multipart.NewWriter(&head)
	keys := make([]string, 0, len(postFields))
	for k := range postFields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := mw.WriteField(k, postFields[k]); err != nil {
			return fmt.Errorf("write form field %s: %w", k, err)
		}
	}
	// file part header（Content-Type 由 CLI 按扩展名推断；不额外添加 Content-Type 表单字段）
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filepath.Base(localPath)))
	partHeader.Set("Content-Type", contentType)
	if _, err := mw.CreatePart(partHeader); err != nil {
		return fmt.Errorf("create file form field: %w", err)
	}

	// 收尾 boundary 与文件内容流式拼接
	tail := fmt.Sprintf("\r\n--%s--\r\n", mw.Boundary())
	body := io.MultiReader(bytes.NewReader(head.Bytes()), f, strings.NewReader(tail))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, body)
	if err != nil {
		return fmt.Errorf("build post request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.ContentLength = int64(head.Len()) + info.Size() + int64(len(tail))

	if debug {
		fmt.Fprintf(os.Stderr, "[debug] POST %s (multipart body %d bytes)\n", postURL, req.ContentLength)
	}
	// 大文件流式上传不设置客户端总超时：固定超时会中断慢速但正常的大文件上传。
	// 取消通过请求 context（cmd.Context()，响应 Ctrl-C）传递，与仓库其他 HTTP 调用一致。
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyText, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post file failed: status=%d body=%s", resp.StatusCode, string(bodyText))
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
