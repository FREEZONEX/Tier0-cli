package cmd

import (
	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: i18n.T("Query historical data for topics", "查询点位历史数据"),
	Long: i18n.T(
		"Query historical data for one or more UNS topics.\n\nExamples:\n  tier0 uns history --topics demo --start-time 2026-01-01T00:00:00Z --end-time 2026-01-02T00:00:00Z\n  tier0 uns history --topics demo --start-time \"-1h\" --end-time now --size 50\n  tier0 uns history --topics demo --start-time 2026-01-01T00:00:00Z --end-time 2026-01-02T00:00:00Z --agg-function avg --agg-interval 1h",
		"查询一个或多个 UNS 点位的历史数据。\n\n示例:\n  tier0 uns history --topics demo --start-time 2026-01-01T00:00:00Z --end-time 2026-01-02T00:00:00Z\n  tier0 uns history --topics demo --start-time \"-1h\" --end-time now --size 50\n  tier0 uns history --topics demo --start-time 2026-01-01T00:00:00Z --end-time 2026-01-02T00:00:00Z --agg-function avg --agg-interval 1h",
	),
	RunE: runUnsHistory,
}

func init() {
	unsHistoryCmd.Flags().StringSliceP("topics", "t", nil,
		i18n.T("Topic name(s) (repeatable, required)", "点位名称（可重复指定，必填）"))
	unsHistoryCmd.Flags().String("start-time", "",
		i18n.T("Start timestamp (ISO 8601 or relative, e.g. \"-1h\", required)", "起始时间戳 (ISO 8601 或相对时间，如 \"-1h\"，必填)"))
	unsHistoryCmd.Flags().String("end-time", "",
		i18n.T("End timestamp (ISO 8601, default: now)", "结束时间戳 (ISO 8601，默认: 现在)"))
	unsHistoryCmd.Flags().Int("page", 1,
		i18n.T("Page number", "页码"))
	unsHistoryCmd.Flags().IntP("size", "l", 100,
		i18n.T("Page size (max data points)", "每页大小（最大数据点数）"))
	unsHistoryCmd.Flags().String("agg-interval", "",
		i18n.T("Aggregation interval (e.g. 1m, 1h, 1d)", "聚合间隔（如 1m, 1h, 1d）"))
	unsHistoryCmd.Flags().String("agg-function", "",
		i18n.T("Aggregation function (avg/max/min/sum/count)", "聚合函数（avg/max/min/sum/count）"))
	unsHistoryCmd.Flags().String("agg-field", "",
		i18n.T("Aggregation field name", "聚合字段名"))
	unsHistoryCmd.MarkFlagRequired("topics")
	unsHistoryCmd.MarkFlagRequired("start-time")
	unsHistoryCmd.MarkFlagRequired("end-time")
}

func runUnsHistory(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topics, _ := cmd.Flags().GetStringSlice("topics")
	startTime, _ := cmd.Flags().GetString("start-time")
	endTime, _ := cmd.Flags().GetString("end-time")
	page, _ := cmd.Flags().GetInt("page")
	size, _ := cmd.Flags().GetInt("size")
	aggInterval, _ := cmd.Flags().GetString("agg-interval")
	aggFunction, _ := cmd.Flags().GetString("agg-function")
	aggField, _ := cmd.Flags().GetString("agg-field")

	payload := map[string]any{
		"topics":     topics,
		"start_time": startTime,
		"end_time":   endTime,
		"page":       page,
		"size":       size,
	}

	if aggInterval != "" && aggFunction != "" && aggField != "" {
		payload["aggregation"] = map[string]any{
			"interval": aggInterval,
			"function": aggFunction,
			"field":    aggField,
		}
	}

	body := cmdutil.JSONString(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/history", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode {
		stdout.Write([]byte(resp + "\n"))
	}
	return nil
}
