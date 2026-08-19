package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/errs"
	"github.com/spf13/cobra"
)

type mqttPublishPreview struct {
	Action           string `json:"action"`
	Broker           string `json:"broker"`
	Topic            string `json:"topic"`
	QoS              int    `json:"qos"`
	Retain           bool   `json:"retain"`
	PayloadBytes     int    `json:"payloadBytes"`
	CredentialSource string `json:"credential"`
}

func writeMQTTPublishDryRun(command *cobra.Command, preview mqttPublishPreview) (bool, error) {
	dryRun, _ := command.Flags().GetBool("dry-run")
	if !dryRun {
		return false, nil
	}
	jsonMode, _ := command.Flags().GetBool("json")
	if jsonMode {
		envelope := map[string]any{
			"ok":      true,
			"dry_run": true,
			"data": map[string]any{
				"mqtt": []mqttPublishPreview{preview},
			},
		}
		if err := json.NewEncoder(command.OutOrStdout()).Encode(envelope); err != nil {
			cliErr := errs.New(errs.CategoryInternal, 0, "failed to write MQTT dry-run output: "+err.Error()).WithCause(err)
			return true, handleCLIError(command, cliErr)
		}
		return true, nil
	}
	stdout := command.OutOrStdout()
	if _, err := fmt.Fprintln(stdout, "# dry-run: MQTT message not published"); err != nil {
		return true, err
	}
	_, err := fmt.Fprintf(stdout, "PUBLISH %s\n  topic=%s qos=%d retain=%t bytes=%d credential=%s\n",
		preview.Broker, preview.Topic, preview.QoS, preview.Retain, preview.PayloadBytes, strings.TrimSpace(preview.CredentialSource))
	return true, err
}
