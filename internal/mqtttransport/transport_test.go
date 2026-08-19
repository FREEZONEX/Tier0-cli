package mqtttransport

import (
	"fmt"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

func TestNormalizeBroker(t *testing.T) {
	tests := map[string]string{
		"mqtt.example":                   "ssl://mqtt.example:8883",
		"mqtt.example:1883":              "tcp://mqtt.example:1883",
		"mqtt.example:8084":              "wss://mqtt.example:8084/mqtt",
		"mqtt://mqtt.example":            "tcp://mqtt.example:1883",
		"mqtts://mqtt.example":           "ssl://mqtt.example:8883",
		"wss://mqtt.example:8084/custom": "wss://mqtt.example:8084/custom",
	}
	for input, want := range tests {
		got, err := NormalizeBroker(input)
		if err != nil {
			t.Fatalf("NormalizeBroker(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeBroker(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeBrokerRejectsEmbeddedCredentialsAndBadPorts(t *testing.T) {
	for _, broker := range []string{"ssl://user:secret@mqtt.example:8883", "ssl://mqtt.example:70000", "ssl://mqtt.example:bad"} {
		if _, err := NormalizeBroker(broker); err == nil {
			t.Fatalf("NormalizeBroker(%q) succeeded", broker)
		}
	}
}

func TestTopicValidation(t *testing.T) {
	for _, topic := range []string{"Vision/cam1/State/Result", "Plant/+/Metric/Temperature", "Plant/#"} {
		if err := ValidateSubscribeTopic(topic); err != nil {
			t.Fatalf("valid subscribe topic %q rejected: %v", topic, err)
		}
	}
	for _, topic := range []string{"Plant/a+", "Plant/#/x", "Plant/x#"} {
		if err := ValidateSubscribeTopic(topic); err == nil {
			t.Fatalf("invalid subscribe topic %q accepted", topic)
		}
	}
	if err := ValidatePublishTopic("Vision/cam1/State/Result"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePublishTopic("Vision/+/State/Result"); err == nil {
		t.Fatal("publish wildcard accepted")
	}
}

func TestRuntimeClientIDSuffix(t *testing.T) {
	base, err := RuntimeClientID("client", false)
	if err != nil || base != "client" {
		t.Fatalf("fixed client ID = %q, %v", base, err)
	}
	first, err := RuntimeClientID("client", true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RuntimeClientID("client", true)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) != len("client-")+8 {
		t.Fatalf("unexpected random client IDs: %q %q", first, second)
	}
}

func TestIsAuthenticationError(t *testing.T) {
	if !IsAuthenticationError(fmt.Errorf("connect: %w", packets.ErrorRefusedNotAuthorised)) {
		t.Fatal("wrapped authorization error was not classified")
	}
	if IsAuthenticationError(fmt.Errorf("connection reset")) {
		t.Fatal("network error was classified as authentication")
	}
}
