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
	GitHubAPI      = "https://api.github.com"
	RepoOwner      = "FREEZONEX"
	RepoName       = "Tier0-cli"
	NPMRegistry    = "https://registry.npmjs.org"
	NPMMirror      = "https://registry.npmmirror.com"
	NPMPackage     = "@tier0/cli"
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

// fetchLatestVersionNPM queries the npm registry for the latest published version.
// Tries npmjs.org first, falls back to npmmirror.com for users in China.
func fetchLatestVersionNPM() (string, error) {
	for _, registry := range []string{NPMRegistry, NPMMirror} {
		ver, err := fetchVersionFromRegistry(registry)
		if err == nil {
			return ver, nil
		}
	}
	return "", fmt.Errorf("无法从 npm registry 或 npmmirror 获取版本信息")
}

func fetchVersionFromRegistry(registry string) (string, error) {
	url := fmt.Sprintf("%s/%s/latest", registry, NPMPackage)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return "", err
	}
	if pkg.Version == "" {
		return "", fmt.Errorf("empty version")
	}
	return pkg.Version, nil
}

// buildReleaseFromVersion constructs a Release with a pre-computed download URL
// without calling the GitHub API. The URL pattern matches what release.sh uploads.
func buildReleaseFromVersion(ver string) *Release {
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}
	platform := platformReleaseName()
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	assetName := fmt.Sprintf("tier0-cli-%s-%s%s", ver, platform, ext)
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		RepoOwner, RepoName, ver, assetName,
	)
	return &Release{
		TagName: ver,
		HTMLURL: fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", RepoOwner, RepoName, ver),
		Assets: []Asset{{
			Name:               assetName,
			BrowserDownloadURL: downloadURL,
		}},
	}
}

// FetchLatestRelease 获取最新 Release。
// 优先查询 npm registry（无限速），失败时回退到 GitHub API。
func FetchLatestRelease() (*Release, error) {
	// Primary: npm registry — no rate limit
	if ver, err := fetchLatestVersionNPM(); err == nil {
		return buildReleaseFromVersion(ver), nil
	}

	// Fallback: GitHub API
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

// FetchRelease 获取指定版本的 Release。
// 直接构造下载 URL，无需调用 GitHub API。
func FetchRelease(ver string) (*Release, error) {
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}
	// Build the release directly from the known version — no API call needed.
	release := buildReleaseFromVersion(ver)

	// Verify the asset actually exists with a HEAD request (avoids downloading a 404).
	asset := release.FindAsset()
	if asset == nil {
		return nil, fmt.Errorf("无法构建版本 %s 的下载地址", ver)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	headResp, err := client.Head(asset.BrowserDownloadURL)
	if err != nil {
		// Network error — return the release anyway and let the download step fail with a clear error.
		return release, nil
	}
	defer headResp.Body.Close()
	if headResp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("版本 %s 不存在（资源文件未找到）", ver)
	}
	return release, nil
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
