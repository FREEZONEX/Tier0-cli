package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or update configuration",
	Long:  "View or update CLI configuration (base URL and API key). Without flags, displays the current settings.",

	Example: `  tier0 config
  tier0 config --base-url https://tier0.dev
  tier0 config --api-key sk-per-xxxxxx`,

	RunE: runConfig,
}

func init() {
	configCmd.Flags().String("base-url", "",
		"Set the platform base URL")
	configCmd.Flags().String("api-key", "",
		"Set the API key (alternative to tier0 login)")
	configCmd.Flags().String("lang", "",
		"Deprecated compatibility flag; CLI output is English-only")
}

func runConfig(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	setBaseURL, _ := cmd.Flags().GetString("base-url")
	setAPIKey, _ := cmd.Flags().GetString("api-key")
	setLang, _ := cmd.Flags().GetString("lang")
	stdout := cmd.OutOrStdout()

	if setBaseURL != "" || setAPIKey != "" || setLang != "" {
		if setLang != "" {
			setLang = strings.ToLower(strings.TrimSpace(setLang))
			if setLang != "en" {
				return invalidArgument(cmd, "--lang", fmt.Sprintf("unsupported language %q; Tier0 CLI output is English-only", setLang))
			}
		}

		profile, err := config.LoadProfile()
		if err != nil {
			return configCommandError(cmd, "failed to load config: "+err.Error(), err)
		}
		if setBaseURL != "" {
			setBaseURL = strings.TrimRight(strings.TrimSpace(setBaseURL), "/")
			if _, err := config.Origin(setBaseURL); err != nil {
				return invalidArgument(cmd, "--base-url", "expected an HTTP(S) URL without credentials, query, or fragment")
			}
			if !config.SameOrigin(profile.BaseURL, setBaseURL) && setAPIKey == "" && profile.APIKey != "" {
				profile.APIKey = ""
				if !jsonMode {
					fmt.Fprintln(stdout, "Saved API key cleared because the base URL origin changed.")
				}
			}
			profile.BaseURL = setBaseURL
		}
		if setAPIKey != "" {
			profile.APIKey = strings.TrimSpace(setAPIKey)
			if profile.APIKey == "" {
				return invalidArgument(cmd, "--api-key", "API key must not be blank")
			}
			if profile.BaseURL == "" {
				// Bind a newly supplied key, never an existing unbound key. An
				// environment override is ambiguous: ask for an explicit origin.
				if strings.TrimSpace(os.Getenv("TIER0_BASE_URL")) != "" {
					return invalidArgument(cmd, "--base-url", "specify the target base URL when configuring a key with TIER0_BASE_URL set")
				}
				profile.BaseURL = cmdutil.DefaultBaseURL
			}
		}
		if setLang != "" {
			profile.Lang = "en"
		}
		if err := config.ReplaceProfile(profile); err != nil {
			return configCommandError(cmd, "failed to save config: "+err.Error(), err)
		}
		if jsonMode {
			if err := writeConfigJSON(stdout, profile); err != nil {
				return internalCommandError(cmd, "failed to write config JSON: "+err.Error(), err)
			}
			return nil
		}
		if setBaseURL != "" {
			fmt.Fprintf(stdout, "✓ BaseURL set to: %s\n", strings.TrimRight(setBaseURL, "/"))
		}
		if setAPIKey != "" {
			fmt.Fprintf(stdout, "✓ API Key set (%s...)\n", profile.APIKey[:min(8, len(profile.APIKey))])
			if setBaseURL == "" {
				fmt.Fprintf(stdout, "  Bound to BaseURL: %s\n", profile.BaseURL)
			}
		}
		if setLang != "" {
			fmt.Fprintln(stdout, "✓ Language set to: en")
		}
		return nil
	}

	// Read mode
	profile, err := config.LoadProfile()
	if err != nil {
		return configCommandError(cmd, "failed to load config: "+err.Error(), err)
	}

	baseURL := cmdutil.ResolveBaseURL("")
	if profile.BaseURL != "" {
		baseURL = profile.BaseURL
	}
	if jsonMode {
		profile.BaseURL = baseURL
		if err := writeConfigJSON(stdout, profile); err != nil {
			return internalCommandError(cmd, "failed to write config JSON: "+err.Error(), err)
		}
		return nil
	}
	fmt.Fprintf(stdout, "BaseURL:  %s\n", baseURL)
	fmt.Fprintln(stdout, "Language: en")
	if profile.APIKey != "" {
		fmt.Fprintf(stdout, "API Key:  %s...\n", profile.APIKey[:min(8, len(profile.APIKey))])
	} else {
		fmt.Fprintln(stdout, "API Key:  (not set)")
	}
	return nil
}

func writeConfigJSON(stdout io.Writer, profile config.Profile) error {
	baseURL := profile.BaseURL
	if baseURL == "" {
		baseURL = cmdutil.ResolveBaseURL("")
	}
	prefix := ""
	if profile.APIKey != "" {
		prefix = profile.APIKey[:min(8, len(profile.APIKey))]
	}
	result := map[string]any{
		"baseURL":          baseURL,
		"language":         "en",
		"apiKeyConfigured": profile.APIKey != "",
		"apiKeyPrefix":     prefix,
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, string(encoded))
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
