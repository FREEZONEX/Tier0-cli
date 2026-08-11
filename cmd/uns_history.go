package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Query historical data for topics",
	Long:  "Query historical data for one or more UNS topics.\n\nTime formats accepted:\n  Relative: -1h  -30m  -7d  -1w\n  Absolute: 2026-01-01T00:00:00Z  (ISO 8601)\n  Keyword:  now\n\nExamples:\n  tier0 uns history -t demo --start -1h\n  tier0 uns history -t demo --start -24h --end now --fn avg --interval 1h\n  tier0 uns history -t demo --start 2026-01-01T00:00:00Z --end 2026-01-02T00:00:00Z",

	RunE: runUnsHistory,
}

func init() {
	unsHistoryCmd.Flags().StringSliceP("topic", "t", nil,
		"Topic name(s) (repeatable, required)")
	unsHistoryCmd.Flags().String("start", "",
		"Start time: relative (-1h/-30m/-7d), ISO 8601, or 'now' (required)")
	unsHistoryCmd.Flags().String("end", "now",
		"End time: relative, ISO 8601, or 'now' (default: now)")
	unsHistoryCmd.Flags().Int("page", 1,
		"Page number")
	unsHistoryCmd.Flags().IntP("size", "l", 100,
		"Page size (max data points)")
	unsHistoryCmd.Flags().String("interval", "",
		"Aggregation interval (e.g. 1m, 1h, 1d)")
	unsHistoryCmd.Flags().String("fn", "",
		"Aggregation function (avg/max/min/sum/count)")
	unsHistoryCmd.Flags().String("field", "",
		"Aggregation field name")
	unsHistoryCmd.MarkFlagRequired("topic")
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
				return "", fmt.Errorf(
					"invalid relative time %q (e.g. -1h, -30m, -7d, -1w)",
					expr)
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

	return "", fmt.Errorf(
		"unrecognized time expression %q — use relative (-1h/-30m/-7d), ISO 8601, or 'now'",
		expr)
}

func runUnsHistory(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topics, _ := cmd.Flags().GetStringSlice("topic")
	startExpr, _ := cmd.Flags().GetString("start")
	endExpr, _ := cmd.Flags().GetString("end")
	page, _ := cmd.Flags().GetInt("page")
	size, _ := cmd.Flags().GetInt("size")
	interval, _ := cmd.Flags().GetString("interval")
	fn, _ := cmd.Flags().GetString("fn")
	field, _ := cmd.Flags().GetString("field")

	startTime, err := parseTimeToISO(startExpr)
	if err != nil {
		return invalidArgumentCause(cmd, "--start", err.Error(), err)
	}
	endTime, err := parseTimeToISO(endExpr)
	if err != nil {
		return invalidArgumentCause(cmd, "--end", err.Error(), err)
	}
	if page < 1 {
		return invalidArgument(cmd, "--page", "--page must be at least 1")
	}
	if size < 1 {
		return invalidArgument(cmd, "--size", "--size must be at least 1")
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
	checker := notice.Start()
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/history", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckResponse(resp); err != nil {
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
