package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/mqttprofile"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var mqttAuthCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a revocable MQTT credential",
	Long:  "Create an MQTT credential. The password is returned only by this create call. Use --save to store it locally for publish and subscribe commands.",
	RunE:  runMQTTAuthCreate,
}

type mqttAuthCreateData struct {
	ID                          int64  `json:"id"`
	Name                        string `json:"name"`
	ClientID                    string `json:"clientID"`
	Username                    string `json:"username"`
	Password                    string `json:"password"`
	ClientIDRandomSuffixEnabled bool   `json:"clientIDRandomSuffixEnabled"`
}

func init() {
	mqttAuthCreateCmd.Flags().String("name", "", "MQTT credential name (required)")
	mqttAuthCreateCmd.Flags().String("description", "", "MQTT credential description")
	mqttAuthCreateCmd.Flags().Bool("random-suffix", true, "Allow a random runtime suffix on the MQTT client ID")
	mqttAuthCreateCmd.Flags().String("save", "", "Save the credential under a local profile name")
	mqttAuthCreateCmd.Flags().String("broker", "", "MQTT broker URL to save instead of service discovery")
	_ = mqttAuthCreateCmd.MarkFlagRequired("name")
	addDryRunFlag(mqttAuthCreateCmd)
}

func runMQTTAuthCreate(command *cobra.Command, args []string) error {
	name, _ := command.Flags().GetString("name")
	description, _ := command.Flags().GetString("description")
	randomSuffix, _ := command.Flags().GetBool("random-suffix")
	saveName, _ := command.Flags().GetString("save")
	brokerOverride, _ := command.Flags().GetString("broker")
	jsonMode, _ := command.Flags().GetBool("json")
	debug, _ := command.Flags().GetBool("debug")
	name = strings.TrimSpace(name)
	if name == "" {
		return invalidArgument(command, "--name", "MQTT credential name cannot be empty")
	}
	if command.Flags().Changed("broker") && strings.TrimSpace(saveName) == "" {
		return invalidArgument(command, "--broker", "--broker is only used with --save")
	}

	payload := map[string]any{
		"name":                        name,
		"clientIDRandomSuffixEnabled": randomSuffix,
	}
	if strings.TrimSpace(description) != "" {
		payload["description"] = strings.TrimSpace(description)
	}
	if handled, err := writeDryRun(command, "POST", "/openapi/v1/mqtt-auth/create", payload); handled {
		return err
	}

	var (
		store  *mqttprofile.Store
		broker string
	)
	if strings.TrimSpace(saveName) != "" {
		var err error
		store, err = mqttCredentialStore()
		if err != nil {
			return configCommandError(command, "failed to resolve MQTT credential store", err)
		}
		if err := store.Prepare(saveName); err != nil {
			return invalidArgumentCause(command, "--save", err.Error(), err)
		}
		broker, err = discoverMQTTBroker(command, brokerOverride, debug)
		if err != nil {
			return apiCommandError(command, "failed to discover MQTT broker before creating the credential", err)
		}
	}

	checker := notice.Start()
	resp, err := cmdutil.DoAPI(command.Context(), "/openapi/v1/mqtt-auth/create", "POST", cmdutil.JSONString(payload), debug)
	if err != nil {
		return cmdutil.HandleCommandError(command.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckResponse(resp); err != nil {
		return cmdutil.HandleCommandError(command.ErrOrStderr(), err, jsonMode)
	}
	var created mqttAuthCreateData
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &created); err != nil {
		return internalCommandError(command, "failed to parse MQTT credential response", err)
	}
	if created.ID <= 0 || created.ClientID == "" || created.Username == "" || created.Password == "" {
		return internalCommandError(command, "MQTT credential response is incomplete", nil)
	}
	if store != nil {
		credential := mqttprofile.Credential{
			ID:                          created.ID,
			Name:                        created.Name,
			BaseURL:                     cmdutil.ResolveBaseURL(""),
			Broker:                      broker,
			ClientID:                    created.ClientID,
			Username:                    created.Username,
			Password:                    created.Password,
			ClientIDRandomSuffixEnabled: created.ClientIDRandomSuffixEnabled,
			CreatedAt:                   time.Now().UTC(),
		}
		if err := store.Save(saveName, credential); err != nil {
			return configCommandError(command, "credential was created remotely but could not be saved locally; delete remote credential ID "+fmt.Sprint(created.ID), err)
		}
	}

	stdout := command.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, command.ErrOrStderr())
	if jsonMode {
		return nil
	}
	fmt.Fprintf(stdout, "Created MQTT credential: %s (ID %d)\n", created.Name, created.ID)
	fmt.Fprintf(stdout, "ClientID: %s\nUsername: %s\n", created.ClientID, created.Username)
	if store != nil {
		fmt.Fprintf(stdout, "Saved profile: %s\nBroker: %s\nPassword: stored securely (not displayed)\n", saveName, broker)
	} else {
		fmt.Fprintf(stdout, "Password: %s\n\nSave this password now; it cannot be returned again by the OpenAPI.\n", created.Password)
	}
	return nil
}
