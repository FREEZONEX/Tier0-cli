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
	Long:  "Query historical data for one or more UNS topics.\n\nTime formats accepted:\n  Relative: -1h  -30m  -7d  -1w\n  Absolute: 2026-01-01T00:00:00Z  (ISO 8601)\n  Keyword:  now\n\nExamples:\n  tier0 uns history -t demo --start -1h\n  tier0 uns history -t demo --start -24h --end now --count-mode none\n  tier0 uns history -t demo --start -24h --end now --auto-sparse\n  tier0 uns history -t demo --start -24h --end now --interval 1h --aggregate-field temperature=avg",

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
		"Page size per topic (max data points per topic)")
	unsHistoryCmd.Flags().Bool("auto-sparse", false,
		"Omit page and size so the server can automatically sample large result sets")
	unsHistoryCmd.Flags().String("count-mode", "",
		"Count mode: exact (compatible default) or none (skip exact COUNT and follow meta.hasMore)")
	unsHistoryCmd.Flags().String("interval", "",
		"Aggregation interval (e.g. 1m, 1h, 1d)")
	unsHistoryCmd.Flags().String("fn", "",
		"Single-field aggregation function (avg/max/min/sum/count/first/last)")
	unsHistoryCmd.Flags().String("field", "",
		"Single-field aggregation field name")
	unsHistoryCmd.Flags().StringSlice("aggregate-field", nil,
		"Multi-field aggregation as name or name=function (repeatable)")
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
	autoSparse, _ := cmd.Flags().GetBool("auto-sparse")
	countMode, _ := cmd.Flags().GetString("count-mode")
	interval, _ := cmd.Flags().GetString("interval")
	fn, _ := cmd.Flags().GetString("fn")
	field, _ := cmd.Flags().GetString("field")
	aggregateFieldSpecs, _ := cmd.Flags().GetStringSlice("aggregate-field")

	startTime, err := parseTimeToISO(startExpr)
	if err != nil {
		return invalidArgumentCause(cmd, "--start", err.Error(), err)
	}
	endTime, err := parseTimeToISO(endExpr)
	if err != nil {
		return invalidArgumentCause(cmd, "--end", err.Error(), err)
	}
	if cmd.Flags().Changed("page") && page < 1 {
		return invalidArgument(cmd, "--page", "--page must be at least 1")
	}
	if cmd.Flags().Changed("size") && size < 1 {
		return invalidArgument(cmd, "--size", "--size must be at least 1")
	}
	if autoSparse && (cmd.Flags().Changed("page") || cmd.Flags().Changed("size")) {
		return invalidArgument(cmd, "--auto-sparse", "--auto-sparse cannot be combined with --page or --size")
	}
	countMode = strings.ToLower(strings.TrimSpace(countMode))
	if countMode != "" && countMode != "exact" && countMode != "none" {
		return invalidArgument(cmd, "--count-mode", "--count-mode must be exact or none")
	}
	fn = strings.ToLower(strings.TrimSpace(fn))
	interval = strings.TrimSpace(interval)
	field = strings.TrimSpace(field)
	if fn != "" && !isHistoryAggregationFunction(fn) {
		return invalidArgument(cmd, "--fn", "--fn must be avg, max, min, sum, count, first, or last")
	}
	aggregateFields, err := parseHistoryAggregationFields(aggregateFieldSpecs)
	if err != nil {
		return invalidArgumentCause(cmd, "--aggregate-field", err.Error(), err)
	}
	if len(aggregateFields) > 0 && (fn != "" || field != "") {
		return invalidArgument(cmd, "--aggregate-field", "--aggregate-field cannot be combined with --fn or --field")
	}
	if (fn != "" || field != "" || len(aggregateFields) > 0) && interval == "" {
		return invalidArgument(cmd, "--interval", "--interval is required when aggregation fields or functions are set")
	}

	payload := map[string]any{
		"topics":     topics,
		"start_time": startTime,
		"end_time":   endTime,
	}
	if !autoSparse {
		payload["page"] = page
		payload["size"] = size
	}
	if countMode != "" {
		payload["countMode"] = countMode
	}
	if fn != "" || interval != "" || field != "" || len(aggregateFields) > 0 {
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
		if len(aggregateFields) > 0 {
			agg["fields"] = aggregateFields
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

func isHistoryAggregationFunction(value string) bool {
	switch value {
	case "avg", "max", "min", "sum", "count", "first", "last":
		return true
	default:
		return false
	}
}

func parseHistoryAggregationFields(specs []string) ([]map[string]any, error) {
	fields := make([]map[string]any, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		parts := strings.SplitN(strings.TrimSpace(spec), "=", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return nil, fmt.Errorf("field name is required (use name or name=function)")
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("field %q is repeated", name)
		}
		seen[key] = struct{}{}
		item := map[string]any{"name": name}
		if len(parts) == 2 {
			function := strings.ToLower(strings.TrimSpace(parts[1]))
			if !isHistoryAggregationFunction(function) {
				return nil, fmt.Errorf("field %q function must be avg, max, min, sum, count, first, or last", name)
			}
			item["function"] = function
		}
		fields = append(fields, item)
	}
	return fields, nil
}
