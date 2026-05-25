package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: i18n.T("Query historical data for topics", "查询点位历史数据"),
	Long: i18n.T(
		"Query historical data for one or more UNS topics.\n\nTime formats accepted:\n  Relative: -1h  -30m  -7d  -1w\n  Absolute: 2026-01-01T00:00:00Z  (ISO 8601)\n  Keyword:  now\n\nExamples:\n  tier0 uns history -t demo --start -1h\n  tier0 uns history -t demo --start -24h --end now --fn avg --interval 1h\n  tier0 uns history -t demo --start 2026-01-01T00:00:00Z --end 2026-01-02T00:00:00Z",
		"查询一个或多个 UNS 点位的历史数据。\n\n时间格式：\n  相对时间: -1h  -30m  -7d  -1w\n  绝对时间: 2026-01-01T00:00:00Z (ISO 8601)\n  关键字:   now\n\n示例:\n  tier0 uns history -t demo --start -1h\n  tier0 uns history -t demo --start -24h --end now --fn avg --interval 1h\n  tier0 uns history -t demo --start 2026-01-01T00:00:00Z --end 2026-01-02T00:00:00Z",
	),
	RunE: runUnsHistory,
}

func init() {
	unsHistoryCmd.Flags().StringSliceP("topics", "t", nil,
		i18n.T("Topic name(s) (repeatable, required)", "点位名称（可重复指定，必填）"))
	unsHistoryCmd.Flags().String("start", "",
		i18n.T("Start time: relative (-1h/-30m/-7d), ISO 8601, or 'now' (required)", "起始时间：相对(-1h/-30m/-7d)、ISO 8601 或 now（必填）"))
	unsHistoryCmd.Flags().String("end", "now",
		i18n.T("End time: relative, ISO 8601, or 'now' (default: now)", "结束时间：相对、ISO 8601 或 now（默认: now）"))
	unsHistoryCmd.Flags().Int("page", 1,
		i18n.T("Page number", "页码"))
	unsHistoryCmd.Flags().IntP("size", "l", 100,
		i18n.T("Page size (max data points)", "每页大小"))
	unsHistoryCmd.Flags().String("interval", "",
		i18n.T("Aggregation interval (e.g. 1m, 1h, 1d)", "聚合间隔（如 1m、1h、1d）"))
	unsHistoryCmd.Flags().String("fn", "",
		i18n.T("Aggregation function (avg/max/min/sum/count)", "聚合函数（avg/max/min/sum/count）"))
	unsHistoryCmd.Flags().String("field", "",
		i18n.T("Aggregation field name", "聚合字段名"))
	unsHistoryCmd.MarkFlagRequired("topics")
	unsHistoryCmd.MarkFlagRequired("start")
}

// parseTimeToISO converts a user-supplied time expression to an ISO 8601 string.
// Supported formats:
//   - "now"                   → current time
//   - "-1h", "-30m", "-7d"  → now minus duration
//   - "2026-01-01T00:00:00Z" → already ISO 8601, returned as-is
func parseTimeToISO(expr string) (string, error) {
	expr = strings.TrimSpace(expr)

	if expr == "now" || expr == "" {
		return time.Now().UTC().Format(time.RFC3339), nil
	}

	// Relative: starts with "-" followed by number + unit
	if strings.HasPrefix(expr, "-") && len(expr) >= 2 {
		unitChar := expr[len(expr)-1]
		if unitChar == 'h' || unitChar == 'm' || unitChar == 'd' || unitChar == 'w' {
			numStr := expr[1 : len(expr)-1]
			n, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return "", fmt.Errorf(i18n.T(
					"invalid relative time %q (e.g. -1h, -30m, -7d, -1w)",
					"相对时间格式错误 %q（示例: -1h、-30m、-7d、-1w）",
				), expr)
			}
			var dur time.Duration
			switch unitChar {
			case 'm':
				dur = time.Duration(n) * time.Minute
			case 'h':
				dur = time.Duration(n) * time.Hour
			case 'd':
				dur = time.Duration(n) * 24 * time.Hour
			case 'w':
				dur = time.Duration(n) * 7 * 24 * time.Hour
			}
			return time.Now().UTC().Add(-dur).Format(time.RFC3339), nil
		}
	}

	// ISO 8601 / date string — validate by parsing, then normalise
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, expr); err == nil {
			return t.UTC().Format(time.RFC3339), nil
		}
	}

	return "", fmt.Errorf(i18n.T(
		"unrecognized time expression %q — use relative (-1h/-30m/-7d), ISO 8601, or 'now'",
		"无法识别的时间格式 %q — 请使用相对时间(-1h/-30m/-7d)、ISO 8601 或 now",
	), expr)
}

func runUnsHistory(cmd *cobra.Command, args []string) error {
	checker := notice.Start()
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topics, _ := cmd.Flags().GetStringSlice("topics")
	startExpr, _ := cmd.Flags().GetString("start")
	endExpr, _ := cmd.Flags().GetString("end")
	page, _ := cmd.Flags().GetInt("page")
	size, _ := cmd.Flags().GetInt("size")
	interval, _ := cmd.Flags().GetString("interval")
	fn, _ := cmd.Flags().GetString("fn")
	field, _ := cmd.Flags().GetString("field")

	startTime, err := parseTimeToISO(startExpr)
	if err != nil {
		return fmt.Errorf("--start: %w", err)
	}
	endTime, err := parseTimeToISO(endExpr)
	if err != nil {
		return fmt.Errorf("--end: %w", err)
	}

	payload := map[string]any{
		"topics":     topics,
		"start_time": startTime,
		"end_time":   endTime,
		"page":       page,
		"size":       size,
	}
	if fn != "" || interval != "" || field != "" {
		agg := map[string]any{}
		if fn != "" {
			agg["function"] = fn
		}
		if interval != "" {
			agg["interval"] = interval
		}
		if field != "" {
			agg["field"] = field
		}
		payload["aggregation"] = agg
	}

	body := cmdutil.JSONString(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/history", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	if jsonMode {
		checker.Emit(resp, true, stdout, cmd.ErrOrStderr())
	} else {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, cmd.ErrOrStderr())
	}
	return nil
}
