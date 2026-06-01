package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/cmd"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// HighRiskError (exit 10): HandleCommandError is NOT called for this path.
		// highrisk.WriteEnvelope owns the JSON output.
		var hre *highrisk.HighRiskError
		if errors.As(err, &hre) {
			highrisk.WriteEnvelope(os.Stderr, hre)
			os.Exit(10)
		}

		// For errors that went through HandleCommandError in RunE, stderr is
		// already written — we only need to set the right exit code.
		//
		// For errors that bypassed HandleCommandError (e.g. Cobra's own
		// "unknown flag" messages), emit a plain-text fallback so the user
		// is not left silent, then exit 2 (validation).
		cat := cmdutil.CategoryOf(err)
		if cat == errs.CategoryAPI {
			// Unclassified error: likely a Cobra flag/subcommand error that never
			// went through HandleCommandError. Print it and exit 2.
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		os.Exit(errs.ExitCode(cat))
	}
}
