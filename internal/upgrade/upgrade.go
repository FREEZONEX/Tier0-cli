package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
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

// Perform 执行升级
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
		// Update cache so background notice also sees the current latest.
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
		err := fmt.Errorf("未找到适配当前平台 (%s) 的安装包", platformReleaseName())
		return &Result{
			CurrentVersion: version.BuildVersion,
			LatestVersion:  release.TagName,
			ErrorMessage:   err.Error(),
		}, err
	}

	result := &Result{
		CurrentVersion: version.BuildVersion,
		LatestVersion:  release.TagName,
		DownloadURL:    asset.BrowserDownloadURL,
	}

	if opts.DryRun {
		return result, nil
	}

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

	return nil
}
