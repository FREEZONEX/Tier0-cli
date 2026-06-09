package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsAttachmentsCmd = &cobra.Command{
	Use:     "attachments",
	Aliases: []string{"attachment", "files"},
	Short:   i18n.T("Manage UNS attachments", "管理 UNS 附件"),
	Long: i18n.T(
		"Upload and list files bound to a UNS node by unsId.",
		"按 unsId 上传和查询绑定到 UNS 节点的附件。",
	),
}

var unsAttachmentUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: i18n.T("Upload a file attachment to a UNS node", "上传 UNS 节点附件"),
	RunE:  runUnsAttachmentUpload,
}

var unsAttachmentListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   i18n.T("List attachments of a UNS node", "查询 UNS 节点附件"),
	RunE:    runUnsAttachmentList,
}

func init() {
	unsAttachmentsCmd.AddCommand(unsAttachmentUploadCmd)
	unsAttachmentsCmd.AddCommand(unsAttachmentListCmd)

	unsAttachmentUploadCmd.Flags().Int64("uns-id", 0,
		i18n.T("UNS node ID (required)", "UNS 节点 ID（必填）"))
	unsAttachmentUploadCmd.Flags().StringP("file", "f", "",
		i18n.T("Local file path to upload (required)", "要上传的本地文件路径（必填）"))
	unsAttachmentUploadCmd.Flags().String("file-name", "",
		i18n.T("Override attachment fileName", "覆盖附件 fileName"))
	unsAttachmentUploadCmd.Flags().String("sha256", "",
		i18n.T("Optional client-side sha256", "可选的客户端 sha256"))
	unsAttachmentUploadCmd.MarkFlagRequired("uns-id")
	unsAttachmentUploadCmd.MarkFlagRequired("file")

	unsAttachmentListCmd.Flags().Int64("uns-id", 0,
		i18n.T("UNS node ID (required)", "UNS 节点 ID（必填）"))
	unsAttachmentListCmd.Flags().Int("page-no", 1,
		i18n.T("Page number", "页码"))
	unsAttachmentListCmd.Flags().Int("page-size", 20,
		i18n.T("Page size", "每页条数"))
	unsAttachmentListCmd.Flags().Bool("include-file-url", true,
		i18n.T("Include downloadable fileUrl", "返回可下载 fileUrl"))
	unsAttachmentListCmd.MarkFlagRequired("uns-id")
}

func runUnsAttachmentUpload(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	unsID, _ := cmd.Flags().GetInt64("uns-id")
	filePath, _ := cmd.Flags().GetString("file")
	fileName, _ := cmd.Flags().GetString("file-name")
	sha256, _ := cmd.Flags().GetString("sha256")

	if unsID <= 0 {
		return fmt.Errorf(i18n.T("uns-id is required", "uns-id 为必填"))
	}
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf(i18n.T("file is required", "file 为必填"))
	}

	endpoint := fmt.Sprintf("/openapi/v1/uns/%d/attachments", unsID)
	fields := map[string]string{
		"fileName": fileName,
		"sha256":   sha256,
	}
	resp, err := cmdutil.DoMultipart(cmd.Context(), endpoint, "file", filePath, fileName, fields, debug)
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
		UnsID    int64  `json:"unsId"`
		FileName string `json:"fileName"`
		FilePath string `json:"filePath"`
		FileURL  string `json:"fileUrl"`
	}
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}
	fmt.Fprintf(stdout, i18n.T("✓ Attachment uploaded: %s\n", "✓ 附件上传成功: %s\n"), result.FileName)
	fmt.Fprintf(stdout, "unsId: %d\nfilePath: %s\n", result.UnsID, result.FilePath)
	if result.FileURL != "" {
		fmt.Fprintf(stdout, "fileUrl: %s\n", result.FileURL)
	}
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}

func runUnsAttachmentList(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	unsID, _ := cmd.Flags().GetInt64("uns-id")
	pageNo, _ := cmd.Flags().GetInt("page-no")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	includeFileURL, _ := cmd.Flags().GetBool("include-file-url")

	if unsID <= 0 {
		return fmt.Errorf(i18n.T("uns-id is required", "uns-id 为必填"))
	}
	q := url.Values{}
	q.Set("pageNo", strconv.Itoa(pageNo))
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("includeFileUrl", strconv.FormatBool(includeFileURL))
	endpoint := fmt.Sprintf("/openapi/v1/uns/%d/attachments?%s", unsID, q.Encode())

	resp, err := cmdutil.DoAPI(cmd.Context(), endpoint, "GET", "", debug)
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
		List []struct {
			FileName string `json:"fileName"`
			FilePath string `json:"filePath"`
			FileURL  string `json:"fileUrl"`
		} `json:"list"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		return nil
	}
	if len(result.List) == 0 {
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
		fmt.Fprintln(stdout, i18n.T("No attachments found.", "暂无附件。"))
		return nil
	}
	fmt.Fprintf(stdout, "%-30s  %-48s  %s\n", i18n.T("FileName", "文件名"), "FilePath", "FileUrl")
	fmt.Fprintln(stdout, strings.Repeat("-", 120))
	for _, item := range result.List {
		fmt.Fprintf(stdout, "%-30s  %-48s  %s\n", item.FileName, item.FilePath, item.FileURL)
	}
	fmt.Fprintf(stdout, i18n.T("Total: %d\n", "总数: %d\n"), result.Total)
	checker.Emit("", false, stdout, cmd.ErrOrStderr())
	return nil
}
