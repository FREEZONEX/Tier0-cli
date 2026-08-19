package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/mqtttransport"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var mqttPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish a message over MQTT",
	Long:  "Publish arbitrary bytes or JSON to a Tier0 MQTT topic. Use --file for complex JSON arrays and objects.",
	RunE:  runMQTTPublish,
}

func init() {
	mqttPublishCmd.Flags().StringP("topic", "t", "", "MQTT topic to publish to (required)")
	mqttPublishCmd.Flags().StringP("message", "m", "", "Message text or inline JSON")
	mqttPublishCmd.Flags().StringP("file", "f", "", "Read the message bytes from a file")
	mqttPublishCmd.Flags().Bool("stdin", false, "Read the message bytes from stdin")
	mqttPublishCmd.Flags().Bool("json-message", false, "Require the message to be valid JSON")
	mqttPublishCmd.Flags().Int("qos", 0, "MQTT QoS level (0/1/2)")
	mqttPublishCmd.Flags().Bool("retain", false, "Set the MQTT retain flag")
	mqttPublishCmd.Flags().Duration("timeout", 30*time.Second, "Overall publish timeout")
	_ = mqttPublishCmd.MarkFlagRequired("topic")
	addMQTTConnectionFlags(mqttPublishCmd)
	addDryRunFlag(mqttPublishCmd)
}

func runMQTTPublish(command *cobra.Command, args []string) error {
	topic, _ := command.Flags().GetString("topic")
	qosValue, _ := command.Flags().GetInt("qos")
	retain, _ := command.Flags().GetBool("retain")
	timeout, _ := command.Flags().GetDuration("timeout")
	jsonMessage, _ := command.Flags().GetBool("json-message")
	jsonMode, _ := command.Flags().GetBool("json")
	qos, err := mqtttransport.ParseQoS(qosValue)
	if err != nil {
		return invalidArgumentCause(command, "--qos", err.Error(), err)
	}
	if err := mqtttransport.ValidatePublishTopic(topic); err != nil {
		return invalidArgumentCause(command, "--topic", err.Error(), err)
	}
	if timeout <= 0 {
		return invalidArgument(command, "--timeout", "--timeout must be greater than zero")
	}
	payload, err := readMQTTPublishPayload(command)
	if err != nil {
		return err
	}
	if jsonMessage && !json.Valid(payload) {
		return invalidArgument(command, "--message/--file/--stdin", "MQTT message must be valid JSON when --json-message is set")
	}

	connection, credentialSource, err := resolveMQTTConnection(command, false)
	if err != nil {
		return configCommandError(command, "failed to resolve MQTT connection", err)
	}
	preview := mqttPublishPreview{
		Action: "publish", Broker: connection.Broker, Topic: topic, QoS: int(qos), Retain: retain,
		PayloadBytes: len(payload), CredentialSource: credentialSource,
	}
	if handled, err := writeMQTTPublishDryRun(command, preview); handled {
		return err
	}
	if connection.InsecureSkipVerify && !jsonMode {
		fmt.Fprintln(command.ErrOrStderr(), "warning: MQTT TLS certificate verification is disabled")
	}
	printMQTTDebug(command, "publish", connection.Broker, connection.ClientID, topic, credentialSource)

	ctx, cancel := context.WithTimeout(command.Context(), timeout)
	defer cancel()
	if err := mqtttransport.Publish(ctx, connection, topic, qos, retain, payload); err != nil {
		return handleMQTTTransportError(command, "publish", err)
	}

	result := cmdutil.JSONString(map[string]any{
		"ok": true,
		"data": map[string]any{
			"published": true, "topic": topic, "qos": qos, "retain": retain, "payloadBytes": len(payload),
		},
	})
	checker := notice.Start()
	checker.Emit(result, jsonMode, command.OutOrStdout(), command.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(command.OutOrStdout(), "Published: %s (qos=%d retain=%t bytes=%d)\n", topic, qos, retain, len(payload))
	}
	return nil
}

func readMQTTPublishPayload(command *cobra.Command) ([]byte, error) {
	sources := 0
	if command.Flags().Changed("message") {
		sources++
	}
	if command.Flags().Changed("file") {
		sources++
	}
	stdin, _ := command.Flags().GetBool("stdin")
	if stdin {
		sources++
	}
	if sources != 1 {
		return nil, invalidArgument(command, "--message/--file/--stdin", "specify exactly one of --message, --file, or --stdin")
	}
	if command.Flags().Changed("message") {
		message, _ := command.Flags().GetString("message")
		return []byte(message), nil
	}
	if command.Flags().Changed("file") {
		path, _ := command.Flags().GetString("file")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fileIOError(command, "--file", "read MQTT message file", path, err)
		}
		return data, nil
	}
	data, err := io.ReadAll(command.InOrStdin())
	if err != nil {
		return nil, fileIOError(command, "--stdin", "read MQTT message from stdin", "stdin", err)
	}
	return data, nil
}
