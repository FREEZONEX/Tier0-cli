package upgrade

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const (
	npmInstallTimeout = 10 * time.Minute
	npmPackage        = "@tier0/cli"
)

// NpmResult holds the output of an npm install run.
type NpmResult struct {
	Stdout string
	Stderr string
	Err    error
}

// NpmAvailable reports whether npm is in PATH.
func NpmAvailable() bool {
	_, err := exec.LookPath("npm")
	return err == nil
}

// RunNpmInstall runs: npm install -g @tier0/cli@<version>
// Tries default registry first; if it fails, retries with npmmirror.com.
// The package's postinstall script downloads the binary to ~/.tier0/bin/.
func RunNpmInstall(version string) *NpmResult {
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		return &NpmResult{Err: fmt.Errorf("npm not found in PATH: %w", err)}
	}

	if version != "" && version[0] != 'v' {
		version = "v" + version
	}
	target := npmPackage
	if version != "" {
		target = npmPackage + "@" + version
	}

	// Try default registry first, then npmmirror as fallback.
	registries := []string{"", "https://registry.npmmirror.com"}
	for i, registry := range registries {
		r := runNpmInstallWithRegistry(npmPath, target, registry)
		if r.Err == nil {
			return r
		}
		if i < len(registries)-1 {
			// first attempt failed, try mirror
			continue
		}
		return r
	}
	return &NpmResult{Err: fmt.Errorf("npm install failed on all registries")}
}

func runNpmInstallWithRegistry(npmPath, target, registry string) *NpmResult {
	r := &NpmResult{}
	ctx, cancel := context.WithTimeout(context.Background(), npmInstallTimeout)
	defer cancel()

	args := []string{"install", "-g", target}
	if registry != "" {
		args = append(args, "--registry", registry)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, npmPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	r.Err = cmd.Run()
	r.Stdout = stdout.String()
	r.Stderr = stderr.String()

	if ctx.Err() == context.DeadlineExceeded {
		r.Err = fmt.Errorf("npm install timed out after %s", npmInstallTimeout)
	}
	return r
}
