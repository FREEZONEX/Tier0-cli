package upgrade

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

func TestPerformNpmVerifiesInstalledVersionSuccess(t *testing.T) {
	restore := stubUpgradeGlobals(t, "v9.9.9", 0)
	defer restore()

	npmCalled := false
	runNpmInstall = func(version string) *NpmResult {
		npmCalled = true
		if version != "v9.9.9" {
			t.Fatalf("RunNpmInstall version = %q, want v9.9.9", version)
		}
		return &NpmResult{}
	}

	result, err := Perform(Options{TargetVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("Perform returned error: %v", err)
	}
	if !npmCalled {
		t.Fatal("RunNpmInstall was not called")
	}
	if result.Method != "npm" {
		t.Fatalf("method = %q, want npm", result.Method)
	}
	if result.ErrorMessage != "" {
		t.Fatalf("ErrorMessage = %q, want empty", result.ErrorMessage)
	}
}

func TestPerformNpmVerificationFailureReturnsError(t *testing.T) {
	restore := stubUpgradeGlobals(t, "", 137)
	defer restore()

	runNpmInstall = func(version string) *NpmResult {
		return &NpmResult{}
	}

	result, err := Perform(Options{TargetVersion: "v9.9.9"})
	if err == nil {
		t.Fatal("Perform returned nil error, want verification error")
	}
	if result.Method == "npm" {
		t.Fatalf("method = %q, want not npm on failed verification", result.Method)
	}
	if !strings.Contains(result.ErrorMessage, "installed binary verification failed") {
		t.Fatalf("ErrorMessage = %q, want verification failure", result.ErrorMessage)
	}
	if !strings.Contains(result.ErrorMessage, "exit status 137") {
		t.Fatalf("ErrorMessage = %q, want exit status 137", result.ErrorMessage)
	}
}

func TestVerifyInstalledVersionFallsBackToCurrentExecutable(t *testing.T) {
	restore := stubUpgradeGlobals(t, "tier0 version v9.9.9\n", 0)
	defer restore()

	execLookPath = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}

	if err := verifyInstalledVersion("v9.9.9"); err != nil {
		t.Fatalf("verifyInstalledVersion returned error: %v", err)
	}
}

func stubUpgradeGlobals(t *testing.T, versionOutput string, exitCode int) func() {
	t.Helper()

	oldBuildVersion := version.BuildVersion
	oldFetchLatestRelease := fetchLatestRelease
	oldFetchRelease := fetchRelease
	oldNpmAvailable := npmAvailable
	oldRunNpmInstall := runNpmInstall
	oldExecLookPath := execLookPath
	oldExecCommandContext := execCommandContext
	oldOutput := os.Getenv("TIER0_TEST_VERSION_OUTPUT")
	oldExitCode := os.Getenv("TIER0_TEST_EXIT_CODE")
	_, hadOutput := os.LookupEnv("TIER0_TEST_VERSION_OUTPUT")
	_, hadExitCode := os.LookupEnv("TIER0_TEST_EXIT_CODE")

	version.BuildVersion = "v0.0.1"
	fetchLatestRelease = func() (*Release, error) {
		return buildReleaseFromVersion("v9.9.9"), nil
	}
	fetchRelease = func(ver string) (*Release, error) {
		return buildReleaseFromVersion(ver), nil
	}
	npmAvailable = func() bool { return true }
	runNpmInstall = func(version string) *NpmResult {
		return &NpmResult{}
	}
	execLookPath = func(file string) (string, error) {
		if file != "tier0" {
			return "", fmt.Errorf("unexpected executable lookup: %s", file)
		}
		return "tier0-test-helper", nil
	}
	execCommandContext = fakeExecCommandContext
	os.Setenv("TIER0_TEST_VERSION_OUTPUT", versionOutput)
	os.Setenv("TIER0_TEST_EXIT_CODE", fmt.Sprintf("%d", exitCode))

	return func() {
		version.BuildVersion = oldBuildVersion
		fetchLatestRelease = oldFetchLatestRelease
		fetchRelease = oldFetchRelease
		npmAvailable = oldNpmAvailable
		runNpmInstall = oldRunNpmInstall
		execLookPath = oldExecLookPath
		execCommandContext = oldExecCommandContext
		if hadOutput {
			os.Setenv("TIER0_TEST_VERSION_OUTPUT", oldOutput)
		} else {
			os.Unsetenv("TIER0_TEST_VERSION_OUTPUT")
		}
		if hadExitCode {
			os.Setenv("TIER0_TEST_EXIT_CODE", oldExitCode)
		} else {
			os.Unsetenv("TIER0_TEST_EXIT_CODE")
		}
	}
}

func fakeExecCommandContext(ctx context.Context, command string, args ...string) *exec.Cmd {
	cmdArgs := []string{"-test.run=TestHelperProcess", "--", command}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Print(os.Getenv("TIER0_TEST_VERSION_OUTPUT"))
	var exitCode int
	fmt.Sscanf(os.Getenv("TIER0_TEST_EXIT_CODE"), "%d", &exitCode)
	os.Exit(exitCode)
}
