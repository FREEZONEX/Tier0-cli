package cmd

import "github.com/spf13/cobra"

var mqttCmd = &cobra.Command{
	Use:   "mqtt",
	Short: "Manage MQTT credentials and exchange real-time messages",
	Long:  "Create revocable MQTT credentials, publish messages, and subscribe to Tier0 MQTT topics.",
}

var mqttAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage MQTT credentials",
}

func init() {
	mqttAuthCmd.AddCommand(mqttAuthCreateCmd)
	mqttAuthCmd.AddCommand(mqttAuthDeleteCmd)
	mqttCmd.AddCommand(mqttAuthCmd)
	mqttCmd.AddCommand(mqttPublishCmd)
	mqttCmd.AddCommand(mqttSubscribeCmd)
}
