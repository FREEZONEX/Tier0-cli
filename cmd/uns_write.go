package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsWriteCmd = &cobra.Command{
	Use:   "write",
	Short: "Write value to a UNS topic",
	Long:  "Write a value to one or more UNS topics.\n\nExamples:\n  tier0 uns write --topic demo --value '{\"temp\":25}'\n  tier0 uns write --topic demo --file payload.json\n  tier0 uns write --topic sensor1 --value '{\"on\":true}' --qos 1 --retain",

	RunE: runUnsWrite,
}

func init() {
	unsWriteCmd.Flags().StringP("topic", "t", "",
		"Topic name to write to (required)")
	unsWriteCmd.Flags().StringP("value", "v", "",
		"JSON value string (mutually exclusive with --file)")
	unsWriteCmd.Flags().StringP("file", "f", "",
		"Read value from file (mutually exclusive with --value)")
	unsWriteCmd.Flags().Int("qos", 0,
		"MQTT QoS level (0/1/2)")
	unsWriteCmd.Flags().Bool("retain", false,
		"Set MQTT retain flag")
	unsWriteCmd.MarkFlagRequired("topic")
	addDryRunFlag(unsWriteCmd)
}

func runUnsWrite(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topic, _ := cmd.Flags().GetString("topic")
	value, _ := cmd.Flags().GetString("value")
	file, _ := cmd.Flags().GetString("file")
	qos, _ := cmd.Flags().GetInt("qos")
	retain, _ := cmd.Flags().GetBool("retain")

	if cmd.Flags().Changed("value") && cmd.Flags().Changed("file") {
		return invalidArgument(cmd, "--value/--file", "--value and --file are mutually exclusive")
	}
	if qos < 0 || qos > 2 {
		return invalidArgument(cmd, "--qos", "--qos must be 0, 1, or 2")
	}

	if file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return fileIOError(cmd, "--file", "read value file", file, err)
		}
		value = string(raw)
	}
	if value == "" {
		return invalidArgument(cmd, "--value/--file", "specify a value via --value '<json>' or --file <path>")
	}

	var valueObj any
	if err := json.Unmarshal([]byte(value), &valueObj); err != nil {
		param := "--value"
		if file != "" {
			param = "--file"
		}
		return invalidArgumentCause(cmd, param, "value must be valid JSON: "+err.Error(), err)
	}

	writeItem := map[string]any{
		"topic": topic,
		"value": valueObj,
	}

	payload := map[string]any{
		"writes": []any{writeItem},
	}
	if qos != 0 {
		payload["qos"] = qos
	}
	if retain {
		payload["retain"] = true
	}
	if handled, err := writeDryRun(cmd, "POST", "/openapi/v1/uns/write", payload); handled {
		return err
	}

	checker := notice.Start()
	body := cmdutil.JSONString(payload)

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/write", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(stdout, "Written: %s\n", topic)
	}
	return nil
}
