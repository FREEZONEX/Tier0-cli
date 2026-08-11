package cmd

// 大文件分片上传（multipart）实现（TASK-025）。
//
// 纯客户端直传：通过 API 网关获取预签名分片 URL，客户端并发 PUT 直传对象存储，
// 最后调用 complete 合并。中断后在本地保留状态文件（.tier0-upload-<hash>.json），
// 支持 --resume 跳过已完成分片续传，--abort 放弃上传并清理状态。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
	"github.com/spf13/cobra"
)

const (
	// 超过该大小的文件自动切换为分片直传，避免单次 POST 上传超时
	multipartThreshold = 100 * 1024 * 1024
	// 默认分片大小 10MB，最小 5MB（后端限制）
	defaultMultipartSize = 10 * 1024 * 1024
	minMultipartSize     = 5 * 1024 * 1024
	// 单次 part-urls 请求携带的分片号数量上限
	partURLBatchSize = 1000
	// 单个分片 PUT 的超时与最大尝试次数
	partUploadTimeout     = 60 * time.Second
	partUploadMaxAttempts = 3
	// 状态文件名中路径 hash 的截断长度（十六进制位）
	stateHashLen = 16
)

// multipartOptions 汇集 assets upload 中与分片上传相关的选项。
type multipartOptions struct {
	business      string
	useBy         string
	visibility    string
	appInstanceID string
	sessionID     string
	resume        bool
	abort         bool
}

// uploadState 是断点续传状态文件的磁盘内容。
// mu 仅用于内存并发访问保护，不参与序列化。
type uploadState struct {
	mu        sync.Mutex
	LocalPath string         `json:"localPath"`
	FileName  string         `json:"fileName"`
	Size      int64          `json:"size"`
	FileKey   string         `json:"fileKey"`
	UploadID  string         `json:"uploadId"`
	PartSize  int64          `json:"partSize"`
	PartCount int            `json:"partCount"`
	Parts     map[int]string `json:"parts"` // partNumber -> etag，已上传成功分片
	StartedAt time.Time      `json:"startedAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// isCompleted 判断指定分片是否已上传成功（续传时跳过）。
func (s *uploadState) isCompleted(n int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.Parts[n]
	return ok
}

// addPart 记录一个上传成功分片的 ETag。
func (s *uploadState) addPart(n int, etag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Parts == nil {
		s.Parts = map[int]string{}
	}
	s.Parts[n] = etag
	s.UpdatedAt = time.Now()
}

// partLen 返回指定分片应上传的字节数（最后一片可能不足 partSize）。
func (s *uploadState) partLen(n int) int64 {
	offset := (int64(n) - 1) * s.PartSize
	length := s.Size - offset
	if length > s.PartSize {
		length = s.PartSize
	}
	return length
}

// stateStore 负责状态文件的读写，写盘时先写临时文件再原子 rename，
// 避免进程被强杀时留下半个 JSON。
type stateStore struct {
	path string
	mu   sync.Mutex
}

func newStateStore(path string) *stateStore {
	return &stateStore{path: path}
}

func (s *stateStore) exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

func (s *stateStore) load() (*uploadState, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var st uploadState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("parse resume state %s: %w", s.path, err)
	}
	if st.Parts == nil {
		st.Parts = map[int]string{}
	}
	return &st, nil
}

func (s *stateStore) save(st *uploadState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st.mu.Lock()
	b, err := json.Marshal(st)
	st.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal resume state: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write resume state: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename resume state: %w", err)
	}
	return nil
}

// partURLProvider 按需（按批）拉取预签名分片 URL 并缓存。
type partURLProvider struct {
	ctx       context.Context
	fileKey   string
	uploadID  string
	partCount int
	debug     bool

	mu   sync.Mutex
	urls map[int]string
}

func newPartURLProvider(ctx context.Context, state *uploadState, debug bool) *partURLProvider {
	return &partURLProvider{
		ctx:       ctx,
		fileKey:   state.FileKey,
		uploadID:  state.UploadID,
		partCount: state.PartCount,
		debug:     debug,
		urls:      map[int]string{},
	}
}

// url 返回指定分片的预签名 URL；未缓存时从 n 起连续拉取一批。
func (p *partURLProvider) url(n int) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if u, ok := p.urls[n]; ok {
		return u, nil
	}
	nums := make([]int, 0, partURLBatchSize)
	for i := n; i <= p.partCount && len(nums) < partURLBatchSize; i++ {
		if _, ok := p.urls[i]; !ok {
			nums = append(nums, i)
		}
	}
	if len(nums) == 0 {
		return "", fmt.Errorf("no URL cached for part %d", n)
	}
	infos, err := multipartPartURLs(p.ctx, p.fileKey, p.uploadID, nums, p.debug)
	if err != nil {
		return "", err
	}
	for _, info := range infos {
		if info.URL != "" {
			p.urls[info.PartNumber] = info.URL
		}
	}
	if u, ok := p.urls[n]; ok {
		return u, nil
	}
	return "", fmt.Errorf("part %d URL missing from part-urls response", n)
}

// invalidate 清除指定分片 URL 缓存（如 403 过期），下次上传前重拉。
func (p *partURLProvider) invalidate(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.urls, n)
}

// progress 向 stderr 输出上传进度；TTY 下用 \r 原地刷新，非 TTY 下按百分比换行输出。
type progress struct {
	w     io.Writer
	total int64
	tty   bool

	mu   sync.Mutex
	done int64
	last int
}

func newProgress(w io.Writer, total int64) *progress {
	tty := false
	if f, ok := w.(*os.File); ok {
		if st, err := f.Stat(); err == nil {
			tty = st.Mode()&os.ModeCharDevice != 0
		}
	}
	return &progress{w: w, total: total, tty: tty}
}

func (p *progress) add(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done += n
	pct := int(p.done * 100 / p.total)
	if pct == p.last {
		return
	}
	p.last = pct
	if p.tty {
		fmt.Fprintf(p.w, "\rUploading: %s / %s (%d%%)", humanSize(p.done), humanSize(p.total), pct)
	} else {
		fmt.Fprintf(p.w, "Uploading: %s / %s (%d%%)\n", humanSize(p.done), humanSize(p.total), pct)
	}
}

// warn 打印警告并先换行，避免与 \r 进度行粘连。
func (p *progress) warn(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tty {
		fmt.Fprintln(p.w)
	}
	fmt.Fprintln(p.w, "warn:", fmt.Sprintf(format, args...))
}

// finish 结束 \r 进度行（TTY 下补一个换行）。
func (p *progress) finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tty {
		fmt.Fprintln(p.w)
	}
}

// runMultipartUpload 执行完整的分片上传流程：
// init -> 并发分片直传 -> complete，并负责断点状态的创建/续传/清理。
func runMultipartUpload(cmd *cobra.Command, localPath string, info os.FileInfo, opts multipartOptions) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")

	// --abort：放弃已中断的上传并清理状态文件
	if opts.abort {
		return abortMultipartUpload(cmd, localPath, jsonMode, debug)
	}

	// --multipart-size 支持纯字节数或带单位写法（10MB / 5MiB / 10485760）
	partSizeStr, _ := cmd.Flags().GetString("multipart-size")
	partSize, err := parseSize(partSizeStr)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.InvalidArgument("--multipart-size", "invalid part size: "+err.Error()), jsonMode)
	}
	if partSize < minMultipartSize {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.InvalidArgument("--multipart-size",
			fmt.Sprintf("--multipart-size must be at least %s, got %s", humanSize(minMultipartSize), humanSize(partSize))), jsonMode)
	}
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	if concurrency < 1 {
		concurrency = 1
	}

	statePath, err := multipartStatePath(localPath)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.FileIO(localPath, "resolve state path", err), jsonMode)
	}
	store := newStateStore(statePath)

	var state *uploadState
	switch {
	case opts.resume:
		// 断点续传：读取上次中断留下的状态，跳过已完成分片
		st, loadErr := store.load()
		if loadErr != nil {
			if os.IsNotExist(loadErr) {
				return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.New(errs.CategoryValidation, 0, "no resume state found for "+localPath).
					WithHint("Start a fresh upload first.", "tier0 assets upload "+localPath), jsonMode)
			}
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.FileIO(statePath, "read resume state", loadErr), jsonMode)
		}
		abs, _ := filepath.Abs(localPath)
		if st.Size != info.Size() || st.LocalPath != abs || st.PartSize <= 0 {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.New(errs.CategoryValidation, 0, "resume state does not match the local file").
				WithHint("Remove the stale state file and upload again.", "rm "+statePath), jsonMode)
		}
		state = st
	case store.exists():
		// 存在上次中断的 multipart 上传：要求用户显式续传或放弃，避免静默产生孤立上传
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.New(errs.CategoryValidation, 0, "an interrupted multipart upload exists for this file").
			WithHint("Continue it with --resume, or discard it with --abort.", "tier0 assets upload "+localPath+" --resume"), jsonMode)
	default:
		// 新上传：先 init 拿到 fileKey / uploadId / 分片大小
		state, err = multipartInit(cmd.Context(), localPath, info.Size(), opts, partSize, debug)
		if err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
		}
		if err := store.save(state); err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.FileIO(statePath, "write resume state", err), jsonMode)
		}
	}

	// 打开文件，分片读取不整读内存
	f, err := os.Open(localPath)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.FileIO(localPath, "open file", err), jsonMode)
	}
	defer f.Close()

	prog := newProgress(cmd.ErrOrStderr(), state.Size)
	if !jsonMode {
		if opts.resume {
			fmt.Fprintf(cmd.ErrOrStderr(), "Resuming multipart upload: %d/%d parts done\n", len(state.Parts), state.PartCount)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "Multipart upload: %d part(s) of %s, concurrency %d\n", state.PartCount, humanSize(state.PartSize), concurrency)
		}
	}

	provider := newPartURLProvider(cmd.Context(), state, debug)
	if err := uploadParts(cmd.Context(), f, state, store, provider, concurrency, debug, prog); err != nil {
		// 失败保留状态文件，提示 --resume 续传
		prog.finish()
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), uploadFailure(err, localPath), jsonMode)
	}

	// 全部上传成功：complete 合并分片，成功后清理状态文件
	result, err := multipartComplete(cmd.Context(), state, debug)
	prog.finish()
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	_ = os.Remove(statePath)

	if jsonMode {
		fmt.Fprintln(cmd.OutOrStdout(), cmdutil.JSONString(map[string]any{
			"filePath":  result.FilePath,
			"fileUrl":   result.FileURL,
			"sizeBytes": result.SizeBytes,
			"uploadId":  state.UploadID,
			"partCount": state.PartCount,
			"expiresAt": result.ExpiresAt,
		}))
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Uploaded")
		fmt.Fprintf(cmd.OutOrStdout(), "  filePath: %s\n", result.FilePath)
		if result.FileURL != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  fileUrl:  %s\n", result.FileURL)
		}
	}
	return nil
}

// uploadParts 用并发 worker 池上传所有未完成分片；任一失败即取消并返回首个错误。
func uploadParts(ctx context.Context, f *os.File, state *uploadState, store *stateStore,
	provider *partURLProvider, concurrency int, debug bool, prog *progress) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	var wg sync.WaitGroup
	var (
		firstErrMu sync.Mutex
		firstErr   error
	)

	worker := func() {
		defer wg.Done()
		for n := range jobs {
			// 续传场景下已上传分片直接跳过
			if state.isCompleted(n) {
				continue
			}
			etag, err := uploadPartWithRetry(ctx, f, state, provider, n, debug)
			if err != nil {
				firstErrMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				firstErrMu.Unlock()
				cancel()
				return
			}
			// 记录 ETag 并落盘，保证中断后可续传
			state.addPart(n, etag)
			if err := store.save(state); err != nil {
				prog.warn("persist resume state: %v", err)
			}
			prog.add(state.partLen(n))
		}
	}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	// 投递未完成的分片号；出错或取消时停止投递
	go func() {
		defer close(jobs)
		for n := 1; n <= state.PartCount; n++ {
			if state.isCompleted(n) {
				continue
			}
			select {
			case jobs <- n:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()

	firstErrMu.Lock()
	defer firstErrMu.Unlock()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

// uploadPartWithRetry 上传单个分片，失败退避重试；403 视为 URL 过期，重拉后再试。
func uploadPartWithRetry(ctx context.Context, f *os.File, state *uploadState,
	provider *partURLProvider, n int, debug bool) (string, error) {
	offset := (int64(n) - 1) * state.PartSize
	length := state.partLen(n)

	var lastErr error
	for attempt := 1; attempt <= partUploadMaxAttempts; attempt++ {
		url, err := provider.url(n)
		if err != nil {
			return "", err
		}
		etag, err := putPart(ctx, f, url, n, offset, length, debug)
		if err == nil {
			return etag, nil
		}
		lastErr = err
		if !partRetryable(err) {
			return "", err
		}
		if attempt < partUploadMaxAttempts {
			if isPartForbidden(err) {
				provider.invalidate(n)
			}
			time.Sleep(backoff(attempt))
		}
	}
	return "", fmt.Errorf("upload part %d/%d: %w", n, state.PartCount, lastErr)
}

// partHTTPError 是分片直传 PUT 的非 2xx 响应。
type partHTTPError struct {
	Status int
	Body   string
}

func (e *partHTTPError) Error() string {
	return fmt.Sprintf("PUT part failed: status=%d body=%s", e.Status, e.Body)
}

// partRetryable 判断分片上传错误是否值得重试：
// 传输层错误、5xx、408/429 以及 403（URL 过期，重拉后有效）均可重试。
func partRetryable(err error) bool {
	var herr *partHTTPError
	if !errors.As(err, &herr) {
		return true
	}
	switch {
	case herr.Status >= 500:
		return true
	case herr.Status == http.StatusRequestTimeout, herr.Status == http.StatusTooManyRequests:
		return true
	case herr.Status == http.StatusForbidden:
		return true
	default:
		return false
	}
}

func isPartForbidden(err error) bool {
	var herr *partHTTPError
	return errors.As(err, &herr) && herr.Status == http.StatusForbidden
}

// backoff 返回第 attempt 次重试前的等待时长（1s 起步，封顶 8s）。
func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * time.Second
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}

// putPart 通过预签名 URL 直传一个分片，返回响应头中的 ETag。
// 使用 io.NewSectionReader 按偏移切片读取文件，避免整文件读入内存；
// 每个分片限时 60s，超时仅影响当前分片，可续传。
func putPart(ctx context.Context, f *os.File, url string, partNumber int, offset, length int64, debug bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, partUploadTimeout)
	defer cancel()

	body := io.NewSectionReader(f, offset, length)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return "", fmt.Errorf("build part %d request: %w", partNumber, err)
	}
	req.ContentLength = length
	req.Header.Set("Expect", "100-continue")

	if debug {
		fmt.Fprintf(os.Stderr, "[debug] PUT part %d (%d bytes)\n", partNumber, length)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("PUT part %d: %w", partNumber, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyText, _ := io.ReadAll(resp.Body)
		return "", &partHTTPError{Status: resp.StatusCode, Body: string(bodyText)}
	}
	// 读取并丢弃响应体以复用连接；ETag 用于最终 complete
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return "", fmt.Errorf("read part %d response: %w", partNumber, err)
	}
	etag := strings.TrimSpace(resp.Header.Get("ETag"))
	if etag == "" {
		return "", fmt.Errorf("part %d response missing ETag header", partNumber)
	}
	return etag, nil
}

// ── 后端 multipart 接口调用 ───────────────────────────────────────────────────

type multipartInitData struct {
	FileKey   string `json:"fileKey"`
	FilePath  string `json:"filePath"`
	UploadID  string `json:"uploadId"`
	PartSize  int64  `json:"partSize"`
	PartCount int    `json:"partCount"`
	ExpiresAt int64  `json:"expiresAt"`
}

type partURLInfo struct {
	PartNumber int    `json:"partNumber"`
	URL        string `json:"url"`
	ExpiresAt  int64  `json:"expiresAt"`
}

type multipartCompleteData struct {
	FilePath  string `json:"filePath"`
	FileURL   string `json:"fileUrl"`
	SizeBytes int64  `json:"sizeBytes"`
	ExpiresAt int64  `json:"expiresAt"`
}

// multipartCall 调用 multipart 子接口并解析统一 ResultVO 信封。
func multipartCall(ctx context.Context, subpath string, body any, debug bool, out any) error {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return errs.New(errs.CategoryInternal, 0, "marshal multipart request: "+err.Error())
	}
	resp, err := cmdutil.DoAPI(ctx, "/openapi/v1/assets/files/multipart/"+subpath, "POST", string(bodyJSON), debug)
	if err != nil {
		return err
	}
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp), &envelope); err != nil {
		return errs.New(errs.CategoryInternal, 0, "parse multipart response: "+err.Error())
	}
	if envelope.Code != 0 && envelope.Code != 200 {
		return apierr.NewBusiness(envelope.Code, resp)
	}
	if out != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return errs.New(errs.CategoryInternal, 0, "parse multipart response data: "+err.Error())
		}
	}
	return nil
}

// multipartInit 请求创建 multipart 上传，返回用于后续调用的状态。
func multipartInit(ctx context.Context, localPath string, size int64, opts multipartOptions, partSize int64, debug bool) (*uploadState, error) {
	fileName := filepath.Base(localPath)
	contentType := mimeTypeByExtension(filepath.Ext(fileName))
	body := map[string]any{
		"fileName":      fileName,
		"contentType":   contentType,
		"size":          size,
		"partSize":      partSize,
		"business":      opts.business,
		"useBy":         opts.useBy,
		"visibility":    opts.visibility,
		"appInstanceId": opts.appInstanceID,
		"sessionId":     opts.sessionID,
	}
	var data multipartInitData
	if err := multipartCall(ctx, "init", body, debug, &data); err != nil {
		return nil, err
	}
	if data.FileKey == "" || data.UploadID == "" {
		return nil, errs.New(errs.CategoryAPI, 0, "invalid multipart init response: missing fileKey or uploadId")
	}
	// 服务端可能按其限制调整 partSize / partCount，客户端以服务端为准
	effPartSize := partSize
	if data.PartSize > 0 {
		effPartSize = data.PartSize
	}
	partCount := partCountFor(size, effPartSize)
	if data.PartSize <= 0 && data.PartCount > 0 {
		partCount = data.PartCount
	}
	abs, _ := filepath.Abs(localPath)
	return &uploadState{
		LocalPath: abs,
		FileName:  fileName,
		Size:      size,
		FileKey:   data.FileKey,
		UploadID:  data.UploadID,
		PartSize:  effPartSize,
		PartCount: partCount,
		Parts:     map[int]string{},
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// partCountFor 计算分片总数：ceil(size/partSize)。
func partCountFor(size, partSize int64) int {
	if partSize <= 0 {
		return 0
	}
	return int((size + partSize - 1) / partSize)
}

// multipartPartURLs 批量拉取指定分片的预签名直传 URL。
func multipartPartURLs(ctx context.Context, fileKey, uploadID string, partNumbers []int, debug bool) ([]partURLInfo, error) {
	var data struct {
		PartURLs []partURLInfo `json:"partUrls"`
	}
	if err := multipartCall(ctx, "part-urls", map[string]any{
		"fileKey":     fileKey,
		"uploadId":    uploadID,
		"partNumbers": partNumbers,
	}, debug, &data); err != nil {
		return nil, err
	}
	return data.PartURLs, nil
}

// multipartComplete 携带全部分片的 ETag 合并上传，返回最终文件信息。
func multipartComplete(ctx context.Context, state *uploadState, debug bool) (*multipartCompleteData, error) {
	// 按分片号升序组织 parts 列表
	state.mu.Lock()
	nums := make([]int, 0, len(state.Parts))
	for n := range state.Parts {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	parts := make([]map[string]any, 0, len(nums))
	for _, n := range nums {
		parts = append(parts, map[string]any{"partNumber": n, "etag": state.Parts[n]})
	}
	state.mu.Unlock()

	if len(parts) != state.PartCount {
		return nil, errs.New(errs.CategoryInternal, 0,
			fmt.Sprintf("incomplete multipart upload: %d/%d parts uploaded", len(parts), state.PartCount))
	}
	var data multipartCompleteData
	if err := multipartCall(ctx, "complete", map[string]any{
		"fileKey":  state.FileKey,
		"uploadId": state.UploadID,
		"parts":    parts,
	}, debug, &data); err != nil {
		return nil, err
	}
	if data.FilePath == "" {
		return nil, errs.New(errs.CategoryAPI, 0, "invalid multipart complete response: missing filePath")
	}
	return &data, nil
}

// abortMultipartUpload 放弃已中断的 multipart 上传并清理本地状态文件。
func abortMultipartUpload(cmd *cobra.Command, localPath string, jsonMode, debug bool) error {
	statePath, err := multipartStatePath(localPath)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.FileIO(localPath, "resolve state path", err), jsonMode)
	}
	store := newStateStore(statePath)
	state, err := store.load()
	if err != nil {
		if os.IsNotExist(err) {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.New(errs.CategoryValidation, 0, "no interrupted multipart upload state found for "+localPath).
				WithHint("Nothing to abort; start a fresh upload first.", "tier0 assets upload "+localPath), jsonMode)
		}
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), errs.FileIO(statePath, "read resume state", err), jsonMode)
	}

	var data struct {
		Aborted bool `json:"aborted"`
	}
	if err := multipartCall(cmd.Context(), "abort", map[string]any{
		"fileKey":  state.FileKey,
		"uploadId": state.UploadID,
	}, debug, &data); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	// abort 成功后清理本地状态文件
	_ = os.Remove(statePath)

	if jsonMode {
		fmt.Fprintln(cmd.OutOrStdout(), cmdutil.JSONString(map[string]any{
			"aborted":   data.Aborted,
			"uploadId":  state.UploadID,
			"localPath": localPath,
		}))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Aborted multipart upload (uploadId %s)\n", state.UploadID)
	}
	return nil
}

// uploadFailure 为分片上传中断补充续传提示。
func uploadFailure(err error, localPath string) error {
	const hint = "Upload interrupted. Re-run with --resume to continue from completed parts; lower --concurrency if timeouts persist."
	cmdStr := "tier0 assets upload " + localPath + " --resume"

	var ce *errs.CLIError
	if errors.As(err, &ce) {
		return ce.WithHint(hint, cmdStr)
	}
	return errs.New(errs.CategoryAPI, 0, err.Error()).
		WithCause(err).
		WithHint(hint, cmdStr)
}

// ── 工具函数 ───────────────────────────────────────────────────────────────────

// multipartStatePath 返回与本地文件同目录的断点状态文件路径，
// 文件名含绝对路径 hash，同一文件多次运行共用同一状态。
func multipartStatePath(localPath string) (string, error) {
	abs, err := filepath.Abs(localPath)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(abs))
	h := hex.EncodeToString(sum[:])[:stateHashLen]
	return filepath.Join(filepath.Dir(abs), ".tier0-upload-"+h+".json"), nil
}

// multipartStateExists 判断本地文件是否存在断点状态文件。
func multipartStateExists(localPath string) bool {
	p, err := multipartStatePath(localPath)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// parseSize 解析字节数，支持纯数字或带单位写法（B/KB/MB/GB 与 KIB/MIB/GIB，大小写不敏感）。
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("missing numeric value in %q", s)
	}
	num, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", s[:i])
	}
	unit := strings.ToUpper(strings.TrimSpace(s[i:]))
	var mult int64
	switch unit {
	case "", "B":
		mult = 1
	case "K", "KB", "KIB":
		mult = 1 << 10
	case "M", "MB", "MIB":
		mult = 1 << 20
	case "G", "GB", "GIB":
		mult = 1 << 30
	default:
		return 0, fmt.Errorf("unknown unit %q (use B/KB/MB/GB or KIB/MIB/GIB)", unit)
	}
	if num > math.MaxInt64/mult {
		return 0, fmt.Errorf("size %q overflows int64", s)
	}
	return num * mult, nil
}

// humanSize 将字节数格式化为可读的容量表示。
func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
