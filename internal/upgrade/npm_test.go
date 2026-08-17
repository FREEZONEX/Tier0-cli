package upgrade

import (
	"errors"
	"strings"
	"testing"
)

func TestNpmFailureDetailIncludesCapturedStderr(t *testing.T) {
	detail := npmFailureDetail(&NpmResult{
		Err:    errors.New("exit status 1"),
		Stderr: "npm error\nInstallation failed: EBUSY: resource busy or locked, copyfile tier0.exe",
	})

	if !strings.Contains(detail, "exit status 1") {
		t.Fatalf("detail = %q, want exit status", detail)
	}
	if !strings.Contains(detail, "EBUSY: resource busy or locked") {
		t.Fatalf("detail = %q, want captured stderr", detail)
	}
}

func TestNpmFailureDetailFallsBackToStdout(t *testing.T) {
	detail := npmFailureDetail(&NpmResult{
		Err:    errors.New("exit status 1"),
		Stdout: "postinstall failed",
	})

	if detail != "exit status 1: postinstall failed" {
		t.Fatalf("detail = %q", detail)
	}
}

func TestNpmFailureDetailLimitsOutput(t *testing.T) {
	detail := npmFailureDetail(&NpmResult{
		Err:    errors.New("exit status 1"),
		Stderr: strings.Repeat("x", npmFailureDetailLimit) + "TAIL",
	})

	if !strings.HasSuffix(detail, "TAIL") {
		t.Fatalf("detail does not preserve stderr tail: %q", detail)
	}
	if len(detail) > npmFailureDetailLimit+len("exit status 1: ...") {
		t.Fatalf("detail length = %d, want bounded output", len(detail))
	}
}
