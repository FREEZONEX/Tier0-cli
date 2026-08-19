package cmd

import (
	"fmt"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/mqttprofile"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var mqttAuthDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a revocable MQTT credential",
	Long:  "Delete a remote MQTT credential by saved profile or ID. A saved local profile is removed only after the server confirms deletion.",
	RunE:  runMQTTAuthDelete,
}

func init() {
	mqttAuthDeleteCmd.Flags().String("credential", "", "Saved MQTT credential profile")
	mqttAuthDeleteCmd.Flags().Int64("id", 0, "Remote MQTT credential ID")
	mqttAuthDeleteCmd.Flags().BoolP("yes", "y", false, "Confirm credential deletion")
	addDryRunFlag(mqttAuthDeleteCmd)
}

func runMQTTAuthDelete(command *cobra.Command, args []string) error {
	profileName, _ := command.Flags().GetString("credential")
	id, _ := command.Flags().GetInt64("id")
	confirmed, _ := command.Flags().GetBool("yes")
	jsonMode, _ := command.Flags().GetBool("json")
	debug, _ := command.Flags().GetBool("debug")
	if strings.TrimSpace(profileName) != "" && id != 0 {
		return invalidArgument(command, "--credential/--id", "--credential and --id are mutually exclusive")
	}
	var store *mqttprofile.Store
	if strings.TrimSpace(profileName) != "" {
		var err error
		store, err = mqttCredentialStore()
		if err != nil {
			return configCommandError(command, "failed to resolve MQTT credential store", err)
		}
		credential, err := store.Load(profileName)
		if err != nil {
			return invalidArgumentCause(command, "--credential", err.Error(), err)
		}
		if err := validateMQTTProfileBaseURL(profileName, credential.BaseURL); err != nil {
			return configCommandError(command, "refusing to delete an MQTT credential from a different Tier0 instance", err)
		}
		id = credential.ID
	}
	if id <= 0 {
		return invalidArgument(command, "--credential/--id", "specify --credential <profile> or --id <positive-id>")
	}
	payload := map[string]any{"id": id}
	if handled, err := writeDryRun(command, "POST", "/openapi/v1/mqtt-auth/delete", payload); handled {
		return err
	}
	if err := highrisk.Guard(confirmed, "mqtt auth delete", fmt.Sprintf("Delete MQTT credential ID %d and prevent future broker connections.", id)); err != nil {
		return err
	}

	checker := notice.Start()
	resp, err := cmdutil.DoAPI(command.Context(), "/openapi/v1/mqtt-auth/delete", "POST", cmdutil.JSONString(payload), debug)
	if err != nil {
		return cmdutil.HandleCommandError(command.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckResponse(resp); err != nil {
		return cmdutil.HandleCommandError(command.ErrOrStderr(), err, jsonMode)
	}
	if store != nil {
		if err := store.Delete(profileName); err != nil {
			return configCommandError(command, "remote credential was deleted but the local profile could not be removed", err)
		}
	}
	stdout := command.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, command.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(stdout, "Deleted MQTT credential ID %d\n", id)
		if profileName != "" {
			fmt.Fprintf(stdout, "Removed local profile: %s\n", profileName)
		}
	}
	return nil
}
