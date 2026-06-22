package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/client"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose local Tier0 CLI connectivity and auth",
	Long:  "Run local diagnostics for the configured Tier0 instance: base URL, API key presence, OpenAPI connectivity, and API key identity.",

	RunE: runDoctor,
}

type doctorCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	stdout := cmd.OutOrStdout()

	profile, cfgErr := config.LoadProfile()
	baseURL := cmdutil.ResolveBaseURL("")
	checks := []doctorCheck{}

	if cfgErr != nil {
		checks = append(checks, doctorCheck{
			Name: "config", OK: false, Message: cfgErr.Error(),
			Hint: "Run tier0 config --base-url <url> --api-key <key>.",
		})
	} else {
		checks = append(checks, doctorCheck{Name: "config", OK: true, Message: configMessage(baseURL, profile)})
	}

	infoResp, infoErr := doctorInfo(cmd.Context(), baseURL, debug)
	if infoErr != nil {
		checks = append(checks, doctorCheck{
			Name: "openapi_info", OK: false, Message: infoErr.Error(),
			Hint: "Check the base URL with tier0 config --base-url <url>.",
		})
	} else {
		checks = append(checks, doctorCheck{Name: "openapi_info", OK: true, Message: infoResp})
	}

	if strings.TrimSpace(profile.APIKey) == "" {
		checks = append(checks, doctorCheck{
			Name: "api_key", OK: false, Message: "API key is not configured",
			Hint: "Run tier0 login or tier0 config --api-key <key>.",
		})
	} else {
		checks = append(checks, doctorCheck{Name: "api_key", OK: true, Message: maskSecret(profile.APIKey)})
		whoResp, whoErr := client.New(baseURL, profile.APIKey).DoAPI(cmd.Context(), "/openapi/v1/auth/whoami", "POST", "{}", debug)
		if whoErr != nil {
			checks = append(checks, doctorCheck{Name: "auth_whoami", OK: false, Message: whoErr.Error(), Hint: "Verify the API key and workspace permissions."})
		} else if err := cmdutil.CheckOK(whoResp); err != nil {
			checks = append(checks, doctorCheck{Name: "auth_whoami", OK: false, Message: err.Error(), Hint: "Verify the API key and workspace permissions."})
		} else {
			checks = append(checks, doctorCheck{Name: "auth_whoami", OK: true, Message: doctorWhoamiSummary(whoResp)})
		}
	}

	allOK := true
	for _, check := range checks {
		if !check.OK {
			allOK = false
			break
		}
	}
	if jsonMode {
		out := map[string]any{"ok": allOK, "baseURL": baseURL, "checks": checks}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(stdout, string(data))
		if !allOK {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), doctorFailure(checks), true)
		}
		return nil
	}
	printDoctor(stdout, checks)
	if !allOK {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), doctorFailure(checks), jsonMode)
	}
	return nil
}

func doctorFailure(checks []doctorCheck) error {
	category := errs.CategoryConfig
	for _, check := range checks {
		if check.OK {
			continue
		}
		if check.Name == "openapi_info" {
			category = errs.CategoryNetwork
			break
		}
	}
	return errs.New(category, 0, "doctor found issues").
		WithHint("Review the failed doctor checks above.", "")
}

func doctorInfo(ctx context.Context, baseURL string, debug bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/openapi/v1/info", strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tier0-Source", "tier0-cli")
	if debug {
		fmt.Fprintf(os.Stderr, "[debug] POST %s\n", req.URL.String())
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return doctorInfoSummary(string(body)), nil
}

func doctorInfoSummary(resp string) string {
	var result struct {
		Name         string   `json:"name"`
		Version      string   `json:"version"`
		MqttBroker   string   `json:"mqttBroker"`
		Capabilities []string `json:"capabilities"`
	}
	_ = json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result)
	parts := []string{}
	if result.Name != "" {
		parts = append(parts, result.Name)
	}
	if result.Version != "" {
		parts = append(parts, "version="+result.Version)
	}
	if result.MqttBroker != "" {
		parts = append(parts, "mqtt="+result.MqttBroker)
	}
	if len(result.Capabilities) > 0 {
		parts = append(parts, "capabilities="+strings.Join(result.Capabilities, ","))
	}
	if len(parts) == 0 {
		return resp
	}
	return strings.Join(parts, "; ")
}

func doctorWhoamiSummary(resp string) string {
	var result struct {
		UserName      string `json:"userName"`
		Email         string `json:"email"`
		WorkspaceName string `json:"workspaceName"`
		WorkspaceID   int64  `json:"workspaceID"`
		ApiKeyName    string `json:"apiKeyName"`
		KeyType       string `json:"keyType"`
	}
	_ = json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result)
	parts := []string{}
	if result.UserName != "" {
		parts = append(parts, "user="+result.UserName)
	}
	if result.Email != "" {
		parts = append(parts, "email="+result.Email)
	}
	if result.WorkspaceName != "" {
		parts = append(parts, "workspace="+result.WorkspaceName)
	} else if result.WorkspaceID != 0 {
		parts = append(parts, fmt.Sprintf("workspaceID=%d", result.WorkspaceID))
	}
	if result.ApiKeyName != "" {
		parts = append(parts, "key="+result.ApiKeyName)
	}
	if result.KeyType != "" {
		parts = append(parts, "type="+result.KeyType)
	}
	if len(parts) == 0 {
		return "authenticated"
	}
	return strings.Join(parts, "; ")
}

func configMessage(baseURL string, profile config.Profile) string {
	parts := []string{"baseURL=" + baseURL}
	if profile.Lang != "" {
		parts = append(parts, "lang="+profile.Lang)
	}
	return strings.Join(parts, "; ")
}

func printDoctor(w io.Writer, checks []doctorCheck) {
	for _, check := range checks {
		prefix := "OK"
		if !check.OK {
			prefix = "FAIL"
		}
		fmt.Fprintf(w, "%-4s %-14s %s\n", prefix, check.Name, check.Message)
		if check.Hint != "" {
			fmt.Fprintf(w, "     Hint: %s\n", check.Hint)
		}
	}
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8 {
		return "***"
	}
	return value[:8] + "..."
}
