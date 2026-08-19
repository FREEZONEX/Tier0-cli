package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
	"github.com/FREEZONEX/Tier0-cli/internal/mqttprofile"
	"github.com/FREEZONEX/Tier0-cli/internal/mqtttransport"
	"github.com/spf13/cobra"
)

var mqttCredentialStore = mqttprofile.DefaultStore

func addMQTTConnectionFlags(command *cobra.Command) {
	command.Flags().String("credential", "", "Saved MQTT credential profile")
	command.Flags().String("broker", "", "Override MQTT broker URL")
	command.Flags().String("ca-file", "", "PEM CA certificate for a private MQTT broker")
	command.Flags().String("tls-server-name", "", "Override the MQTT TLS server name")
	command.Flags().Bool("insecure-skip-verify", false, "Skip MQTT TLS certificate verification (unsafe)")
	command.Flags().Duration("connect-timeout", 15*time.Second, "MQTT connection timeout")
}

func resolveMQTTConnection(command *cobra.Command, autoReconnect bool) (mqtttransport.Connection, string, error) {
	profileName, _ := command.Flags().GetString("credential")
	brokerOverride, _ := command.Flags().GetString("broker")
	caFile, _ := command.Flags().GetString("ca-file")
	tlsServerName, _ := command.Flags().GetString("tls-server-name")
	insecure, _ := command.Flags().GetBool("insecure-skip-verify")
	connectTimeout, _ := command.Flags().GetDuration("connect-timeout")
	if connectTimeout <= 0 {
		return mqtttransport.Connection{}, "", fmt.Errorf("--connect-timeout must be greater than zero")
	}

	var (
		broker       string
		clientID     string
		username     string
		password     string
		randomSuffix bool
		source       string
	)
	if strings.TrimSpace(profileName) != "" {
		store, err := mqttCredentialStore()
		if err != nil {
			return mqtttransport.Connection{}, "", err
		}
		credential, err := store.Load(profileName)
		if err != nil {
			return mqtttransport.Connection{}, "", err
		}
		if err := validateMQTTProfileBaseURL(profileName, credential.BaseURL); err != nil {
			return mqtttransport.Connection{}, "", err
		}
		broker = credential.Broker
		clientID = credential.ClientID
		username = credential.Username
		password = credential.Password
		randomSuffix = credential.ClientIDRandomSuffixEnabled
		source = profileName
	} else {
		broker = os.Getenv("TIER0_MQTT_BROKER")
		clientID = os.Getenv("TIER0_MQTT_CLIENT_ID")
		username = os.Getenv("TIER0_MQTT_USERNAME")
		password = os.Getenv("TIER0_MQTT_PASSWORD")
		randomSuffix = strings.EqualFold(os.Getenv("TIER0_MQTT_RANDOM_SUFFIX"), "true") || os.Getenv("TIER0_MQTT_RANDOM_SUFFIX") == "1"
		source = "environment"
		if broker == "" || clientID == "" || username == "" || password == "" {
			return mqtttransport.Connection{}, "", fmt.Errorf("specify --credential <profile>, or set TIER0_MQTT_BROKER, TIER0_MQTT_CLIENT_ID, TIER0_MQTT_USERNAME, and TIER0_MQTT_PASSWORD")
		}
	}
	if strings.TrimSpace(brokerOverride) != "" {
		broker = brokerOverride
	}
	normalizedBroker, err := mqtttransport.NormalizeBroker(broker)
	if err != nil {
		return mqtttransport.Connection{}, "", err
	}
	runtimeClientID, err := mqtttransport.RuntimeClientID(clientID, randomSuffix)
	if err != nil {
		return mqtttransport.Connection{}, "", err
	}
	return mqtttransport.Connection{
		Broker:             normalizedBroker,
		ClientID:           runtimeClientID,
		Username:           username,
		Password:           password,
		CAFile:             caFile,
		TLSServerName:      tlsServerName,
		InsecureSkipVerify: insecure,
		ConnectTimeout:     connectTimeout,
		AutoReconnect:      autoReconnect,
	}, source, nil
}

func validateMQTTProfileBaseURL(profileName, profileBaseURL string) error {
	currentBaseURL := strings.TrimRight(cmdutil.ResolveBaseURL(""), "/")
	profileBaseURL = strings.TrimRight(strings.TrimSpace(profileBaseURL), "/")
	if profileBaseURL != "" && !strings.EqualFold(currentBaseURL, profileBaseURL) {
		return fmt.Errorf("MQTT credential profile %q belongs to %s, but the CLI is configured for %s", profileName, profileBaseURL, currentBaseURL)
	}
	return nil
}

func handleMQTTTransportError(command *cobra.Command, action string, err error) error {
	category := errs.CategoryNetwork
	if mqtttransport.IsAuthenticationError(err) {
		category = errs.CategoryAuthentication
	}
	cliErr := errs.New(category, 0, "MQTT "+action+" failed: "+err.Error()).
		WithHint("Check the broker, TLS settings, and MQTT credential.", "tier0 mqtt auth create --name cli --save cli").
		WithCause(err)
	return handleCLIError(command, cliErr)
}

func discoverMQTTBroker(command *cobra.Command, override string, debug bool) (string, error) {
	if strings.TrimSpace(override) != "" {
		return mqtttransport.NormalizeBroker(override)
	}
	resp, err := cmdutil.DoAPI(command.Context(), "/openapi/v1/info", "POST", "{}", debug)
	if err != nil {
		return "", err
	}
	if err := cmdutil.CheckResponse(resp); err != nil {
		return "", err
	}
	var result struct {
		MqttBroker string `json:"mqttBroker"`
		MQTT       struct {
			TCPURL string `json:"tcpUrl"`
			URL    string `json:"url"`
		} `json:"mqtt"`
	}
	if err := json.Unmarshal([]byte(cmdutil.ExtractData(resp)), &result); err != nil {
		return "", fmt.Errorf("parse Tier0 MQTT broker info: %w", err)
	}
	broker := result.MQTT.TCPURL
	if broker == "" {
		broker = result.MQTT.URL
	}
	if broker == "" {
		broker = result.MqttBroker
	}
	if broker == "" {
		return "", fmt.Errorf("Tier0 service info did not return an MQTT broker; specify --broker")
	}
	return mqtttransport.NormalizeBroker(broker)
}

func printMQTTDebug(command *cobra.Command, action, broker, clientID, topic, credentialSource string) {
	debug, _ := command.Flags().GetBool("debug")
	if !debug {
		return
	}
	fmt.Fprintf(command.ErrOrStderr(), "[debug] MQTT %s broker=%s clientID=%s topic=%s credential=%s\n", action, broker, clientID, topic, credentialSource)
}
