package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
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
	mqttAuthCreateCmd.Flags().Bool("cloud", false, "Use the Cloud v2 MQTT credential endpoint")
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
	cloud, _ := command.Flags().GetBool("cloud")
	endpoint := "/openapi/v1/mqtt-auth/create"
	if cloud {
		endpoint = "/openapi/v2/mqtt-auth/create"
	}
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
	if handled, err := writeMQTTAuthDryRun(command, endpoint, payload); handled {
		return err
	}
	c, err := cmdutil.NewConfiguredClient()
	if err != nil {
		return cmdutil.HandleCommandError(command.ErrOrStderr(), err, jsonMode)
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
		lock, err := store.Lock(saveName)
		if err != nil {
			return configCommandError(command, "failed to lock MQTT credential profile", err)
		}
		defer lock.Unlock()
		if err := store.Prepare(saveName); err != nil {
			return invalidArgumentCause(command, "--save", err.Error(), err)
		}
		broker, err = discoverMQTTBroker(command, c, brokerOverride, cloud, debug)
		if err != nil {
			return apiCommandError(command, "failed to discover MQTT broker before creating the credential", err)
		}
	}

	checker := notice.Start()
	resp, err := c.DoAPI(command.Context(), endpoint, "POST", cmdutil.JSONString(payload), debug)
	if err != nil {
		return mqttMutationFailure(command, "creation", err, false)
	}
	if err := cmdutil.CheckResponse(resp); err != nil {
		return mqttMutationFailure(command, "creation", err, true)
	}
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal([]byte(resp), &envelope) != nil || envelope.Code == nil || (*envelope.Code != 0 && *envelope.Code != 200) || len(envelope.Data) == 0 {
		return mqttCreateUnknown(command, "MQTT credential creation response is invalid; a credential may have been created")
	}
	var created mqttAuthCreateData
	if err := json.Unmarshal(envelope.Data, &created); err != nil {
		return mqttCreateUnknown(command, "MQTT credential creation response is invalid; a credential may have been created")
	}
	if created.ID <= 0 || created.ClientID == "" || created.Username == "" || created.Password == "" {
		return mqttCreateUnknown(command, "MQTT credential creation response is incomplete; a credential may have been created")
	}
	if store != nil {
		version := "v1"
		if cloud {
			version = "v2"
		}
		credential := mqttprofile.Credential{
			ID:                          created.ID,
			Name:                        created.Name,
			BaseURL:                     c.BaseURL(),
			APIVersion:                  version,
			Broker:                      broker,
			ClientID:                    created.ClientID,
			Username:                    created.Username,
			Password:                    created.Password,
			ClientIDRandomSuffixEnabled: created.ClientIDRandomSuffixEnabled,
			CreatedAt:                   time.Now().UTC(),
		}
		if err := store.Save(saveName, credential); err != nil {
			return mqttCreateUnknown(command, "MQTT credential ID "+fmt.Sprint(created.ID)+" was created remotely but could not be saved locally")
		}
	}

	stdout := command.OutOrStdout()
	if jsonMode && store != nil {
		var envelope map[string]json.RawMessage
		var data map[string]json.RawMessage
		if json.Unmarshal([]byte(resp), &envelope) != nil || json.Unmarshal(envelope["data"], &data) != nil || data == nil {
			return mqttCreateUnknown(command, "MQTT credential was created and saved, but its response could not be prepared")
		}
		for key := range data {
			if strings.EqualFold(key, "password") {
				delete(data, key)
			}
		}
		envelope["data"], _ = json.Marshal(data)
		redacted, _ := json.Marshal(envelope)
		resp = string(redacted)
	}
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

func mqttCreateUnknown(command *cobra.Command, message string) error {
	jsonMode, _ := command.Flags().GetBool("json")
	err := errs.New(errs.CategoryAPI, 0, message).WithExecution("unknown").WithHint("Inspect remote credentials and the local profile before attempting another creation.", "")
	return cmdutil.HandleCommandError(command.ErrOrStderr(), err, jsonMode)
}
