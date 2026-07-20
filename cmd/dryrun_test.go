package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type dryRunTestEnvelope struct {
	OK     bool `json:"ok"`
	DryRun bool `json:"dry_run"`
	Data   struct {
		API []struct {
			Method string         `json:"method"`
			URL    string         `json:"url"`
			Body   map[string]any `json:"body"`
		} `json:"api"`
	} `json:"data"`
}

func TestMutationCommandsShareDryRunContract(t *testing.T) {
	t.Setenv("TIER0_BASE_URL", "https://example.test/")

	tests := []struct {
		name     string
		args     []string
		method   string
		endpoint string
	}{
		{
			name:     "raw api",
			args:     []string{"api", "/custom", "--method", "PATCH", "--body", `{"value":1}`, "--dry-run", "--json"},
			method:   "PATCH",
			endpoint: "/custom",
		},
		{
			name:     "uns write",
			args:     []string{"uns", "write", "--topic", "Plant/Metric/T", "--value", `{"value":1}`, "--dry-run", "--json"},
			method:   "POST",
			endpoint: "/openapi/v1/uns/write",
		},
		{
			name:     "uns create",
			args:     []string{"uns", "create", "--topic", "Plant/Metric/T", "--type", "topic", "--dry-run", "--json"},
			method:   "POST",
			endpoint: "/openapi/v1/uns/create",
		},
		{
			name:     "uns update",
			args:     []string{"uns", "update", "--path", "Plant/Metric/T", "--description", "updated", "--dry-run", "--json"},
			method:   "POST",
			endpoint: "/openapi/v1/uns/update",
		},
		{
			name:     "uns delete without yes",
			args:     []string{"uns", "delete", "--path", "Plant/Metric/T", "--dry-run", "--json"},
			method:   "POST",
			endpoint: "/openapi/v1/uns/delete",
		},
		{
			name:     "uns restore without yes",
			args:     []string{"uns", "restore", "--path", "Plant/Metric/T", "--dry-run", "--json"},
			method:   "POST",
			endpoint: "/openapi/v1/uns/restore",
		},
		{
			name:     "flow create",
			args:     []string{"flow", "create", "--name", "demo", "--source", "--template", `[]`, "--dry-run", "--json"},
			method:   "POST",
			endpoint: "/openapi/v1/flow/create",
		},
		{
			name:     "flow update",
			args:     []string{"flow", "update", "--id", "7", "--name", "updated", "--dry-run", "--json"},
			method:   "POST",
			endpoint: "/openapi/v1/flow/update",
		},
		{
			name:     "flow delete without yes",
			args:     []string{"flow", "delete", "--id", "7", "--dry-run", "--json"},
			method:   "POST",
			endpoint: "/openapi/v1/flow/delete",
		},
		{
			name:     "flow deploy without yes",
			args:     []string{"flow", "deploy", "--id", "7", "--flows-json", `[]`, "--dry-run", "--json"},
			method:   "POST",
			endpoint: "/openapi/v1/flow/deploy",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := executeRootForTest(test.args...)
			if err != nil {
				t.Fatalf("execute error: %v\nstderr: %s", err, stderr)
			}
			if stderr != "" {
				t.Fatalf("dry-run wrote stderr: %s", stderr)
			}

			var envelope dryRunTestEnvelope
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, stdout)
			}
			if !envelope.OK || !envelope.DryRun || len(envelope.Data.API) != 1 {
				t.Fatalf("unexpected dry-run envelope: %#v", envelope)
			}
			call := envelope.Data.API[0]
			if call.Method != test.method {
				t.Fatalf("method = %q, want %q", call.Method, test.method)
			}
			if call.URL != "https://example.test"+test.endpoint {
				t.Fatalf("url = %q, want %q", call.URL, "https://example.test"+test.endpoint)
			}
		})
	}
}

func TestCommandValidationErrorsAreTyped(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		param string
	}{
		{
			name:  "qos range",
			args:  []string{"uns", "write", "--topic", "demo", "--value", `1`, "--qos", "3", "--json"},
			param: "--qos",
		},
		{
			name:  "flow type conflict",
			args:  []string{"flow", "create", "--name", "demo", "--source", "--event", "--json"},
			param: "--source/--event",
		},
		{
			name:  "favorite conflict",
			args:  []string{"flow", "update", "--id", "7", "--favorite", "--unfavorite", "--json"},
			param: "--favorite/--unfavorite",
		},
		{
			name:  "strict api JSON",
			args:  []string{"api", "/custom", "--body", `{path:/}`, "--json"},
			param: "--body",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := executeRootForTest(test.args...)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if stdout != "" {
				t.Fatalf("validation wrote stdout: %s", stdout)
			}
			var envelope struct {
				OK    bool `json:"ok"`
				Error struct {
					Type    string `json:"type"`
					Subtype string `json:"subtype"`
					Param   string `json:"param"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &envelope); err != nil {
				t.Fatalf("invalid error envelope: %v\n%s", err, stderr)
			}
			if envelope.OK || envelope.Error.Type != "validation" || envelope.Error.Subtype != "invalid_argument" {
				t.Fatalf("unexpected error envelope: %#v", envelope)
			}
			if envelope.Error.Param != test.param {
				t.Fatalf("param = %q, want %q", envelope.Error.Param, test.param)
			}
		})
	}
}

func TestExplicitEmptyUpdateValuesArePreserved(t *testing.T) {
	t.Setenv("TIER0_BASE_URL", "https://example.test/")
	tests := []struct {
		name string
		args []string
		key  string
	}{
		{
			name: "clear flow description",
			args: []string{"flow", "update", "--id", "7", "--desc", "", "--dry-run", "--json"},
			key:  "description",
		},
		{
			name: "clear UNS description",
			args: []string{"uns", "update", "--path", "Plant/Metric/T", "--description", "", "--dry-run", "--json"},
			key:  "description",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := executeRootForTest(test.args...)
			if err != nil {
				t.Fatalf("execute error: %v\nstderr: %s", err, stderr)
			}
			var envelope dryRunTestEnvelope
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, stdout)
			}
			body := envelope.Data.API[0].Body
			value, ok := body[test.key]
			if !ok || value != "" {
				t.Fatalf("body[%q] = %#v, want explicit empty string; body=%#v", test.key, value, body)
			}
		})
	}
}

func executeRootForTest(args ...string) (stdout, stderr string, err error) {
	resetCommandFlags(rootCmd)
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	rootCmd.SetArgs(nil)
	resetCommandFlags(rootCmd)
	return out.String(), errOut.String(), err
}

func resetCommandFlags(command *cobra.Command) {
	resetFlagSet(command.Flags())
	resetFlagSet(command.PersistentFlags())
	for _, child := range command.Commands() {
		resetCommandFlags(child)
	}
}

func resetFlagSet(flags *pflag.FlagSet) {
	flags.VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
}
