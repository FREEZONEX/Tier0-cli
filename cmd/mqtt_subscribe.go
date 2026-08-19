package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/FREEZONEX/Tier0-cli/internal/mqtttransport"
	"github.com/spf13/cobra"
)

var mqttSubscribeCmd = &cobra.Command{
	Use:   "subscribe",
	Short: "Subscribe to MQTT messages",
	Long:  "Subscribe to a Tier0 MQTT topic filter. Use --count or --timeout for bounded automation and AI agent runs.",
	RunE:  runMQTTSubscribe,
}

type mqttMessageOutput struct {
	Topic         string `json:"topic"`
	Payload       any    `json:"payload,omitempty"`
	PayloadBase64 string `json:"payloadBase64,omitempty"`
	Encoding      string `json:"encoding,omitempty"`
	QoS           byte   `json:"qos"`
	Retained      bool   `json:"retained"`
	Duplicate     bool   `json:"duplicate"`
	ReceivedAt    string `json:"receivedAt"`
}

func init() {
	mqttSubscribeCmd.Flags().StringP("topic", "t", "", "MQTT topic filter to subscribe to (required)")
	mqttSubscribeCmd.Flags().Int("qos", 0, "MQTT QoS level (0/1/2)")
	mqttSubscribeCmd.Flags().Int("count", 0, "Exit after receiving this many messages (0 means unlimited)")
	mqttSubscribeCmd.Flags().Duration("timeout", 0, "Exit after this duration (0 means no overall timeout)")
	mqttSubscribeCmd.Flags().String("format", "pretty", "Output format: pretty, ndjson, or raw")
	_ = mqttSubscribeCmd.MarkFlagRequired("topic")
	addMQTTConnectionFlags(mqttSubscribeCmd)
}

func runMQTTSubscribe(command *cobra.Command, args []string) error {
	topic, _ := command.Flags().GetString("topic")
	qosValue, _ := command.Flags().GetInt("qos")
	count, _ := command.Flags().GetInt("count")
	timeout, _ := command.Flags().GetDuration("timeout")
	format, _ := command.Flags().GetString("format")
	jsonMode, _ := command.Flags().GetBool("json")
	debug, _ := command.Flags().GetBool("debug")
	qos, err := mqtttransport.ParseQoS(qosValue)
	if err != nil {
		return invalidArgumentCause(command, "--qos", err.Error(), err)
	}
	if err := mqtttransport.ValidateSubscribeTopic(topic); err != nil {
		return invalidArgumentCause(command, "--topic", err.Error(), err)
	}
	if count < 0 {
		return invalidArgument(command, "--count", "--count cannot be negative")
	}
	if timeout < 0 {
		return invalidArgument(command, "--timeout", "--timeout cannot be negative")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if jsonMode {
		if command.Flags().Changed("format") && format != "ndjson" {
			return invalidArgument(command, "--json/--format", "--json requires --format ndjson")
		}
		format = "ndjson"
	}
	if format != "pretty" && format != "ndjson" && format != "raw" {
		return invalidArgument(command, "--format", "--format must be pretty, ndjson, or raw")
	}

	connection, credentialSource, err := resolveMQTTConnection(command, true)
	if err != nil {
		return configCommandError(command, "failed to resolve MQTT connection", err)
	}
	if connection.InsecureSkipVerify && !jsonMode {
		fmt.Fprintln(command.ErrOrStderr(), "warning: MQTT TLS certificate verification is disabled")
	}
	printMQTTDebug(command, "subscribe", connection.Broker, connection.ClientID, topic, credentialSource)

	ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	subscription, err := mqtttransport.Subscribe(ctx, connection, topic, qos)
	if err != nil {
		return handleMQTTTransportError(command, "subscribe", err)
	}
	defer subscription.Close()

	received := 0
	for {
		select {
		case message := <-subscription.Messages():
			if err := writeMQTTMessage(command, format, message); err != nil {
				return internalCommandError(command, "failed to write MQTT message", err)
			}
			received++
			if count > 0 && received >= count {
				return nil
			}
		case err := <-subscription.Errors():
			if debug && err != nil {
				fmt.Fprintf(command.ErrOrStderr(), "[debug] %v; waiting for MQTT reconnect\n", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func writeMQTTMessage(command *cobra.Command, format string, message mqtttransport.Message) error {
	stdout := command.OutOrStdout()
	switch format {
	case "raw":
		_, err := stdout.Write(message.Payload)
		return err
	case "ndjson":
		return json.NewEncoder(stdout).Encode(newMQTTMessageOutput(message))
	default:
		fmt.Fprintf(stdout, "[%s] %s qos=%d retained=%t\n", message.Received.Format(time.RFC3339Nano), message.Topic, message.QoS, message.Retained)
		var value any
		if json.Unmarshal(message.Payload, &value) == nil {
			pretty, _ := json.MarshalIndent(value, "", "  ")
			fmt.Fprintln(stdout, string(pretty))
		} else if utf8.Valid(message.Payload) {
			fmt.Fprintln(stdout, string(message.Payload))
		} else {
			fmt.Fprintln(stdout, base64.StdEncoding.EncodeToString(message.Payload))
		}
		return nil
	}
}

func newMQTTMessageOutput(message mqtttransport.Message) mqttMessageOutput {
	output := mqttMessageOutput{
		Topic: message.Topic, QoS: message.QoS, Retained: message.Retained, Duplicate: message.Duplicate,
		ReceivedAt: message.Received.Format(time.RFC3339Nano),
	}
	var value any
	if json.Unmarshal(message.Payload, &value) == nil {
		output.Payload = value
	} else if utf8.Valid(message.Payload) {
		output.Payload = string(message.Payload)
	} else {
		output.PayloadBase64 = base64.StdEncoding.EncodeToString(message.Payload)
		output.Encoding = "base64"
	}
	return output
}
