package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

// Options 升级选项
type Options struct {
	TargetVersion string // 指定升级到的版本（为空则升级到最新）
	DryRun        bool   // 只检查不安装
}

// Result 升级结果
type Result struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpToDate       bool   `json:"upToDate"`
	DownloadURL    string `json:"downloadUrl,omitempty"`
	ErrorMessage   string `json:"error,omitempty"`
	// Method indicates how the upgrade was performed: "npm" or "github"
	Method string `json:"method,omitempty"`
}

// Check reports whether a newer version is available.
// It consults a local cache (~/.tier0/update-state.json, TTL 24 h) before
// hitting the GitHub API, matching lark-cli's approach.
func Check() (*Result, error) {
	// 1. Try the cache first.
	cached, _ := loadCachedState()
	if isCacheValid(cached) {
		upToDate := !IsNewer(version.BuildVersion, cached.LatestVersion)
		result := &Result{
			CurrentVersion: version.BuildVersion,
			LatestVersion:  cached.LatestVersion,
			UpToDate:       upToDate,
		}
		return result, nil
	}

	// 2. Cache miss or expired — fetch from GitHub.
	latestRelease, err := FetchLatestRelease()
	if err != nil {
		// On network failure, return what the (stale) cache says rather than
		// surfacing a noisy error to the user.
		if cached != nil && cached.LatestVersion != "" {
			upToDate := !IsNewer(version.BuildVersion, cached.LatestVersion)
			return &Result{
				CurrentVersion: version.BuildVersion,
				LatestVersion:  cached.LatestVersion,
				UpToDate:       upToDate,
			}, nil
		}
		return &Result{
			CurrentVersion: version.BuildVersion,
			ErrorMessage:   err.Error(),
		}, nil
	}

	// 3. Persist the fresh result.
	saveCachedState(&updateState{
		LatestVersion: latestRelease.TagName,
		CheckedAt:     time.Now().Unix(),
	})

	upToDate := !IsNewer(version.BuildVersion, latestRelease.TagName)
	result := &Result{
		CurrentVersion: version.BuildVersion,
		LatestVersion:  latestRelease.TagName,
		UpToDate:       upToDate,
	}
	if !upToDate {
		result.DownloadURL = latestRelease.HTMLURL
	}
	return result, nil
}

// Perform 执行升级。
//
// 策略（与 lark-cli 一致）：
//  1. 若 npm 可用 → npm install -g @tier0/cli@<version>（postinstall 负责下二进制）
//  2. npm 不可用 → 直接从 GitHub Releases 下载二进制并替换
func Perform(opts Options) (*Result, error) {
	var release *Release
	var err error
	if opts.TargetVersion != "" {
		release, err = FetchRelease(opts.TargetVersion)
	} else {
		release, err = FetchLatestRelease()
	}
	if err != nil {
		return &Result{CurrentVersion: version.BuildVersion, ErrorMessage: err.Error()}, err
	}

	if opts.TargetVersion == "" && !IsNewer(version.BuildVersion, release.TagName) {
		saveCachedState(&updateState{
			LatestVersion: release.TagName,
			CheckedAt:     time.Now().Unix(),
		})
		return &Result{
			CurrentVersion: version.BuildVersion,
			LatestVersion:  release.TagName,
			UpToDate:       true,
		}, nil
	}

	asset := release.FindAsset()
	if asset == nil {
		e := fmt.Errorf("未找到适配当前平台 (%s) 的安装包", platformReleaseName())
		return &Result{
			CurrentVersion: version.BuildVersion,
			LatestVersion:  release.TagName,
			ErrorMessage:   e.Error(),
		}, e
	}

	result := &Result{
		CurrentVersion: version.BuildVersion,
		LatestVersion:  release.TagName,
		DownloadURL:    asset.BrowserDownloadURL,
	}

	if opts.DryRun {
		return result, nil
	}

	// ── Path 1: npm available → delegate to npm install ──────────────────
	if NpmAvailable() {
		npmResult := RunNpmInstall(release.TagName)
		if npmResult.Err != nil {
			// npm failed; fall through to direct download below
			result.ErrorMessage = fmt.Sprintf("npm install 失败，尝试直接下载: %v", npmResult.Err)
		} else {
			result.Method = "npm"
			return result, nil
		}
	}

	// ── Path 2: direct GitHub download ───────────────────────────────────
	// 安全提示：npm 路径的 JS wrapper 经 npm registry 校验，推荐优先使用。
	// 直接下载路径会验证 SHA256 checksum，但建议安装 Node.js 以使用 npm 路径。
	result.Method = "github"
	binaryPath, err := os.Executable()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("无法获取可执行文件路径: %v", err)
		return result, err
	}
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("无法解析可执行文件路径: %v", err)
		return result, err
	}

	if err := backupBinary(binaryPath, version.BuildVersion); err != nil {
		result.ErrorMessage = fmt.Sprintf("备份失败: %v", err)
		return result, err
	}

	tmpDir, err := os.MkdirTemp("", "tier0-upgrade-*")
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("创建临时目录失败: %v", err)
		return result, err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := downloadFile(asset.BrowserDownloadURL, archivePath); err != nil {
		result.ErrorMessage = fmt.Sprintf("下载失败: %v", err)
		return result, err
	}

	// 下载并验证 SHA256 checksum，防止传输被篡改或文件损坏
	if err := verifyReleaseChecksum(release.TagName, asset.Name, archivePath); err != nil {
		result.ErrorMessage = fmt.Sprintf("SHA256 校验失败: %v", err)
		return result, err
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		result.ErrorMessage = fmt.Sprintf("创建解压目录失败: %v", err)
		return result, err
	}

	if err := extract(archivePath, extractDir); err != nil {
		result.ErrorMessage = fmt.Sprintf("解压失败: %v", err)
		return result, err
	}

	newBinary, err := findBinary(extractDir)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("查找新版本二进制失败: %v", err)
		return result, err
	}

	if err := replaceBinary(newBinary, binaryPath); err != nil {
		result.ErrorMessage = fmt.Sprintf("替换二进制失败: %v（旧版本已备份到 ~/.tier0/backup/）", err)
		return result, err
	}

	return result, nil
}

// verifyReleaseChecksum 从 GitHub Release 下载 sha256sums.txt，
// 找到与 assetName 对应的期望值，并与本地文件的实际 SHA256 比对。
// 校验失败直接返回错误，阻止安装继续进行。
func verifyReleaseChecksum(version, assetName, localPath string) error {
	sumsURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/sha256sums.txt",
		RepoOwner, RepoName, version,
	)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(sumsURL)
	if err != nil {
		return fmt.Errorf("下载 sha256sums.txt 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sha256sums.txt 返回 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("读取 sha256sums.txt 失败: %w", err)
	}

	expected, err := parseChecksum(string(body), assetName)
	if err != nil {
		return err
	}

	actual, err := sha256File(localPath)
	if err != nil {
		return fmt.Errorf("计算本地文件 SHA256 失败: %w", err)
	}

	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("SHA256 不匹配（可能下载损坏或被篡改）:\n  期望: %s\n  实际: %s", expected, actual)
	}
	return nil
}

// parseChecksum 从 sha256sum(1) 格式的文本中提取指定文件名对应的 SHA256。
// 格式为：<hash>  <filename>（两个空格分隔）
func parseChecksum(sumsText, filename string) (string, error) {
	for _, line := range strings.Split(sumsText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// sha256sum 格式：hash  filename（普通文件）或 hash *filename（二进制模式）
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimPrefix(parts[1], "*")
		if name == filename || filepath.Base(name) == filename {
			return strings.ToLower(parts[0]), nil
		}
	}
	return "", fmt.Errorf("sha256sums.txt 中未找到文件 %q 的校验和", filename)
}

// sha256File 计算文件的 SHA256 十六进制字符串。
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BackupDir 返回备份目录
func BackupDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".tier0", "backup")
	}
	return filepath.Join(home, ".tier0", "backup")
}

func backupBinary(binaryPath, oldVersion string) error {
	dir := BackupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}

	backupName := filepath.Base(binaryPath) + "-" + oldVersion
	if runtime.GOOS == "windows" {
		backupName += ".exe"
	}
	backupPath := filepath.Join(dir, backupName)

	src, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("读取当前二进制失败: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("创建备份文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("写入备份文件失败: %w", err)
	}

	if runtime.GOOS != "windows" {
		os.Chmod(backupPath, 0o755)
	}

	return nil
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回状态码 %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("创建下载文件失败: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("写入下载文件失败: %w", err)
	}

	return nil
}

func extract(archivePath, destDir string) error {
	if strings.HasSuffix(archivePath, ".tar.gz") {
		return extractTarGz(archivePath, destDir)
	}
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, destDir)
	}
	return fmt.Errorf("不支持的压缩格式: %s", filepath.Base(archivePath))
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0o755)
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
			os.Chmod(target, os.FileMode(header.Mode))
		}
	}
	return nil
}

func extractZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		target := filepath.Join(destDir, file.Name)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}

		os.MkdirAll(filepath.Dir(target), 0o755)
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.Create(target)
		if err != nil {
			src.Close()
			return err
		}
		_, err = io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func findBinary(dir string) (string, error) {
	var found string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if runtime.GOOS == "windows" {
			if name == "tier0.exe" {
				found = path
				return filepath.SkipAll
			}
		} else {
			if name == "tier0" {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("未在安装包中找到 tier0 二进制文件")
	}
	return found, nil
}

func replaceBinary(newPath, oldPath string) error {
	if runtime.GOOS == "windows" {
		oldBak := oldPath + ".old"
		// 清理可能存在的旧备份，否则 rename 会失败
		os.Remove(oldBak)
		if err := os.Rename(oldPath, oldBak); err != nil {
			return fmt.Errorf("备份旧版本失败: %w", err)
		}
		if err := os.Rename(newPath, oldPath); err != nil {
			// 回滚
			os.Rename(oldBak, oldPath)
			return fmt.Errorf("替换新版本失败: %w", err)
		}
		// Windows 下正在运行的旧文件可能无法立即删除，忽略该错误
		os.Remove(oldBak)
		return nil
	}

	if err := os.Chmod(newPath, 0o755); err != nil {
		return err
	}

	if err := os.Rename(newPath, oldPath); err != nil {
		return err
	}

	// macOS: 从网络下载的二进制经 rename 替换后，AMFI/Gatekeeper 可能
	// 因签名状态不干净而 SIGKILL 新进程（exit:137）。
	// 用 ad-hoc 签名修复，并移除 quarantine 扩展属性。
	if runtime.GOOS == "darwin" {
		fixMacOSSigning(oldPath)
	}

	return nil
}

// fixMacOSSigning 对 macOS 上刚替换的二进制施加 ad-hoc 签名并移除
// quarantine 标记，防止 AMFI/Gatekeeper 在下次执行时 SIGKILL 进程。
// 两步操作均为 best-effort：codesign 或 xattr 不存在时静默跳过，
// 不应因此阻断升级流程。
func fixMacOSSigning(binaryPath string) {
	// Step 1: 移除 quarantine 扩展属性（从 URL 下载的文件会被打标）
	// xattr -d com.apple.quarantine <path> — 不存在属性时会返回非零但无害
	execQuiet("xattr", "-d", "com.apple.quarantine", binaryPath)

	// Step 2: ad-hoc 签名（-s - 表示 ad-hoc identity，-f 强制覆盖已有签名）
	// 这是没有 Apple 开发者证书时的标准做法，与 Homebrew 的 brew reinstall 逻辑一致。
	execQuiet("codesign", "-f", "-s", "-", binaryPath)
}

// execQuiet 运行外部命令，忽略所有输出和错误。
func execQuiet(name string, args ...string) {
	cmd := exec.Command(name, args...)
	_ = cmd.Run()
}
