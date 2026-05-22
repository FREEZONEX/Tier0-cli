package tier0

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/client"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
)

func runUNS(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUNSHelp(stdout)
		return nil
	}

	checker := notice.Start()

	switch args[0] {
	case "browse", "ls":
		return runUNSBrowse(ctx, args[1:], stdout, stderr, checker)
	case "read", "get":
		return runUNSRead(ctx, args[1:], stdout, stderr, checker)
	case "write", "pub":
		return runUNSWrite(ctx, args[1:], stdout, stderr, checker)
	case "search", "find":
		return runUNSSearch(ctx, args[1:], stdout, stderr, checker)
	case "history", "hist":
		return runUNSHistory(ctx, args[1:], stdout, stderr, checker)
	case "create":
		return runUNSCreate(ctx, args[1:], stdout, stderr, checker)
	case "update":
		return runUNSUpdate(ctx, args[1:], stdout, stderr, checker)
	case "delete", "del", "rm":
		return runUNSDelete(ctx, args[1:], stdout, stderr, checker)
	case "restore":
		return runUNSRestore(ctx, args[1:], stdout, stderr, checker)
	case "-h", "--help", "help":
		printUNSHelp(stdout)
		return nil
	default:
		fmt.Fprintf(stderr, i18n.T("unknown uns subcommand: %s\n", "未知 uns 子命令: %s\n"), args[0])
		printUNSHelp(stderr)
		return fmt.Errorf("unknown uns subcommand: %s", args[0])
	}
}

// ── browse ───────────────────────────────────────────────────────────────────

func runUNSBrowse(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	path := "/"
	var depth int64
	var includeMeta, jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path", "-p":
			if i+1 < len(args) {
				path = args[i+1]
				i++
			}
		case "--depth", "-d":
			if i+1 < len(args) {
				depth, _ = strconv.ParseInt(args[i+1], 10, 64)
				i++
			}
		case "--meta", "-m":
			includeMeta = true
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			// positional: treat as path
			if args[i] != "" && !strings.HasPrefix(args[i], "-") {
				path = args[i]
			}
		}
	}

	payload := map[string]interface{}{"path": path}
	if depth > 0 {
		payload["max_depth"] = depth
	}
	if includeMeta {
		payload["include_metadata"] = true
	}
	body, _ := json.Marshal(payload)

	resp, err := doUNSAPI(ctx, "/openapi/v1/uns/browse", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}

	// Pretty-print tree
	var result struct {
		Path     string `json:"path"`
		Type     string `json:"type"`
		Children []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description"`
			Children    []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"children"`
		} `json:"children"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, stderr)
		return nil
	}

	fmt.Fprintf(stdout, "%s  [%s]\n", result.Path, result.Type)
	for _, c := range result.Children {
		icon := "📁"
		if c.Type == "thing" {
			icon = "📊"
		}
		desc := ""
		if c.Description != "" {
			desc = "  # " + c.Description
		}
		fmt.Fprintf(stdout, "  %s %s  [%s]%s\n", icon, c.Name, c.Type, desc)
		for _, cc := range c.Children {
			icon2 := "📁"
			if cc.Type == "thing" {
				icon2 = "📊"
			}
			fmt.Fprintf(stdout, "      %s %s  [%s]\n", icon2, cc.Name, cc.Type)
		}
	}
	checker.Emit("", false, stdout, stderr)
	return nil
}

// ── read ─────────────────────────────────────────────────────────────────────

func runUNSRead(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var topics []string
	var includeMeta bool
	var jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--topic", "-t":
			if i+1 < len(args) {
				topics = append(topics, args[i+1])
				i++
			}
		case "--meta", "-m":
			includeMeta = true
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			if !strings.HasPrefix(args[i], "-") {
				topics = append(topics, args[i])
			}
		}
	}

	if len(topics) == 0 {
		return fmt.Errorf(i18n.T(
			"specify at least one topic: tier0 uns read <topic> [<topic>...] or --topic <topic>",
			"请指定至少一个 topic：tier0 uns read <topic> [<topic>...] 或 --topic <topic>",
		))
	}

	payload := map[string]interface{}{"topics": topics}
	if includeMeta {
		payload["include_metadata"] = true
	}
	body, _ := json.Marshal(payload)

	resp, err := doUNSAPI(ctx, "/openapi/v1/uns/read", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}

	var result struct {
		Results []struct {
			Success bool   `json:"success"`
			Topic   string `json:"topic"`
			Result  *struct {
				Value     interface{} `json:"value"`
				Quality   string      `json:"quality"`
				TimeStamp int64       `json:"timeStamp"`
			} `json:"result"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, stderr)
		return nil
	}

	for _, r := range result.Results {
		if !r.Success {
			msg := ""
			if r.Error != nil {
				msg = r.Error.Message
			}
			fmt.Fprintf(stdout, "✗ %-40s  %s\n", r.Topic, msg)
			continue
		}
		if r.Result == nil {
			fmt.Fprintf(stdout, "? %-40s  (no data)\n", r.Topic)
			continue
		}
		val, _ := json.Marshal(r.Result.Value)
		ts := ""
		if r.Result.TimeStamp > 0 {
			ts = time.UnixMilli(r.Result.TimeStamp).Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(stdout, "%-40s  %-12s  %-10s  %s\n",
			r.Topic, r.Result.Quality, ts, string(val))
	}
	checker.Emit("", false, stdout, stderr)
	return nil
}

// ── write ────────────────────────────────────────────────────────────────────

func runUNSWrite(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var topic, valueStr, bodyFile string
	var qos int64
	var retain, jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--topic", "-t":
			if i+1 < len(args) {
				topic = args[i+1]
				i++
			}
		case "--value", "-v":
			if i+1 < len(args) {
				valueStr = args[i+1]
				i++
			}
		case "--file", "-f":
			if i+1 < len(args) {
				bodyFile = args[i+1]
				i++
			}
		case "--qos":
			if i+1 < len(args) {
				qos, _ = strconv.ParseInt(args[i+1], 10, 64)
				i++
			}
		case "--retain":
			retain = true
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		}
	}

	var body string
	if bodyFile != "" {
		raw, err := os.ReadFile(bodyFile)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to read file: %w", "读取文件失败: %w"), err)
		}
		body = string(raw)
	} else {
		if topic == "" {
			return fmt.Errorf(i18n.T(
				"specify --topic <topic> and --value '<json>', or --file <writes.json>",
				"请通过 --topic 和 --value 指定写入目标，或用 --file 传入批量写入文件",
			))
		}
		if valueStr == "" {
			return fmt.Errorf(i18n.T(
				"specify --value '<json>' (value must be a JSON object, e.g. '{\"temp\":27.5}')",
				"请通过 --value 指定写入值（必须是 JSON 对象，如 '{\"temp\":27.5}'）",
			))
		}
		var val interface{}
		if err := json.Unmarshal([]byte(valueStr), &val); err != nil {
			return fmt.Errorf(i18n.T(
				"--value must be valid JSON: %v",
				"--value 必须是合法 JSON: %v",
			), err)
		}
		payload := map[string]interface{}{
			"writes": []map[string]interface{}{
				{"topic": topic, "value": val},
			},
		}
		if qos > 0 {
			payload["qos"] = qos
		}
		if retain {
			payload["retain"] = true
		}
		b, _ := json.Marshal(payload)
		body = string(b)
	}

	resp, err := doUNSAPI(ctx, "/openapi/v1/uns/write", body, debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}

	var result struct {
		Results []struct {
			Success bool   `json:"success"`
			Topic   string `json:"topic"`
			Error   *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, stderr)
		return nil
	}
	for _, r := range result.Results {
		if r.Success {
			fmt.Fprintf(stdout, i18n.T("✓ Written: %s\n", "✓ 写入成功: %s\n"), r.Topic)
		} else {
			msg := ""
			if r.Error != nil {
				msg = r.Error.Message
			}
			fmt.Fprintf(stdout, i18n.T("✗ Failed:  %s — %s\n", "✗ 写入失败: %s — %s\n"), r.Topic, msg)
		}
	}
	checker.Emit("", false, stdout, stderr)
	return nil
}

// ── search ───────────────────────────────────────────────────────────────────

func runUNSSearch(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var keyword, prefix, topicType string
	var includeMeta, jsonOutput, debug bool
	var page, size int64

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--keyword", "-k":
			if i+1 < len(args) {
				keyword = args[i+1]
				i++
			}
		case "--prefix":
			if i+1 < len(args) {
				prefix = args[i+1]
				i++
			}
		case "--type", "-t":
			if i+1 < len(args) {
				topicType = args[i+1]
				i++
			}
		case "--meta", "-m":
			includeMeta = true
		case "--page":
			if i+1 < len(args) {
				page, _ = strconv.ParseInt(args[i+1], 10, 64)
				i++
			}
		case "--size":
			if i+1 < len(args) {
				size, _ = strconv.ParseInt(args[i+1], 10, 64)
				i++
			}
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			if !strings.HasPrefix(args[i], "-") && keyword == "" {
				keyword = args[i]
			}
		}
	}

	payload := map[string]interface{}{}
	if keyword != "" {
		payload["keyword"] = keyword
	}
	if prefix != "" {
		payload["path_prefix"] = prefix
	}
	if topicType != "" {
		payload["topicType"] = topicType
	}
	if includeMeta {
		payload["include_metadata"] = true
	}
	if page > 0 {
		payload["page"] = page
	}
	if size > 0 {
		payload["size"] = size
	}
	body, _ := json.Marshal(payload)

	resp, err := doUNSAPI(ctx, "/openapi/v1/uns/search", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}

	var result struct {
		Total int64 `json:"total"`
		List  []struct {
			Path        string `json:"path"`
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"list"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, stderr)
		return nil
	}

	if len(result.List) == 0 {
		fmt.Fprintln(stdout, i18n.T("No results found.", "未找到匹配节点。"))
		checker.Emit("", false, stdout, stderr)
		return nil
	}

	fmt.Fprintf(stdout, i18n.T("Found %d result(s):\n\n", "共找到 %d 个节点:\n\n"), result.Total)
	fmt.Fprintf(stdout, "%-8s  %-50s  %s\n",
		i18n.T("Type", "类型"),
		i18n.T("Path", "路径"),
		i18n.T("Description", "说明"),
	)
	fmt.Fprintln(stdout, strings.Repeat("-", 80))
	for _, item := range result.List {
		fmt.Fprintf(stdout, "%-8s  %-50s  %s\n", item.Type, item.Path, item.Description)
	}
	checker.Emit("", false, stdout, stderr)
	return nil
}

// ── history ──────────────────────────────────────────────────────────────────

func runUNSHistory(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var topics []string
	var start, end, page, size int64
	var fn, interval string
	var jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--topic", "-t":
			if i+1 < len(args) {
				topics = append(topics, args[i+1])
				i++
			}
		case "--start":
			if i+1 < len(args) {
				start, _ = strconv.ParseInt(args[i+1], 10, 64)
				i++
			}
		case "--end":
			if i+1 < len(args) {
				end, _ = strconv.ParseInt(args[i+1], 10, 64)
				i++
			}
		case "--fn", "--function":
			if i+1 < len(args) {
				fn = args[i+1]
				i++
			}
		case "--interval":
			if i+1 < len(args) {
				interval = args[i+1]
				i++
			}
		case "--page":
			if i+1 < len(args) {
				page, _ = strconv.ParseInt(args[i+1], 10, 64)
				i++
			}
		case "--size":
			if i+1 < len(args) {
				size, _ = strconv.ParseInt(args[i+1], 10, 64)
				i++
			}
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			if !strings.HasPrefix(args[i], "-") {
				topics = append(topics, args[i])
			}
		}
	}

	if len(topics) == 0 {
		return fmt.Errorf(i18n.T(
			"specify at least one topic: tier0 uns history <topic> [--start <unix_sec>] [--end <unix_sec>]",
			"请指定至少一个 topic：tier0 uns history <topic> [--start <unix秒>] [--end <unix秒>]",
		))
	}

	payload := map[string]interface{}{"topics": topics}
	if start > 0 {
		payload["start"] = start
	}
	if end > 0 {
		payload["end"] = end
	}
	if fn != "" {
		payload["function"] = fn
	}
	if interval != "" {
		payload["interval"] = interval
	}
	if page > 0 {
		payload["page"] = page
	}
	if size > 0 {
		payload["size"] = size
	}
	body, _ := json.Marshal(payload)

	resp, err := doUNSAPI(ctx, "/openapi/v1/uns/history", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	// history always raw JSON — too complex for table
	checker.Emit(resp, true, stdout, stderr)
	return nil
}

// ── create ───────────────────────────────────────────────────────────────────

func runUNSCreate(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var bodyStr, bodyFile string
	var jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file", "-f":
			if i+1 < len(args) {
				bodyFile = args[i+1]
				i++
			}
		case "--body":
			if i+1 < len(args) {
				bodyStr = args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		}
	}

	var body string
	switch {
	case bodyFile != "":
		raw, err := os.ReadFile(bodyFile)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to read file: %w", "读取文件失败: %w"), err)
		}
		body = string(raw)
	case bodyStr != "":
		body = bodyStr
	default:
		return fmt.Errorf(i18n.T(
			"provide namespace definition via --file <structure.json> or --body '<json>'",
			"请通过 --file <structure.json> 或 --body '<json>' 提供命名空间定义",
		))
	}

	resp, err := doUNSAPI(ctx, "/openapi/v1/uns/create", body, debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}
	fmt.Fprintln(stdout, i18n.T("✓ Namespace node(s) created.", "✓ 命名空间节点创建成功。"))
	checker.Emit("", false, stdout, stderr)
	return nil
}

// ── update ───────────────────────────────────────────────────────────────────

func runUNSUpdate(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var path, name, desc, displayName, bodyFile string
	var jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path", "-p":
			if i+1 < len(args) {
				path = args[i+1]
				i++
			}
		case "--name", "-n":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--desc", "--description":
			if i+1 < len(args) {
				desc = args[i+1]
				i++
			}
		case "--display-name":
			if i+1 < len(args) {
				displayName = args[i+1]
				i++
			}
		case "--file", "-f":
			if i+1 < len(args) {
				bodyFile = args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		}
	}

	var body string
	if bodyFile != "" {
		raw, err := os.ReadFile(bodyFile)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to read file: %w", "读取文件失败: %w"), err)
		}
		body = string(raw)
	} else {
		if path == "" {
			return fmt.Errorf(i18n.T(
				"specify --path <path> (node to update)",
				"请通过 --path 指定要更新的节点路径",
			))
		}
		payload := map[string]interface{}{"path": path}
		var mask []string
		if name != "" {
			payload["name"] = name
			mask = append(mask, "name")
		}
		if desc != "" {
			payload["description"] = desc
			mask = append(mask, "description")
		}
		if displayName != "" {
			payload["displayName"] = displayName
			mask = append(mask, "displayName")
		}
		if len(mask) == 0 {
			return fmt.Errorf(i18n.T(
				"specify at least one field to update: --name, --desc, --display-name, or --file",
				"请至少指定一个要更新的字段：--name、--desc、--display-name，或用 --file 传入完整更新体",
			))
		}
		payload["updateMask"] = mask
		b, _ := json.Marshal(payload)
		body = string(b)
	}

	resp, err := doUNSAPI(ctx, "/openapi/v1/uns/update", body, debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}
	fmt.Fprintln(stdout, i18n.T("✓ Node updated.", "✓ 节点更新成功。"))
	checker.Emit("", false, stdout, stderr)
	return nil
}

// ── delete ───────────────────────────────────────────────────────────────────

func runUNSDelete(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var path string
	var hard, confirmed, jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path", "-p":
			if i+1 < len(args) {
				path = args[i+1]
				i++
			}
		case "--hard":
			hard = true
		case "--yes", "-y":
			confirmed = true
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			if !strings.HasPrefix(args[i], "-") && path == "" {
				path = args[i]
			}
		}
	}

	if path == "" {
		return fmt.Errorf(i18n.T(
			"specify node path via --path <path>",
			"请通过 --path 指定要删除的节点路径",
		))
	}

	// Hard delete requires explicit confirmation
	if hard {
		summary := i18n.T(
			fmt.Sprintf("Hard-delete UNS node '%s' — PERMANENT, cannot be restored.", path),
			fmt.Sprintf("永久删除 UNS 节点 '%s' — 不可恢复，无法通过 restore 撤销。", path),
		)
		if err := highrisk.Guard(confirmed, "uns delete --hard", summary); err != nil {
			return err
		}
	}

	payload := map[string]interface{}{"path": path}
	if hard {
		payload["hard_delete"] = true
	}
	body, _ := json.Marshal(payload)

	resp, err := doUNSAPI(ctx, "/openapi/v1/uns/delete", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}
	if hard {
		fmt.Fprintf(stdout, i18n.T("✓ Node permanently deleted: %s\n", "✓ 节点已永久删除: %s\n"), path)
	} else {
		fmt.Fprintf(stdout, i18n.T("✓ Node soft-deleted: %s (restore with: tier0 uns restore --path %s)\n",
			"✓ 节点已软删除: %s（可通过 tier0 uns restore --path %s 恢复）\n"), path, path)
	}
	checker.Emit("", false, stdout, stderr)
	return nil
}

// ── restore ──────────────────────────────────────────────────────────────────

func runUNSRestore(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var path string
	var jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path", "-p":
			if i+1 < len(args) {
				path = args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			if !strings.HasPrefix(args[i], "-") && path == "" {
				path = args[i]
			}
		}
	}

	if path == "" {
		return fmt.Errorf(i18n.T(
			"specify node path via --path <path>",
			"请通过 --path 指定要恢复的节点路径",
		))
	}

	body, _ := json.Marshal(map[string]string{"path": path})
	resp, err := doUNSAPI(ctx, "/openapi/v1/uns/restore", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}
	fmt.Fprintf(stdout, i18n.T("✓ Node restored: %s\n", "✓ 节点已恢复: %s\n"), path)
	checker.Emit("", false, stdout, stderr)
	return nil
}

// ── shared helper ─────────────────────────────────────────────────────────────

func doUNSAPI(ctx context.Context, endpoint, body string, debug bool) (string, error) {
	profile, err := config.LoadProfile()
	if err != nil {
		return "", fmt.Errorf(i18n.T("failed to load config: %w", "加载配置失败: %w"), err)
	}
	if profile.APIKey == "" {
		return "", apierr.New(401, `{"code":401,"msg":"API Key not found"}`)
	}
	c := client.New(profile.BaseURL, profile.APIKey)
	return c.DoAPI(ctx, endpoint, "POST", body, debug)
}

// ── help ──────────────────────────────────────────────────────────────────────

func printUNSHelp(w io.Writer) {
	fmt.Fprintln(w, i18n.T("Usage: tier0 uns <subcommand> [flags]", "用法: tier0 uns <子命令> [选项]"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T(
		"Manage UNS (Unified Namespace) nodes and data.",
		"管理 UNS（统一命名空间）节点与数据。",
	))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Subcommands:", "子命令:"))
	fmt.Fprintln(w, i18n.T("  browse, ls      Browse namespace tree", "  browse, ls      浏览命名空间树"))
	fmt.Fprintln(w, i18n.T("  read, get       Read current value of topic(s)", "  read, get       读取数据点当前值"))
	fmt.Fprintln(w, i18n.T("  write, pub      Write value to topic(s)", "  write, pub      写入数据点"))
	fmt.Fprintln(w, i18n.T("  search, find    Search nodes by keyword / prefix", "  search, find    按关键字或路径前缀搜索节点"))
	fmt.Fprintln(w, i18n.T("  history, hist   Query historical data", "  history, hist   查询历史数据"))
	fmt.Fprintln(w, i18n.T("  create          Create namespace node(s)", "  create          创建命名空间节点"))
	fmt.Fprintln(w, i18n.T("  update          Update node metadata", "  update          更新节点元数据"))
	fmt.Fprintln(w, i18n.T("  delete, rm      Soft- or hard-delete a node", "  delete, rm      软删除或永久删除节点"))
	fmt.Fprintln(w, i18n.T("  restore         Restore a soft-deleted node", "  restore         恢复软删除节点"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (browse):", "选项 (browse):"))
	fmt.Fprintln(w, i18n.T("  --path, -p <path>       Start path (default: /)", "  --path, -p <path>       起始路径（默认: /）"))
	fmt.Fprintln(w, i18n.T("  --depth, -d <n>         Max recursion depth", "  --depth, -d <n>         最大递归深度"))
	fmt.Fprintln(w, i18n.T("  --meta, -m              Include node metadata", "  --meta, -m              包含节点元数据"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (read):", "选项 (read):"))
	fmt.Fprintln(w, i18n.T("  <topic> [<topic>...]    Topic path(s) as positional args", "  <topic> [<topic>...]    topic 路径（位置参数）"))
	fmt.Fprintln(w, i18n.T("  --topic, -t <topic>     Topic path (repeatable)", "  --topic, -t <topic>     topic 路径（可重复）"))
	fmt.Fprintln(w, i18n.T("  --meta, -m              Include field definitions", "  --meta, -m              包含字段定义"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (write):", "选项 (write):"))
	fmt.Fprintln(w, i18n.T("  --topic, -t <topic>     Target topic", "  --topic, -t <topic>     目标 topic"))
	fmt.Fprintln(w, i18n.T("  --value, -v '<json>'    Value as JSON object", "  --value, -v '<json>'    写入值（JSON 对象）"))
	fmt.Fprintln(w, i18n.T("  --file, -f <file>       Batch writes JSON file", "  --file, -f <file>       批量写入 JSON 文件"))
	fmt.Fprintln(w, i18n.T("  --qos <0|1|2>           MQTT QoS level (default 0)", "  --qos <0|1|2>           MQTT QoS 等级（默认 0）"))
	fmt.Fprintln(w, i18n.T("  --retain                Set MQTT retain flag", "  --retain                设置 MQTT retain 标志"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (search):", "选项 (search):"))
	fmt.Fprintln(w, i18n.T("  --keyword, -k <kw>      Search keyword", "  --keyword, -k <kw>      搜索关键字"))
	fmt.Fprintln(w, i18n.T("  --prefix <path>         Path prefix filter", "  --prefix <path>         路径前缀过滤"))
	fmt.Fprintln(w, i18n.T("  --type <folder|thing>   Node type filter", "  --type <folder|thing>   节点类型过滤"))
	fmt.Fprintln(w, i18n.T("  --meta, -m              Include field definitions", "  --meta, -m              包含字段定义"))
	fmt.Fprintln(w, i18n.T("  --page <n>              Page number", "  --page <n>              页码"))
	fmt.Fprintln(w, i18n.T("  --size <n>              Page size", "  --size <n>              每页大小"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (history):", "选项 (history):"))
	fmt.Fprintln(w, i18n.T("  <topic>                 Topic path (positional)", "  <topic>                 topic 路径（位置参数）"))
	fmt.Fprintln(w, i18n.T("  --topic, -t <topic>     Topic path (repeatable)", "  --topic, -t <topic>     topic 路径（可重复）"))
	fmt.Fprintln(w, i18n.T("  --start <unix_sec>      Start time (Unix seconds)", "  --start <unix_sec>      起始时间（Unix 秒）"))
	fmt.Fprintln(w, i18n.T("  --end <unix_sec>        End time (Unix seconds)", "  --end <unix_sec>        结束时间（Unix 秒）"))
	fmt.Fprintln(w, i18n.T("  --fn <avg|max|min|...>  Aggregation function", "  --fn <avg|max|min|...>  聚合函数"))
	fmt.Fprintln(w, i18n.T("  --interval <1h|1m|...>  Aggregation interval", "  --interval <1h|1m|...>  聚合间隔"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (delete):", "选项 (delete):"))
	fmt.Fprintln(w, i18n.T("  --path, -p <path>       Node path (required)", "  --path, -p <path>       节点路径（必填）"))
	fmt.Fprintln(w, i18n.T("  --hard                  Permanent delete (irreversible, requires --yes)", "  --hard                  永久删除（不可逆，需 --yes 确认）"))
	fmt.Fprintln(w, i18n.T("  --yes, -y               Confirm hard delete", "  --yes, -y               确认永久删除"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Common flags:", "通用选项:"))
	fmt.Fprintln(w, i18n.T("  --json                  Output raw JSON", "  --json                  输出原始 JSON"))
	fmt.Fprintln(w, i18n.T("  --debug                 Print HTTP request/response", "  --debug                 输出 HTTP 请求/响应详情"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Examples:", "示例:"))
	fmt.Fprintln(w, "  tier0 uns browse")
	fmt.Fprintln(w, "  tier0 uns browse Plant/Line1 --depth 2")
	fmt.Fprintln(w, "  tier0 uns read Plant/Line1/Metric/Temperature")
	fmt.Fprintln(w, "  tier0 uns read Plant/+/Metric/Temperature --json")
	fmt.Fprintln(w, `  tier0 uns write --topic Plant/Line1/Metric/Temperature --value '{"temperature":27.5,"unit":"C"}'`)
	fmt.Fprintln(w, "  tier0 uns search --keyword temp --type thing")
	fmt.Fprintln(w, "  tier0 uns history Plant/Line1/Metric/Temperature --start 1715000000 --end 1715600000 --fn avg --interval 1h")
	fmt.Fprintln(w, "  tier0 uns delete --path Plant/Line1/OldSensor")
	fmt.Fprintln(w, "  tier0 uns delete --path Plant/Line1/OldSensor --hard --yes")
	fmt.Fprintln(w, "  tier0 uns restore --path Plant/Line1/OldSensor")
}
