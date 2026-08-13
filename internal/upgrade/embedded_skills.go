package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/embeddedskill"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

const (
	SkillSourceEmbedded = "embedded"
	SkillSourceRemote   = "remote"
)

// SkillsMetadata records where the active Skill came from. Remote Skills are
// deliberately preserved across CLI upgrades so they can evolve independently.
type SkillsMetadata struct {
	Version            string `json:"version"`
	Source             string `json:"source,omitempty"`
	Commit             string `json:"commit,omitempty"`
	EmbeddedCLIVersion string `json:"embeddedCliVersion,omitempty"`
	ContentSHA256      string `json:"contentSha256,omitempty"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
	LegacyUpdatedAt    string `json:"updated_at,omitempty"`
}

// LastUpdated returns the current timestamp field while remaining compatible
// with metadata written by CLI versions before v0.6.6.
func (m SkillsMetadata) LastUpdated() string {
	if m.UpdatedAt != "" {
		return m.UpdatedAt
	}
	return m.LegacyUpdatedAt
}

// EmbeddedSkillsResult describes how the trusted embedded baseline was applied.
type EmbeddedSkillsResult struct {
	Action          string `json:"action"`
	Path            string `json:"path"`
	Version         string `json:"version"`
	Source          string `json:"source"`
	ContentSHA256   string `json:"contentSha256,omitempty"`
	PreviousVersion string `json:"previousVersion,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// SkillsStatus reports the active package and the baseline available in the
// running CLI without changing local state.
type SkillsStatus struct {
	Installed       bool   `json:"installed"`
	Healthy         bool   `json:"healthy"`
	Path            string `json:"path"`
	Version         string `json:"version"`
	Source          string `json:"source"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
	EmbeddedVersion string `json:"embeddedVersion"`
}

// GetSkillsStatus inspects the active Skill package.
func GetSkillsStatus(skillsDir string) SkillsStatus {
	status := SkillsStatus{
		Path:            skillsDir,
		Version:         "unknown",
		Source:          "none",
		EmbeddedVersion: version.BuildVersion,
	}
	if skillsDir == "" {
		return status
	}
	status.Installed = validSkillPackage(skillsDir)
	status.Healthy = status.Installed
	if !status.Installed {
		return status
	}
	status.Version = GetSkillsVersion(skillsDir)
	status.Source = "external"
	if metadata, err := ReadSkillsMetadata(skillsDir); err == nil {
		if metadata.Source != "" {
			status.Source = metadata.Source
		}
		status.UpdatedAt = metadata.LastUpdated()
		if metadata.Source == SkillSourceEmbedded {
			installedHash, hashErr := hashSkillDir(skillsDir)
			status.Healthy = hashErr == nil && metadata.ContentSHA256 != "" && installedHash == metadata.ContentSHA256
		}
	}
	return status
}

// ReadSkillsMetadata reads the active Skill provenance metadata.
func ReadSkillsMetadata(skillsDir string) (SkillsMetadata, error) {
	var metadata SkillsMetadata
	data, err := os.ReadFile(filepath.Join(skillsDir, "_meta.json"))
	if err != nil {
		return metadata, err
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return metadata, fmt.Errorf("parse Skill metadata: %w", err)
	}
	return metadata, nil
}

// EnsureEmbeddedSkills materializes the Skill compiled into this CLI when no
// usable Skill exists. It upgrades an older embedded baseline, repairs a damaged
// embedded baseline, and preserves independently updated remote Skills unless
// force is explicitly requested.
func EnsureEmbeddedSkills(skillsDir string, force bool) (*EmbeddedSkillsResult, error) {
	if strings.TrimSpace(skillsDir) == "" {
		return nil, fmt.Errorf("Skill install directory is empty")
	}

	baselineFS, err := embeddedskill.FS()
	if err != nil {
		return nil, fmt.Errorf("open embedded Skill: %w", err)
	}
	baselineHash, err := hashSkillFS(baselineFS)
	if err != nil {
		return nil, fmt.Errorf("hash embedded Skill: %w", err)
	}

	metadata, _ := ReadSkillsMetadata(skillsDir)
	valid := validSkillPackage(skillsDir)
	result := &EmbeddedSkillsResult{
		Action:          "preserved",
		Path:            skillsDir,
		Version:         metadata.Version,
		Source:          metadata.Source,
		ContentSHA256:   metadata.ContentSHA256,
		PreviousVersion: metadata.Version,
	}
	if result.Version == "" && valid {
		result.Version = "unknown"
	}
	if result.Source == "" && valid {
		result.Source = "external"
	}

	reason := ""
	switch {
	case force:
		reason = "forced"
	case !valid:
		reason = "missing_or_damaged"
	case metadata.Source == "" && isLegacyBundledSkillVersion(metadata.Version):
		reason = "legacy_embedded_baseline"
	case metadata.Source == SkillSourceEmbedded:
		installedHash, hashErr := hashSkillDir(skillsDir)
		if hashErr != nil || metadata.ContentSHA256 == "" || installedHash != metadata.ContentSHA256 {
			reason = "damaged_embedded_baseline"
		} else if metadata.EmbeddedCLIVersion != version.BuildVersion || installedHash != baselineHash {
			reason = "new_embedded_baseline"
		} else {
			result.Action = "in_sync"
			result.Reason = "embedded_baseline_current"
			return result, nil
		}
	default:
		result.Reason = "independent_update_preserved"
		return result, nil
	}

	metadata = SkillsMetadata{
		Version:            version.BuildVersion,
		Source:             SkillSourceEmbedded,
		EmbeddedCLIVersion: version.BuildVersion,
		ContentSHA256:      baselineHash,
		UpdatedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	if err := installEmbeddedSkillAtomically(baselineFS, skillsDir, metadata); err != nil {
		return nil, err
	}

	result.Action = "installed"
	if result.PreviousVersion != "" && result.PreviousVersion != "unknown" {
		result.Action = "replaced"
	}
	result.Version = metadata.Version
	result.Source = metadata.Source
	result.ContentSHA256 = metadata.ContentSHA256
	result.Reason = reason
	return result, nil
}

func isLegacyBundledSkillVersion(skillVersion string) bool {
	trimmed := strings.TrimPrefix(strings.TrimSpace(skillVersion), "v")
	parts := strings.SplitN(trimmed, ".", 3)
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func validSkillPackage(skillsDir string) bool {
	info, err := os.Stat(filepath.Join(skillsDir, "SKILL.md"))
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func installEmbeddedSkillAtomically(source fs.FS, skillsDir string, metadata SkillsMetadata) error {
	parent := filepath.Dir(skillsDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create Skill parent directory: %w", err)
	}

	stagingDir, err := os.MkdirTemp(parent, ".tier0-skills-embedded-*")
	if err != nil {
		return fmt.Errorf("create Skill staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	if err := copyEmbeddedFS(source, stagingDir); err != nil {
		return fmt.Errorf("stage embedded Skill: %w", err)
	}
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Skill metadata: %w", err)
	}
	metadataBytes = append(metadataBytes, '\n')
	if err := os.WriteFile(filepath.Join(stagingDir, "_meta.json"), metadataBytes, 0o644); err != nil {
		return fmt.Errorf("write Skill metadata: %w", err)
	}

	return activatePreparedSkill(stagingDir, skillsDir)
}

func activatePreparedSkill(stagingDir, skillsDir string) error {
	backupDir := fmt.Sprintf("%s.backup-%d", skillsDir, time.Now().UnixNano())
	hadExisting := false
	if _, err := os.Lstat(skillsDir); err == nil {
		hadExisting = true
		if err := os.Rename(skillsDir, backupDir); err != nil {
			return fmt.Errorf("back up existing Skill: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing Skill: %w", err)
	}

	if err := os.Rename(stagingDir, skillsDir); err != nil {
		if hadExisting {
			_ = os.Rename(backupDir, skillsDir)
		}
		return fmt.Errorf("activate embedded Skill: %w", err)
	}
	if hadExisting {
		_ = os.RemoveAll(backupDir)
	}
	return nil
}

func copyEmbeddedFS(source fs.FS, destination string) error {
	return fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		clean := filepath.Clean(filepath.FromSlash(path))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe embedded Skill path %q", path)
		}
		target := filepath.Join(destination, clean)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported embedded Skill entry %q", path)
		}
		data, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func hashSkillFS(source fs.FS) (string, error) {
	var paths []string
	err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() && filepath.ToSlash(path) != "_meta.json" {
			paths = append(paths, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		file, err := source.Open(path)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(hash, path)
		_, _ = hash.Write([]byte{0})
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func hashSkillDir(skillsDir string) (string, error) {
	return hashSkillFS(os.DirFS(skillsDir))
}
