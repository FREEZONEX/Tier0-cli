package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/mqttprofile"
	"github.com/FREEZONEX/Tier0-cli/internal/mqtttransport"
)

func TestMQTTAuthCreateDryRun(t *testing.T) {
	t.Setenv("TIER0_BASE_URL", "https://example.test/")
	stdout, stderr, err := executeRootForTest(
		"mqtt", "auth", "create", "--name", "agent", "--description", "CLI agent",
		"--random-suffix=true", "--dry-run", "--json",
	)
	if err != nil {
		t.Fatalf("execute error: %v\nstderr: %s", err, stderr)
	}
	var envelope dryRunTestEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	call := envelope.Data.API[0]
	if call.URL != "https://example.test/openapi/v1/mqtt-auth/create" || call.Body["name"] != "agent" {
		t.Fatalf("unexpected dry-run: %#v", call)
	}
	if call.Body["clientIDRandomSuffixEnabled"] != true {
		t.Fatalf("random suffix not included: %#v", call.Body)
	}
}

func TestMQTTAuthDeleteDryRunDoesNotRequireConfirmation(t *testing.T) {
	t.Setenv("TIER0_BASE_URL", "https://example.test")
	stdout, stderr, err := executeRootForTest("mqtt", "auth", "delete", "--id", "42", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("execute error: %v\nstderr: %s", err, stderr)
	}
	var envelope dryRunTestEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	call := envelope.Data.API[0]
	if call.URL != "https://example.test/openapi/v1/mqtt-auth/delete" || call.Body["id"] != float64(42) {
		t.Fatalf("unexpected dry-run: %#v", call)
	}
}

func TestMQTTPublishDryRunAcceptsJSONArrayAndRedactsCredentials(t *testing.T) {
	t.Setenv("TIER0_MQTT_BROKER", "mqtt.example")
	t.Setenv("TIER0_MQTT_CLIENT_ID", "client")
	t.Setenv("TIER0_MQTT_USERNAME", "mqtt-user")
	t.Setenv("TIER0_MQTT_PASSWORD", "mqtt-secret")
	stdout, stderr, err := executeRootForTest(
		"mqtt", "publish", "--topic", "Vision/cam1/State/Result", "--message", `[{"person_count":1}]`,
		"--json-message", "--qos", "1", "--dry-run", "--json",
	)
	if err != nil {
		t.Fatalf("execute error: %v\nstderr: %s", err, stderr)
	}
	if strings.Contains(stdout, "mqtt-user") || strings.Contains(stdout, "mqtt-secret") {
		t.Fatalf("dry-run leaked MQTT credentials: %s", stdout)
	}
	var envelope struct {
		OK     bool `json:"ok"`
		DryRun bool `json:"dry_run"`
		Data   struct {
			MQTT []mqttPublishPreview `json:"mqtt"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if !envelope.OK || !envelope.DryRun || len(envelope.Data.MQTT) != 1 {
		t.Fatalf("unexpected MQTT dry-run: %#v", envelope)
	}
	preview := envelope.Data.MQTT[0]
	if preview.Broker != "ssl://mqtt.example:8883" || preview.QoS != 1 || preview.PayloadBytes == 0 {
		t.Fatalf("unexpected preview: %#v", preview)
	}
}

func TestMQTTPublishRejectsWildcardTopic(t *testing.T) {
	stdout, stderr, err := executeRootForTest(
		"mqtt", "publish", "--topic", "Vision/+/State/Result", "--message", "{}", "--json",
	)
	if err == nil || stdout != "" {
		t.Fatalf("expected validation error, stdout=%q err=%v", stdout, err)
	}
	if !strings.Contains(stderr, `"param":"--topic"`) {
		t.Fatalf("unexpected error: %s", stderr)
	}
}

func TestMQTTProfileCannotCrossTier0Instances(t *testing.T) {
	store := &mqttprofile.Store{Dir: t.TempDir()}
	if err := store.Save("private", mqttprofile.Credential{
		ID: 7, BaseURL: "https://private.example", Broker: "ssl://mqtt.private.example:8883",
		ClientID: "client", Username: "user", Password: "secret",
	}); err != nil {
		t.Fatal(err)
	}
	originalStore := mqttCredentialStore
	mqttCredentialStore = func() (*mqttprofile.Store, error) { return store, nil }
	t.Cleanup(func() { mqttCredentialStore = originalStore })
	t.Setenv("TIER0_BASE_URL", "https://tier0.example")

	stdout, stderr, err := executeRootForTest(
		"mqtt", "auth", "delete", "--credential", "private", "--dry-run", "--json",
	)
	if err == nil || stdout != "" {
		t.Fatalf("expected cross-instance error, stdout=%q err=%v", stdout, err)
	}
	if !strings.Contains(stderr, "different Tier0 instance") {
		t.Fatalf("unexpected error: %s", stderr)
	}
}

func TestMQTTMessageOutputPreservesJSONAndBinary(t *testing.T) {
	jsonOutput := newMQTTMessageOutput(mqtttransport.Message{
		Topic: "Vision/cam1/State/Result", Payload: []byte(`[{"count":1}]`), QoS: 1, Received: time.Unix(0, 0).UTC(),
	})
	if _, ok := jsonOutput.Payload.([]any); !ok || jsonOutput.PayloadBase64 != "" {
		t.Fatalf("unexpected JSON output: %#v", jsonOutput)
	}
	binaryOutput := newMQTTMessageOutput(mqtttransport.Message{
		Topic: "binary", Payload: []byte{0xff, 0xfe}, Received: time.Unix(0, 0).UTC(),
	})
	if binaryOutput.Encoding != "base64" || binaryOutput.PayloadBase64 == "" {
		t.Fatalf("unexpected binary output: %#v", binaryOutput)
	}
}

func TestEmbeddedMQTTSkillExamplesMatchCommandTree(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(filepath.Dir(thisFile)), "internal", "embeddedskill", "content", "mqtt", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	catalog := buildSkillExampleCatalog()
	for _, ref := range parseSkillExampleRefs(string(data)) {
		commandPath, ok := catalog.resolve(ref.words)
		if !ok {
			t.Errorf("line %d: unknown command in %q", ref.line, ref.raw)
			continue
		}
		for _, flag := range ref.flags {
			if !catalog.hasFlag(commandPath, flag) {
				t.Errorf("line %d: unknown flag %s on tier0 %s", ref.line, flag, commandPath)
			}
		}
	}
}
