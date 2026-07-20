package upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var agentSkillsLookPath = exec.LookPath

var agentSkillsRun = func(name string, args ...string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/d", "/s", "/c", name}, args...)
		return exec.Command("cmd.exe", cmdArgs...).CombinedOutput()
	}
	return exec.Command(name, args...).CombinedOutput()
}

// AgentSkillsSyncArgs builds a cross-platform, non-interactive local install.
// --copy avoids symlink permission requirements on Windows.
func AgentSkillsSyncArgs(skillsDir string) []string {
	abs, err := filepath.Abs(skillsDir)
	if err != nil {
		abs = skillsDir
	}
	return []string{"-y", "skills", "add", abs, "-y", "-g", "--copy"}
}

// SyncAgentSkills copies the locally versioned Tier0 Skill into detected
// global agent directories such as Codex and Claude Code.
func SyncAgentSkills(skillsDir string) error {
	if skillsDir == "" {
		return fmt.Errorf("local Skill directory is empty")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "SKILL.md")); err != nil {
		return fmt.Errorf("local Skill package not found at %s: %w", skillsDir, err)
	}

	npxPath, err := agentSkillsLookPath("npx")
	if err != nil {
		return fmt.Errorf("npx is required to sync Agent Skills: %w", err)
	}
	output, err := agentSkillsRun(npxPath, AgentSkillsSyncArgs(skillsDir)...)
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("Agent Skills sync failed: %s", detail)
	}
	return nil
}
