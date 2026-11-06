package cmd

import (
	"encoding/json"
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
	mqttAuthDeleteCmd.Flags().Bool("cloud", false, "Use the Cloud v2 MQTT credential endpoint")
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
	cloud, _ := command.Flags().GetBool("cloud")
	dryRun, _ := command.Flags().GetBool("dry-run")
	endpoint := "/openapi/v1/mqtt-auth/delete"
	if cloud {
		endpoint = "/openapi/v2/mqtt-auth/delete"
	}
	if strings.TrimSpace(profileName) != "" && id != 0 {
		return invalidArgument(command, "--credential/--id", "--credential and --id are mutually exclusive")
	}
	var store *mqttprofile.Store
	var profileOrigin string
	var original mqttprofile.Credential
	if strings.TrimSpace(profileName) != "" {
		var err error
		store, err = mqttCredentialStore()
		if err != nil {
			return configCommandError(command, "failed to resolve MQTT credential store", err)
		}
		if !dryRun {
			lock, err := store.Lock(profileName)
			if err != nil {
				return configCommandError(command, "failed to lock MQTT credential profile", err)
			}
			defer lock.Unlock()
		}
		credential, err := store.Load(profileName)
		if err != nil {
			return invalidArgumentCause(command, "--credential", err.Error(), err)
		}
		if err := validateMQTTProfileBaseURL(profileName, credential.BaseURL); err != nil {
			return configCommandError(command, "refusing to delete an MQTT credential from a different Tier0 instance", err)
		}
		profileOrigin = credential.BaseURL
		original = credential
		switch credential.APIVersion {
		case "v1", "v2":
			profileCloud := credential.APIVersion == "v2"
			if command.Flags().Changed("cloud") && cloud != profileCloud {
				return invalidArgument(command, "--cloud", "does not match the saved credential API version")
			}
			if profileCloud {
				endpoint = "/openapi/v2/mqtt-auth/delete"
			} else {
				endpoint = "/openapi/v1/mqtt-auth/delete"
			}
		case "":
			if !command.Flags().Changed("cloud") {
				return invalidArgument(command, "--cloud", "legacy credential has no API version; specify --cloud or --cloud=false")
			}
		default:
			return invalidArgument(command, "--credential", "saved credential has an unknown API version")
		}
		id = credential.ID
	}
	if id <= 0 {
		return invalidArgument(command, "--credential/--id", "specify --credential <profile> or --id <positive-id>")
	}
	payload := map[string]any{"id": id}
	if handled, err := writeMQTTAuthDryRun(command, endpoint, payload); handled {
		return err
	}
	if err := highrisk.Guard(confirmed, "mqtt auth delete", fmt.Sprintf("Delete MQTT credential ID %d and prevent future broker connections.", id)); err != nil {
		return err
	}
	c, err := cmdutil.NewConfiguredClient()
	if err != nil {
		return cmdutil.HandleCommandError(command.ErrOrStderr(), err, jsonMode)
	}
	if store != nil {
		if err := validateMQTTProfileOrigin(profileName, profileOrigin, c.BaseURL()); err != nil {
			return configCommandError(command, "refusing to delete an MQTT credential from a different Tier0 instance", err)
		}
	}

	checker := notice.Start()
	resp, err := c.DoAPI(command.Context(), endpoint, "POST", cmdutil.JSONString(payload), debug)
	if err != nil {
		return mqttMutationFailure(command, "deletion", err, false)
	}
	if err := cmdutil.CheckResponse(resp); err != nil {
		return mqttMutationFailure(command, "deletion", err, true)
	}
	var result struct {
		Code *int `json:"code"`
		Data struct {
			Success *bool `json:"success"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(resp), &result) != nil || result.Code == nil || (*result.Code != 0 && *result.Code != 200) || result.Data.Success == nil || !*result.Data.Success {
		return mqttMutationFailure(command, "deletion", nil, true)
	}
	if store != nil {
		if err := store.DeleteIfUnchanged(profileName, original); err != nil {
			return configCommandError(command, "remote credential was deleted but the local profile changed or could not be removed; inspect it before proceeding", err)
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
