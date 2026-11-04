package cmd

import (
	"errors"
	"fmt"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
	"github.com/spf13/cobra"
)

func fileMutationUnknown(cmd *cobra.Command, operation string, cause error) error {
	category, code, status := errs.CategoryAPI, 0, 0
	var api *apierr.APIError
	var cli *errs.CLIError
	if errors.As(cause, &api) {
		category, status, code = api.Category(), api.HTTPStatus, api.Code
		if code == 0 {
			code = status
		}
	} else if errors.As(cause, &cli) {
		category, code, status = cli.Cat, cli.Code, cli.HTTPStatus
	}
	msg := fmt.Sprintf("Cloud file %s outcome is unknown", operation)
	if status != 0 {
		msg = fmt.Sprintf("Cloud file %s failed (HTTP %d); outcome is unknown", operation, status)
	}
	err := errs.New(category, code, msg).WithHTTPStatus(status).WithExecution("unknown").WithHint("Inspect the remote file before taking further action.", "")
	jsonMode, _ := cmd.Flags().GetBool("json")
	return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
}
