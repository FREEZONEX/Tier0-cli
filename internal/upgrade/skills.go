package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// httpGetJSON performs a GET request and returns the response body as a string.
func httpGetJSON(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}
	return string(body), nil
}

// SkillInfo describes one installed skill.
type SkillInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// SkillsListResult is the list-skills result.
type SkillsListResult struct {
	Version string      `json:"version"`
	Skills  []SkillInfo `json:"skills"`
}

// SkillsUpdateResult is the skills update result.
type SkillsUpdateResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpToDate        bool   `json:"upToDate"`
	UpdatedCount    int    `json:"updatedCount,omitempty"`
	ErrorMessage    string `json:"error,omitempty"`
	AgentSyncStatus string `json:"agentSyncStatus,omitempty"`
	AgentSyncError  string `json:"agentSyncError,omitempty"`
}

// FindSkillsDir locates the local skills directory.
func FindSkillsDir(binaryPath string) string {
	// Prefer skill/ beside the binary.
	if binaryPath != "" {
		dir := filepath.Join(filepath.Dir(binaryPath), "skill")
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	// Fallback: ~/.tier0/skills/.
	home, err := os.UserHomeDir()
	if err == nil {
		dir := filepath.Join(home, ".tier0", "skills")
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	return ""
}

// GetSkillsVersion reads skills version metadata.
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

// ListSkills lists all installed skills.
func ListSkills(skillsDir string) (*SkillsListResult, error) {
	if skillsDir == "" {
		return &SkillsListResult{Version: "dev", Skills: nil}, nil
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &SkillsListResult{Version: "unknown", Skills: nil}, nil
		}
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
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

// readSkillDescription reads the description field from SKILL.md frontmatter.
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

// skillRepoLatestCommit fetches the latest commit SHA of the Tier0-skill
// main branch from the GitHub API. The SHA is used as a lightweight
// "version" so we can detect when the skill repo has been updated even
// without a formal release tag.
func skillRepoLatestCommit() (string, error) {
	type ghCommit struct {
		SHA string `json:"sha"`
	}
	url := "https://api.github.com/repos/FREEZONEX/Tier0-skill/commits/main"
	resp, err := httpGetJSON(url)
	if err != nil {
		return "", err
	}
	var c ghCommit
	if err := json.Unmarshal([]byte(resp), &c); err != nil || c.SHA == "" {
		return "", fmt.Errorf("failed to parse skill repo commit: %s", resp)
	}
	return c.SHA[:8], nil // short SHA, 8 chars
}

// CheckSkillsUpdate checks for newer skills by comparing the latest Tier0-skill commit.
func CheckSkillsUpdate(skillsDir string) (*SkillsUpdateResult, error) {
	currentVer := GetSkillsVersion(skillsDir)

	latestSHA, err := skillRepoLatestCommit()
	if err != nil {
		return &SkillsUpdateResult{
			CurrentVersion: currentVer,
			ErrorMessage:   err.Error(),
		}, nil
	}

	upToDate := currentVer == latestSHA

	return &SkillsUpdateResult{
		CurrentVersion: currentVer,
		LatestVersion:  latestSHA,
		UpToDate:       upToDate,
	}, nil
}

// skillRepoArchiveURL is the GitHub archive URL for the Tier0-skill main branch.
const skillRepoArchiveURL = "https://github.com/FREEZONEX/Tier0-skill/archive/refs/heads/main.zip"

// UpdateSkills updates skills directly from the Tier0-skill repository, decoupled from CLI releases.
func UpdateSkills(skillsDir string, dryRun bool) (*SkillsUpdateResult, error) {
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		if dryRun {
			return &SkillsUpdateResult{CurrentVersion: "not installed"}, nil
		}
		if err := os.MkdirAll(skillsDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create skills directory: %w", err)
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

	tmpDir, err := os.MkdirTemp("", "tier0-skills-update-*")
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to create temporary directory: %v", err)
		return result, err
	}
	defer os.RemoveAll(tmpDir)

	// Download the Tier0-skill repo archive directly.
	archivePath := filepath.Join(tmpDir, "tier0-skill-main.zip")
	if err := downloadFile(skillRepoArchiveURL, archivePath); err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to download skill repository: %v", err)
		return result, err
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to create extraction directory: %v", err)
		return result, err
	}

	if err := extractZip(archivePath, extractDir); err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to extract skill repository: %v", err)
		return result, err
	}

	// GitHub archive extracts into "Tier0-skill-main/" — that root IS the skill dir.
	srcSkillsDir := findSkillRepoRoot(extractDir)
	if srcSkillsDir == "" {
		result.ErrorMessage = "skill content not found in downloaded package"
		return result, fmt.Errorf("%s", result.ErrorMessage)
	}

	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.Rename(skillsDir, backupDir); err != nil && !os.IsNotExist(err) {
		result.ErrorMessage = fmt.Sprintf("failed to back up old skills: %v", err)
		return result, err
	}

	if err := copyDir(srcSkillsDir, skillsDir); err != nil {
		os.RemoveAll(skillsDir)
		os.Rename(backupDir, skillsDir)
		result.ErrorMessage = fmt.Sprintf("failed to install new skills; rolled back: %v", err)
		return result, err
	}

	// Write _meta.json with the commit SHA so future checks can compare.
	type metaJSON struct {
		Version   string `json:"version"`
		UpdatedAt string `json:"updated_at"`
	}
	metaBytes, _ := json.Marshal(metaJSON{
		Version:   result.LatestVersion,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	_ = os.MkdirAll(skillsDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillsDir, "_meta.json"), append(metaBytes, '\n'), 0o644)

	entries, _ := os.ReadDir(skillsDir)
	for _, e := range entries {
		if e.IsDir() {
			result.UpdatedCount++
		}
	}

	return result, nil
}

// findSkillRepoRoot locates the extracted Tier0-skill repo root.
// GitHub archives extract into a single top-level directory like "Tier0-skill-main/".
func findSkillRepoRoot(extractDir string) string {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(extractDir, e.Name())
		// The repo root should contain SKILL.md directly.
		if _, err := os.Stat(filepath.Join(candidate, "SKILL.md")); err == nil {
			return candidate
		}
	}
	return ""
}

// findSkillDir finds the skill directory in an extracted package.
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

// copyDir copies a directory recursively.
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

// GetDefaultSkillsDir returns the default skills directory path when it exists.
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
		candidate := filepath.Join(home, ".tier0", "skills")
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return candidate
		}
	}
	return ""
}

// FallbackSkillsDir returns ~/.tier0/skills as the default install location
// when no existing skills directory is found. The directory may not exist yet;
// UpdateSkills will create it.
func FallbackSkillsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".tier0", "skills")
	}
	return filepath.Join(home, ".tier0", "skills")
}

// SkillsLastUpdated returns the skills last-updated timestamp.
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
