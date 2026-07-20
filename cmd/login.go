package cmd

import (
	"fmt"
	"io"

	"github.com/FREEZONEX/Tier0-cli/internal/auth"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate via Device Flow",
	Long:  "Authenticate to the Tier0 platform using Device Flow. Opens a browser for authorization and retrieves a Personal API Key.",

	RunE: runLogin,
}

func init() {
	loginCmd.Flags().Bool("no-wait", false,
		"Print the authorization URL and exit without polling")
	loginCmd.Flags().String("setup-code", "",
		"Poll for an existing setup code instead of creating a new one")
	loginCmd.Flags().String("base-url", "",
		"Override the platform base URL")
}

func runLogin(cmd *cobra.Command, args []string) error {
	noWait, _ := cmd.Flags().GetBool("no-wait")
	setupCode, _ := cmd.Flags().GetString("setup-code")
	jsonMode, _ := cmd.Flags().GetBool("json")
	baseURLArg, _ := cmd.Flags().GetString("base-url")
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	baseURL := cmdutil.ResolveBaseURL(baseURLArg)

	if setupCode != "" {
		return loginPoll(cmd, baseURL, setupCode, jsonMode, stdout, stderr)
	}

	setupCode = auth.GenerateSetupCode()
	consoleURL := auth.BuildConsoleURL(baseURL, setupCode)

	if noWait {
		if jsonMode {
			fmt.Fprintf(stdout, `{"status":"authorization_required","verification_url":"%s","setup_code":"%s","expires_in":600}`+"\n", consoleURL, setupCode)
		} else {
			fmt.Fprintf(stdout,
				"Please complete authorization in your browser:\n  %s\n",
				consoleURL)
			fmt.Fprintf(stdout,
				"setup_code: %s\n",
				setupCode)
			fmt.Fprintf(stdout,
				"Polling automatically with: tier0 login --setup-code %s\n",
				setupCode)
		}
		return nil
	}

	fmt.Fprintln(stdout,
		"Please complete authorization in your browser:",
	)
	fmt.Fprintln(stdout, consoleURL)
	fmt.Fprintln(stdout,
		"\nWaiting for authorization... (polling every 5s, up to 10 minutes)",
	)

	return loginPoll(cmd, baseURL, setupCode, jsonMode, stdout, stderr)
}

func loginPoll(cmd *cobra.Command, baseURL, setupCode string, jsonMode bool, stdout, stderr io.Writer) error {
	result, err := auth.PollSetupCheck(cmd.Context(), baseURL, setupCode, func(current, total int, done bool, pollErr error) {
		if jsonMode || done {
			return
		}
		if pollErr != nil {
			if current%6 == 0 {
				fmt.Fprintf(stdout,
					"  Polling... (%d/%d) network hiccup, retrying...\n",
					current+1, total)
			}
			return
		}
		if current%6 == 0 && current > 0 {
			remainingMin := (total - current) * 5 / 60
			fmt.Fprintf(stdout,
				"  Waiting for authorization... (check %d/%d, ~%d min remaining)\n",
				current+1, total, remainingMin)
		}
	})
	if err != nil {
		return cmdutil.HandleCommandError(stderr, err, jsonMode)
	}

	profile := config.Profile{
		BaseURL: baseURL,
		APIKey:  result.APIKey,
	}
	if err := config.SaveProfile(profile); err != nil {
		return configCommandError(cmd, "failed to save config: "+err.Error(), err)
	}

	if jsonMode {
		fmt.Fprintf(stdout, `{"event":"authorization_complete","api_key":"%s"}`+"\n", result.APIKey)
	} else {
		fmt.Fprintln(stdout, "\n✓ Authorization successful!")
		fmt.Fprintf(stdout,
			"API Key: %s... (saved)\n",
			result.APIKey[:8])
		fmt.Fprintln(stdout,
			"✓ Setup complete. You can now use tier0 api commands.",
		)
	}
	return nil
}
