package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsWriteCmd = &cobra.Command{
	Use:   "write",
	Short: i18n.T("Write value to a UNS topic", "写入 UNS 点位值"),
	Long: i18n.T(
		"Write a value to one or more UNS topics.\n\nExamples:\n  tier0 uns write --topic demo --value '{\"temp\":25}'\n  tier0 uns write --topic demo --file payload.json\n  tier0 uns write --topic sensor1 --value '{\"on\":true}' --qos 1 --retain",
		"向一个或多个 UNS 点位写入值。\n\n示例:\n  tier0 uns write --topic demo --value '{\"temp\":25}'\n  tier0 uns write --topic demo --file payload.json\n  tier0 uns write --topic sensor1 --value '{\"on\":true}' --qos 1 --retain",
	),
	RunE: runUnsWrite,
}

func init() {
	unsWriteCmd.Flags().StringP("topic", "t", "",
		i18n.T("Topic name to write to (required)", "要写入的点位名称（必填）"))
	unsWriteCmd.Flags().StringP("value", "v", "",
		i18n.T("JSON value string (mutually exclusive with --file)", "JSON 值字符串（与 --file 互斥）"))
	unsWriteCmd.Flags().StringP("file", "f", "",
		i18n.T("Read value from file (mutually exclusive with --value)", "从文件读取值（与 --value 互斥）"))
	unsWriteCmd.Flags().Int("qos", 0,
		i18n.T("MQTT QoS level (0/1/2)", "MQTT QoS 等级（0/1/2）"))
	unsWriteCmd.Flags().Bool("retain", false,
		i18n.T("Set MQTT retain flag", "设置 MQTT retain 标志"))
	unsWriteCmd.MarkFlagRequired("topic")
}

func runUnsWrite(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topic, _ := cmd.Flags().GetString("topic")
	value, _ := cmd.Flags().GetString("value")
	file, _ := cmd.Flags().GetString("file")
	qos, _ := cmd.Flags().GetInt("qos")
	retain, _ := cmd.Flags().GetBool("retain")

	if file != "" {
		if value != "" {
			return fmt.Errorf(i18n.T(
				"--value and --file are mutually exclusive",
				"--value 和 --file 不能同时指定",
			))
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to read file: %w", "读取文件失败: %w"), err)
		}
		value = string(raw)
	}
	if value == "" {
		return fmt.Errorf(i18n.T(
			"specify a value via --value '<json>' or --file <path>",
			"请通过 --value '<json>' 或 --file <path> 指定要写入的值",
		))
	}

	var valueObj any
	if err := json.Unmarshal([]byte(value), &valueObj); err != nil {
		return fmt.Errorf(i18n.T("invalid JSON value: %w", "无效的 JSON 值: %w"), err)
	}

	writeItem := map[string]any{
		"topic": topic,
		"value": valueObj,
	}

	payload := map[string]any{
		"writes": []any{writeItem},
	}
	if qos != 0 {
		payload["qos"] = qos
	}
	if retain {
		payload["retain"] = true
	}

	body := cmdutil.JSONString(payload)

	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/write", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		fmt.Fprintf(stdout, i18n.T("Written: %s\n", "写入成功: %s\n"), topic)
	}
	return nil
}
