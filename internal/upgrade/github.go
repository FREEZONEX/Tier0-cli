package upgrade

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
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

// IsNewer reports whether latest is a higher semver than current.
// "dev" builds are never considered outdated.
// Falls back to string inequality if either version is not valid semver.
func IsNewer(current, latest string) bool {
	if strings.TrimPrefix(current, "v") == "dev" {
		return false
	}
	c := parseSemver(current)
	l := parseSemver(latest)
	if c == nil || l == nil {
		// Non-semver fallback: just compare trimmed strings.
		return strings.TrimPrefix(current, "v") != strings.TrimPrefix(latest, "v")
	}
	for i := 0; i < 3; i++ {
		if l[i] > c[i] {
			return true
		}
		if l[i] < c[i] {
			return false
		}
	}
	return false
}

// parseSemver parses a version string like "v1.2.3" or "1.2.3" into [major, minor, patch].
// Returns nil if the string is not a valid semver.
func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release / build metadata (e.g. "1.2.3-beta.1+build")
	v = strings.SplitN(v, "-", 2)[0]
	v = strings.SplitN(v, "+", 2)[0]
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return nums
}
