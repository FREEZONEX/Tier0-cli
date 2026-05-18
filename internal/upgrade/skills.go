package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SkillInfo 单个 skill 的信息
type SkillInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// SkillsListResult 列出 skills 的结果
type SkillsListResult struct {
	Version string      `json:"version"`
	Skills  []SkillInfo `json:"skills"`
}

// SkillsUpdateResult skills 更新的结果
type SkillsUpdateResult struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpToDate       bool   `json:"upToDate"`
	UpdatedCount   int    `json:"updatedCount,omitempty"`
	ErrorMessage   string `json:"error,omitempty"`
}

// FindSkillsDir 查找本地 skills 目录
func FindSkillsDir(binaryPath string) string {
	// 优先：二进制同目录下的 skill/
	if binaryPath != "" {
		dir := filepath.Join(filepath.Dir(binaryPath), "skill")
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	// 备选：~/.tier0/skills/
	home, err := os.UserHomeDir()
	if err == nil {
		dir := filepath.Join(home, ".tier0", "skills")
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	return ""
}

// GetSkillsVersion 读取 skills 的版本信息
func GetSkillsVersion(skillsDir string) string {
	if skillsDir == "" {
		return "unknown"
	}
	metaFile := filepath.Join(skillsDir, "_meta.json")
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return getSkillsVersionFromSubdirs(skillsDir)
	}
	var meta struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &meta) == nil && meta.Version != "" {
		return meta.Version
	}
	return getSkillsVersionFromSubdirs(skillsDir)
}

func getSkillsVersionFromSubdirs(skillsDir string) string {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return "unknown"
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaFile := filepath.Join(skillsDir, entry.Name(), "_meta.json")
		data, err := os.ReadFile(metaFile)
		if err != nil {
			continue
		}
		var meta struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &meta) == nil && meta.Version != "" {
			return meta.Version
		}
	}
	return "unknown"
}

// ListSkills 列出所有已安装的 skills
func ListSkills(skillsDir string) (*SkillsListResult, error) {
	if skillsDir == "" {
		return &SkillsListResult{Version: "dev", Skills: nil}, nil
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &SkillsListResult{Version: "unknown", Skills: nil}, nil
		}
		return nil, fmt.Errorf("读取 skills 目录失败: %w", err)
	}

	var skills []SkillInfo
	version := GetSkillsVersion(skillsDir)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue
		}

		info := SkillInfo{
			Name:    entry.Name(),
			Version: version,
		}

		if desc := readSkillDescription(skillFile); desc != "" {
			info.Description = desc
		}

		metaFile := filepath.Join(skillsDir, entry.Name(), "_meta.json")
		if data, err := os.ReadFile(metaFile); err == nil {
			var meta struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(data, &meta) == nil && meta.Version != "" {
				info.Version = meta.Version
			}
		}

		skills = append(skills, info)
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return &SkillsListResult{
		Version: version,
		Skills:  skills,
	}, nil
}

// readSkillDescription 从 SKILL.md 的 frontmatter 中读取 description 字段
func readSkillDescription(skillFile string) string {
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return ""
	}
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		return ""
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return ""
	}
	frontmatter := content[4 : 4+end]

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			desc := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			desc = strings.Trim(desc, "\"'")
			if len(desc) > 80 {
				desc = desc[:77] + "..."
			}
			return desc
		}
	}
	return ""
}

// CheckSkillsUpdate 检查 skills 是否有新版本
func CheckSkillsUpdate(skillsDir string) (*SkillsUpdateResult, error) {
	currentVer := GetSkillsVersion(skillsDir)

	latestRelease, err := FetchLatestRelease()
	if err != nil {
		return &SkillsUpdateResult{
			CurrentVersion: currentVer,
			ErrorMessage:   err.Error(),
		}, nil
	}

	upToDate := !IsNewer(currentVer, latestRelease.TagName)

	return &SkillsUpdateResult{
		CurrentVersion: currentVer,
		LatestVersion:  latestRelease.TagName,
		UpToDate:       upToDate,
	}, nil
}

// UpdateSkills 更新 skills 到最新版本
func UpdateSkills(skillsDir string, dryRun bool) (*SkillsUpdateResult, error) {
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		if dryRun {
			return &SkillsUpdateResult{CurrentVersion: "未安装"}, nil
		}
		if err := os.MkdirAll(skillsDir, 0o755); err != nil {
			return nil, fmt.Errorf("创建 skills 目录失败: %w", err)
		}
	}

	result, err := CheckSkillsUpdate(skillsDir)
	if err != nil {
		return result, err
	}

	if result.UpToDate {
		return result, nil
	}

	if dryRun {
		return result, nil
	}

	latestRelease, err := FetchLatestRelease()
	if err != nil {
		result.ErrorMessage = err.Error()
		return result, err
	}

	asset := latestRelease.FindAsset()
	if asset == nil {
		err := fmt.Errorf("未找到适配当前平台的 skills 包")
		result.ErrorMessage = err.Error()
		return result, err
	}

	tmpDir, err := os.MkdirTemp("", "tier0-skills-update-*")
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("创建临时目录失败: %v", err)
		return result, err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := downloadFile(asset.BrowserDownloadURL, archivePath); err != nil {
		result.ErrorMessage = fmt.Sprintf("下载 skills 包失败: %v", err)
		return result, err
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		result.ErrorMessage = fmt.Sprintf("创建解压目录失败: %v", err)
		return result, err
	}

	if err := extract(archivePath, extractDir); err != nil {
		result.ErrorMessage = fmt.Sprintf("解压 skills 包失败: %v", err)
		return result, err
	}

	srcSkillsDir := findSkillDir(extractDir)
	if srcSkillsDir == "" {
		result.ErrorMessage = "未在安装包中找到 skills 目录"
		return result, err
	}

	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.Rename(skillsDir, backupDir); err != nil {
		result.ErrorMessage = fmt.Sprintf("备份旧 skills 失败: %v", err)
		return result, err
	}

	if err := copyDir(srcSkillsDir, skillsDir); err != nil {
		os.RemoveAll(skillsDir)
		os.Rename(backupDir, skillsDir)
		result.ErrorMessage = fmt.Sprintf("安装新 skills 失败（已回滚）: %v", err)
		return result, err
	}

	entries, _ := os.ReadDir(skillsDir)
	for _, e := range entries {
		if e.IsDir() {
			result.UpdatedCount++
		}
	}

	return result, nil
}

// findSkillDir 在解压目录中查找 skill/ 目录
func findSkillDir(dir string) string {
	skillDir := filepath.Join(dir, "skill")
	if fi, err := os.Stat(skillDir); err == nil && fi.IsDir() {
		return skillDir
	}
	var found string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if info.IsDir() && info.Name() == "skill" {
			if fi, err := os.Stat(filepath.Join(path, "SKILL.md")); err == nil && !fi.IsDir() {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		dstFile, err := os.Create(target)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err == nil {
			os.Chtimes(target, info.ModTime(), info.ModTime())
		}
		return err
	})
}

// GetDefaultSkillsDir 获取默认的 skills 目录路径
func GetDefaultSkillsDir() string {
	binaryPath, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := FindSkillsDir(binaryPath)
	if dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".tier0", "skills")
	}
	return ""
}

// SkillsLastUpdated 获取 skills 的最后更新时间
func SkillsLastUpdated(skillsDir string) string {
	if skillsDir == "" {
		return ""
	}
	metaFile := filepath.Join(skillsDir, "_meta.json")
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return ""
	}
	var meta struct {
		UpdatedAt string `json:"updatedAt"`
	}
	if json.Unmarshal(data, &meta) == nil {
		return meta.UpdatedAt
	}
	skillFile := filepath.Join(skillsDir, "SKILL.md")
	if fi, err := os.Stat(skillFile); err == nil {
		return fi.ModTime().Format(time.RFC3339)
	}
	return ""
}
