package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/client"
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
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
	profile, err := config.LoadProfile()
	if err != nil {
		return fmt.Errorf("load configured API origin: %w", err)
	}
	currentBaseURL := profile.BaseURL
	if currentBaseURL == "" {
		currentBaseURL = cmdutil.ResolveBaseURL("")
	}
	return validateMQTTProfileOrigin(profileName, profileBaseURL, currentBaseURL)
}

func validateMQTTProfileOrigin(profileName, profileBaseURL, currentBaseURL string) error {
	a, errA := url.Parse(profileBaseURL)
	b, errB := url.Parse(currentBaseURL)
	if errA != nil || errB != nil || a == nil || b == nil || a.User != nil || b.User != nil || a.RawQuery != "" || b.RawQuery != "" || a.Fragment != "" || b.Fragment != "" || !config.SameOrigin(profileBaseURL, currentBaseURL) || strings.TrimRight(a.EscapedPath(), "/") != strings.TrimRight(b.EscapedPath(), "/") {
		return fmt.Errorf("MQTT credential profile %q is not bound to the configured API origin", profileName)
	}
	return nil
}

func writeMQTTAuthDryRun(command *cobra.Command, endpoint string, body any) (bool, error) {
	dryRun, _ := command.Flags().GetBool("dry-run")
	if !dryRun {
		return false, nil
	}
	profile, err := config.LoadProfile()
	if err != nil {
		return true, configCommandError(command, "failed to load configured API origin", err)
	}
	base := profile.BaseURL
	if base == "" {
		base = cmdutil.ResolveBaseURL("")
	}
	preview := cmdutil.RequestPreview{Method: "POST", URL: strings.TrimRight(base, "/") + endpoint, Body: body}
	jsonMode, _ := command.Flags().GetBool("json")
	if err := cmdutil.WriteDryRun(command.OutOrStdout(), preview, jsonMode); err != nil {
		return true, internalCommandError(command, "failed to write dry-run output", err)
	}
	return true, nil
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

func discoverMQTTBroker(command *cobra.Command, c *client.Client, override string, cloud, debug bool) (string, error) {
	if strings.TrimSpace(override) != "" {
		return mqtttransport.NormalizeBroker(override)
	}
	endpoint := "/openapi/v1/info"
	if cloud {
		endpoint = "/openapi/v2/info"
	}
	resp, err := c.DoAPI(command.Context(), endpoint, "POST", "{}", debug)
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
