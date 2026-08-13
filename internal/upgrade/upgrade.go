package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
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

var (
	fetchLatestRelease  = FetchLatestRelease
	fetchRelease        = FetchRelease
	npmAvailable        = NpmAvailable
	runNpmInstall       = RunNpmInstall
	execLookPath        = exec.LookPath
	execCommandContext  = exec.CommandContext
	prepareUpgradeSkill = prepareInstalledSkill
)

// Options controls upgrade behavior.
type Options struct {
	TargetVersion string // Specific target version; empty means latest.
	DryRun        bool   // Check only without installing.
}

// Result is the upgrade result.
type Result struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpToDate       bool   `json:"upToDate"`
	DownloadURL    string `json:"downloadUrl,omitempty"`
	ErrorMessage   string `json:"error,omitempty"`
	// Method indicates how the upgrade was performed: "npm" or "github"
	Method      string `json:"method,omitempty"`
	SkillStatus string `json:"skillStatus,omitempty"`
	SkillError  string `json:"skillError,omitempty"`
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

// Perform executes the upgrade.
//
// Strategy, aligned with lark-cli:
//  1. If npm is available, run npm install -g @tier0/cli@<version>.
//  2. Otherwise, download the binary from GitHub Releases and replace it.
func Perform(opts Options) (*Result, error) {
	var release *Release
	var err error
	if opts.TargetVersion != "" {
		release, err = fetchRelease(opts.TargetVersion)
	} else {
		release, err = fetchLatestRelease()
	}
	if err != nil {
		return &Result{CurrentVersion: version.BuildVersion, ErrorMessage: err.Error()}, err
	}

	if opts.TargetVersion == "" && !IsNewer(version.BuildVersion, release.TagName) {
		saveCachedState(&updateState{
			LatestVersion: release.TagName,
			CheckedAt:     time.Now().Unix(),
		})
		result := &Result{
			CurrentVersion: version.BuildVersion,
			LatestVersion:  release.TagName,
			UpToDate:       true,
		}
		if !opts.DryRun {
			result.SkillStatus, result.SkillError = prepareUpgradeSkill("")
		}
		return result, nil
	}

	asset := release.FindAsset()
	if asset == nil {
		e := fmt.Errorf("no package found for current platform (%s)", platformReleaseName())
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
	if npmAvailable() {
		npmResult := runNpmInstall(release.TagName)
		if npmResult.Err != nil {
			// npm failed; fall through to direct download below
			result.ErrorMessage = fmt.Sprintf("npm install failed; trying direct download: %v", npmResult.Err)
		} else if err := verifyInstalledVersion(release.TagName); err != nil {
			result.ErrorMessage = fmt.Sprintf("npm install completed, but installed binary verification failed: %v", err)
			return result, err
		} else {
			result.Method = "npm"
			result.SkillStatus, result.SkillError = prepareUpgradeSkill("")
			return result, nil
		}
	}

	// ── Path 2: direct GitHub download ───────────────────────────────────
	// Security note: the npm path benefits from npm registry verification and
	// is preferred. Direct download verifies SHA256 checksums as a fallback.
	result.Method = "github"
	binaryPath, err := os.Executable()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to get executable path: %v", err)
		return result, err
	}
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to resolve executable path: %v", err)
		return result, err
	}

	if err := backupBinary(binaryPath, version.BuildVersion); err != nil {
		result.ErrorMessage = fmt.Sprintf("backup failed: %v", err)
		return result, err
	}

	tmpDir, err := os.MkdirTemp("", "tier0-upgrade-*")
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to create temporary directory: %v", err)
		return result, err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := downloadFile(asset.BrowserDownloadURL, archivePath); err != nil {
		result.ErrorMessage = fmt.Sprintf("download failed: %v", err)
		return result, err
	}

	// Download and verify SHA256 checksum to detect tampering or corruption.
	if err := verifyReleaseChecksum(release.TagName, asset.Name, archivePath); err != nil {
		result.ErrorMessage = fmt.Sprintf("SHA256 verification failed: %v", err)
		return result, err
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to create extraction directory: %v", err)
		return result, err
	}

	if err := extract(archivePath, extractDir); err != nil {
		result.ErrorMessage = fmt.Sprintf("extraction failed: %v", err)
		return result, err
	}

	newBinary, err := findBinary(extractDir)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to find new binary: %v", err)
		return result, err
	}

	if err := replaceBinary(newBinary, binaryPath); err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to replace binary: %v; old version was backed up to ~/.tier0/backup/", err)
		return result, err
	}
	result.SkillStatus, result.SkillError = prepareUpgradeSkill(binaryPath)

	return result, nil
}

func prepareInstalledSkill(binaryPath string) (string, string) {
	if binaryPath == "" {
		var err error
		binaryPath, err = execLookPath("tier0")
		if err != nil {
			return "failed", fmt.Sprintf("cannot locate upgraded CLI for Skill install: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := execCommandContext(ctx, binaryPath, "skills", "install", "--no-sync", "--json").CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "failed", "embedded Skill install timed out after 30s"
	}
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return "failed", detail
	}

	if err := SyncAgentSkills(FallbackSkillsDir()); err != nil {
		return "installed", err.Error()
	}
	return "synced", ""
}

func verifyInstalledVersion(expectedVersion string) error {
	exe, err := execLookPath("tier0")
	if err != nil {
		exe, err = os.Executable()
		if err != nil {
			return fmt.Errorf("cannot locate tier0 executable: %w", err)
		}
		if resolved, resolveErr := filepath.EvalSymlinks(exe); resolveErr == nil {
			exe = resolved
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := execCommandContext(ctx, exe, "version").Output()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("tier0 version timed out after 10s")
	}
	if err != nil {
		return fmt.Errorf("tier0 version failed: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return fmt.Errorf("tier0 version returned empty output")
	}
	actual := strings.TrimPrefix(fields[len(fields)-1], "v")
	expected := strings.TrimPrefix(expectedVersion, "v")
	if actual != expected {
		return fmt.Errorf("expected version %s, got %q", expectedVersion, actual)
	}
	return nil
}

// verifyReleaseChecksum downloads sha256sums.txt from the GitHub Release,
// finds the expected value for assetName, and compares it with the local file.
// Verification failure stops installation.
func verifyReleaseChecksum(version, assetName, localPath string) error {
	sumsURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/sha256sums.txt",
		RepoOwner, RepoName, version,
	)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(sumsURL)
	if err != nil {
		return fmt.Errorf("failed to download sha256sums.txt: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sha256sums.txt returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to read sha256sums.txt: %w", err)
	}

	expected, err := parseChecksum(string(body), assetName)
	if err != nil {
		return err
	}

	actual, err := sha256File(localPath)
	if err != nil {
		return fmt.Errorf("failed to calculate local file SHA256: %w", err)
	}

	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("SHA256 mismatch; download may be corrupted or tampered with:\n  expected: %s\n  actual: %s", expected, actual)
	}
	return nil
}

// parseChecksum extracts the SHA256 for filename from sha256sum(1) text.
// Format: <hash>  <filename>.
func parseChecksum(sumsText, filename string) (string, error) {
	for _, line := range strings.Split(sumsText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// sha256sum format: hash  filename, or hash *filename in binary mode.
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimPrefix(parts[1], "*")
		if name == filename || filepath.Base(name) == filename {
			return strings.ToLower(parts[0]), nil
		}
	}
	return "", fmt.Errorf("checksum for file %q not found in sha256sums.txt", filename)
}

// sha256File computes the SHA256 hex string for a file.
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

// BackupDir returns the backup directory.
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
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	backupName := filepath.Base(binaryPath) + "-" + oldVersion
	if runtime.GOOS == "windows" {
		backupName += ".exe"
	}
	backupPath := filepath.Join(dir, backupName)

	src, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read current binary: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
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
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status code %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create download file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("failed to write download file: %w", err)
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
	return fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
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
		return "", fmt.Errorf("tier0 binary not found in package")
	}
	return found, nil
}

func replaceBinary(newPath, oldPath string) error {
	if runtime.GOOS == "windows" {
		oldBak := oldPath + ".old"
		// Remove a stale backup if present; otherwise rename can fail.
		os.Remove(oldBak)
		if err := os.Rename(oldPath, oldBak); err != nil {
			return fmt.Errorf("failed to back up old version: %w", err)
		}
		if err := os.Rename(newPath, oldPath); err != nil {
			// Roll back.
			os.Rename(oldBak, oldPath)
			return fmt.Errorf("failed to replace with new version: %w", err)
		}
		// Windows may not immediately delete a running old file; ignore this.
		os.Remove(oldBak)
		return nil
	}

	if err := os.Chmod(newPath, 0o755); err != nil {
		return err
	}

	if err := os.Rename(newPath, oldPath); err != nil {
		return err
	}

	// macOS: after replacing a downloaded binary with rename, AMFI/Gatekeeper
	// may SIGKILL the new process if the signing state is not clean. Repair with
	// ad-hoc signing and remove the quarantine extended attribute.
	if runtime.GOOS == "darwin" {
		fixMacOSSigning(oldPath)
	}

	return nil
}

// fixMacOSSigning applies ad-hoc signing to a replaced macOS binary and removes
// quarantine so AMFI/Gatekeeper does not SIGKILL the next execution. Both steps
// are best-effort and should not block upgrade if tools are unavailable.
func fixMacOSSigning(binaryPath string) {
	// Step 1: remove the quarantine extended attribute added to URL downloads.
	// xattr returns non-zero when the attribute does not exist; that is harmless.
	execQuiet("xattr", "-d", "com.apple.quarantine", binaryPath)

	// Step 2: ad-hoc sign with -s - and force replacement with -f.
	// This matches the standard approach used when no Apple developer
	// certificate is available.
	execQuiet("codesign", "-f", "-s", "-", binaryPath)
}

// execQuiet runs an external command and ignores all output and errors.
func execQuiet(name string, args ...string) {
	cmd := exec.Command(name, args...)
	_ = cmd.Run()
}
