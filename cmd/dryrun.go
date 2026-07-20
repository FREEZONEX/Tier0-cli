package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
	"github.com/spf13/cobra"
)

func addDryRunFlag(command *cobra.Command) {
	command.Flags().Bool("dry-run", false, "Preview the API request without sending it")
}

func writeDryRun(command *cobra.Command, method, endpoint string, body interface{}) (bool, error) {
	dryRun, _ := command.Flags().GetBool("dry-run")
	if !dryRun {
		return false, nil
	}
	jsonMode, _ := command.Flags().GetBool("json")
	preview := cmdutil.NewRequestPreview(method, endpoint, body)
	if err := cmdutil.WriteDryRun(command.OutOrStdout(), preview, jsonMode); err != nil {
		cliErr := errs.New(errs.CategoryInternal, 0, "failed to write dry-run output: "+err.Error()).
			WithSubtype(errs.SubtypeUnknown).
			WithCause(err)
		return true, cmdutil.HandleCommandError(command.ErrOrStderr(), cliErr, jsonMode)
	}
	return true, nil
}

func invalidArgument(command *cobra.Command, param, message string) error {
	return handleCLIError(command, errs.InvalidArgument(param, message))
}

func invalidArgumentCause(command *cobra.Command, param, message string, cause error) error {
	return handleCLIError(command, errs.InvalidArgument(param, message).WithCause(cause))
}

func fileIOError(command *cobra.Command, param, action, path string, cause error) error {
	message := fmt.Sprintf("failed to %s %q: %v", action, path, cause)
	return handleCLIError(command, errs.FileIO(param, message, cause))
}

func internalCommandError(command *cobra.Command, message string, cause error) error {
	cliErr := errs.New(errs.CategoryInternal, 0, message).
		WithSubtype(errs.SubtypeUnknown).
		WithCause(cause)
	return handleCLIError(command, cliErr)
}

func configCommandError(command *cobra.Command, message string, cause error) error {
	cliErr := errs.New(errs.CategoryConfig, 0, message).
		WithSubtype(errs.SubtypeFileIO).
		WithCause(cause)
	return handleCLIError(command, cliErr)
}

func handleCLIError(command *cobra.Command, cliErr *errs.CLIError) error {
	jsonMode, _ := command.Flags().GetBool("json")
	return cmdutil.HandleCommandError(command.ErrOrStderr(), cliErr, jsonMode)
}

func decodeJSONInput(raw string) (interface{}, error) {
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	return value, nil
}
