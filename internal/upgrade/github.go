package upgrade

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	GitHubAPI = "https://api.github.com"
	RepoOwner = "FREEZONEX"
	RepoName  = "Tier0-cli"
)

// Release 表示 GitHub Release 信息
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []Asset   `json:"assets"`
}

// Asset 表示 Release 中的资源文件
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// platformKey 返回当前平台的标识字符串，如 "linux-amd64"
func platformKey() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// platformReleaseName 返回 release 包中平台对应的友好名称
func platformReleaseName() string {
	switch platformKey() {
	case "linux-amd64":
		return "Linux-x86_64"
	case "linux-arm64":
		return "Linux-aarch64"
	case "darwin-amd64":
		return "macOS-x86_64"
	case "darwin-arm64":
		return "macOS-arm64"
	case "windows-amd64":
		return "Windows-x86_64"
	case "windows-arm64":
		return "Windows-arm64"
	default:
		return runtime.GOOS + "-" + runtime.GOARCH
	}
}

// FetchLatestRelease 从 GitHub API 获取最新 Release
func FetchLatestRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", GitHubAPI, RepoOwner, RepoName)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("无法连接 GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回状态码 %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析 Release 信息失败: %w", err)
	}

	return &release, nil
}

// FetchRelease 获取指定版本的 Release
func FetchRelease(version string) (*Release, error) {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", GitHubAPI, RepoOwner, RepoName, version)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("无法连接 GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("版本 %s 不存在", version)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回状态码 %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析 Release 信息失败: %w", err)
	}

	return &release, nil
}

// FindAsset 在 Release 中查找匹配当前平台的资源文件
func (r *Release) FindAsset() *Asset {
	platformName := platformReleaseName()
	suffix := ".tar.gz"
	if runtime.GOOS == "windows" {
		suffix = ".zip"
	}

	expectedSuffix := platformName + suffix
	for i := range r.Assets {
		if strings.Contains(r.Assets[i].Name, expectedSuffix) {
			return &r.Assets[i]
		}
	}

	// 备选：按 GOOS-GOARCH 格式匹配
	fallbackSuffix := platformKey() + suffix
	for i := range r.Assets {
		if strings.Contains(r.Assets[i].Name, fallbackSuffix) {
			return &r.Assets[i]
		}
	}

	return nil
}

// IsNewer 判断 release 版本是否比当前版本新
func IsNewer(current, latest string) bool {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")
	return current != latest && current != "dev"
}
