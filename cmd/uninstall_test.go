package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func withUninstallFakes(t *testing.T) {
	t.Helper()
	originalLookPath := uninstallLookPath
	originalRunCommand := uninstallRunCommand
	originalUserHomeDir := uninstallUserHomeDir
	t.Cleanup(func() {
		uninstallLookPath = originalLookPath
		uninstallRunCommand = originalRunCommand
		uninstallUserHomeDir = originalUserHomeDir
	})
}

func TestUninstallKeepsAgentSkillAndConfigByDefault(t *testing.T) {
	withUninstallFakes(t)
	home := createUninstallFixture(t)
	uninstallUserHomeDir = func() (string, error) { return home, nil }
	uninstallLookPath = func(name string) (string, error) { return name, nil }
	uninstallRunCommand = func(name string, args []string, env []string) ([]byte, error) {
		if args[0] == "root" {
			return []byte(t.TempDir()), nil
		}
		t.Fatalf("unexpected external command: %s %v", name, args)
		return nil, nil
	}

	var stdout bytes.Buffer
	cmd := newUninstallTestCommand(&stdout, false, false)
	if err := runUninstall(cmd, nil); err != nil {
		t.Fatalf("runUninstall() error = %v", err)
	}
	assertPathExists(t, filepath.Join(home, ".agents", "skills", "tier0"))
	assertPathExists(t, filepath.Join(home, ".tier0", "config.json"))
	assertPathMissing(t, filepath.Join(home, ".tier0", "skills"))
	if !strings.Contains(stdout.String(), "Agent Skill kept") {
		t.Fatalf("output does not explain retained Skill: %s", stdout.String())
	}
}

func TestUninstallPurgeAndRemoveSkillsCleansAllManagedState(t *testing.T) {
	withUninstallFakes(t)
	home := createUninstallFixture(t)
	npmRoot := t.TempDir()
	npmPackageDir := filepath.Join(npmRoot, "@tier0", "cli")
	if err := os.MkdirAll(npmPackageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npmPackageDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	uninstallUserHomeDir = func() (string, error) { return home, nil }
	uninstallLookPath = func(name string) (string, error) { return name, nil }
	uninstallRunCommand = func(name string, args []string, env []string) ([]byte, error) {
		switch {
		case name == "npm" && args[0] == "root":
			return []byte(npmRoot), nil
		case name == "npm" && args[0] == "uninstall":
			return nil, os.RemoveAll(npmPackageDir)
		case name == "npx":
			return nil, os.RemoveAll(filepath.Join(home, ".agents", "skills", "tier0"))
		default:
			t.Fatalf("unexpected external command: %s %v", name, args)
			return nil, nil
		}
	}

	var stdout bytes.Buffer
	cmd := newUninstallTestCommand(&stdout, true, true)
	if err := runUninstall(cmd, nil); err != nil {
		t.Fatalf("runUninstall() error = %v", err)
	}
	assertPathMissing(t, filepath.Join(home, ".tier0"))
	assertPathMissing(t, filepath.Join(home, ".agents", "skills", "tier0"))
	assertPathMissing(t, npmPackageDir)
}

func TestSkillsRemoveArgsAreGlobalAndUseInstalledName(t *testing.T) {
	want := []string{"-y", "--package=skills", "--", "skills", "remove", "tier0", "-y", "-g"}
	if got := skillsRemoveArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("skillsRemoveArgs() = %v, want %v", got, want)
	}
}

func TestWindowsBinaryRemovalScriptWaitsForParentAndRetries(t *testing.T) {
	path := `C:\Users\O'Brien\.tier0\bin\tier0.exe`
	script := windowsBinaryRemovalScript(path, 1234)
	wants := []string{
		"Get-Process -Id 1234",
		"$i -lt 1200",
		"$i -lt 40",
		`C:\Users\O''Brien\.tier0\bin\tier0.exe`,
		`C:\Users\O''Brien\.tier0\bin`,
		`C:\Users\O''Brien\.tier0`,
	}
	for _, want := range wants {
		if !strings.Contains(script, want) {
			t.Fatalf("Windows cleanup script does not contain %q:\n%s", want, script)
		}
	}
}

func TestRunNpxSkillsRemoveVerifiesRemoval(t *testing.T) {
	withUninstallFakes(t)
	home := t.TempDir()
	skillDir := filepath.Join(home, ".agents", "skills", "tier0")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uninstallLookPath = func(name string) (string, error) { return name, nil }
	uninstallRunCommand = func(name string, args []string, env []string) ([]byte, error) {
		if name != "npx" || !reflect.DeepEqual(args, skillsRemoveArgs()) {
			t.Fatalf("unexpected command: %s %v", name, args)
		}
		return nil, os.RemoveAll(skillDir)
	}

	if err := runNpxSkillsRemove(home); err != nil {
		t.Fatalf("runNpxSkillsRemove() error = %v", err)
	}
}

func TestRunNpxSkillsRemoveRejectsFalseSuccess(t *testing.T) {
	withUninstallFakes(t)
	home := t.TempDir()
	skillDir := filepath.Join(home, ".agents", "skills", "tier0")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uninstallLookPath = func(name string) (string, error) { return name, nil }
	uninstallRunCommand = func(name string, args []string, env []string) ([]byte, error) {
		return []byte("No matching skills found"), nil
	}

	err := runNpxSkillsRemove(home)
	if err == nil || !strings.Contains(err.Error(), "reported success") {
		t.Fatalf("runNpxSkillsRemove() error = %v, want post-condition failure", err)
	}
}

func TestRemoveGlobalNpmPackageDisablesLifecycleCleanup(t *testing.T) {
	withUninstallFakes(t)
	npmRoot := t.TempDir()
	packageDir := filepath.Join(npmRoot, "@tier0", "cli")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	uninstallLookPath = func(name string) (string, error) {
		if name != "npm" {
			return "", errors.New("unexpected executable")
		}
		return name, nil
	}
	uninstallRunCommand = func(name string, args []string, env []string) ([]byte, error) {
		switch args[0] {
		case "root":
			return []byte(npmRoot), nil
		case "uninstall":
			if !containsEnv(env, "TIER0_SKIP_UNINSTALL=1") {
				t.Fatal("npm uninstall did not disable recursive lifecycle cleanup")
			}
			return nil, os.RemoveAll(packageDir)
		default:
			t.Fatalf("unexpected npm command: %v", args)
			return nil, nil
		}
	}

	var stdout bytes.Buffer
	removed, err := removeGlobalNpmPackage(&stdout)
	if err != nil {
		t.Fatalf("removeGlobalNpmPackage() error = %v", err)
	}
	if !removed || !strings.Contains(stdout.String(), npmPackageName) {
		t.Fatalf("removed = %t, output = %q", removed, stdout.String())
	}
}

func containsEnv(env []string, value string) bool {
	for _, item := range env {
		if item == value {
			return true
		}
	}
	return false
}

func createUninstallFixture(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	binName := "tier0"
	if runtime.GOOS == "windows" {
		binName = "tier0.exe"
	}
	files := map[string]string{
		filepath.Join(home, ".tier0", "bin", binName):                 "binary",
		filepath.Join(home, ".tier0", "bin", ".version"):              "v0.0.0",
		filepath.Join(home, ".tier0", "skills", "SKILL.md"):           "skill",
		filepath.Join(home, ".tier0", "config.json"):                  "config",
		filepath.Join(home, ".agents", "skills", "tier0", "SKILL.md"): "agent skill",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func newUninstallTestCommand(stdout *bytes.Buffer, purge, removeSkills bool) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.Flags().Bool("purge", purge, "")
	cmd.Flags().Bool("remove-skills", removeSkills, "")
	return cmd
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat error = %v", path, err)
	}
}
