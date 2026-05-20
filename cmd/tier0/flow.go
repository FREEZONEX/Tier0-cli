package tier0

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/client"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
)

const (
	flowTypeSource = "SourceFlow"
	flowTypeEvent  = "EventFlow"
)

func runFlow(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printFlowHelp(stdout)
		return nil
	}

	// Start background version check once for the whole flow command.
	checker := notice.Start()

	switch args[0] {
	case "list", "ls":
		return runFlowList(ctx, args[1:], stdout, stderr, checker)
	case "get":
		return runFlowGet(ctx, args[1:], stdout, stderr, checker)
	case "create":
		return runFlowCreate(ctx, args[1:], stdout, stderr, checker)
	case "update":
		return runFlowUpdate(ctx, args[1:], stdout, stderr, checker)
	case "delete", "del", "rm":
		return runFlowDelete(ctx, args[1:], stdout, stderr, checker)
	case "data":
		return runFlowData(ctx, args[1:], stdout, stderr, checker)
	case "deploy":
		return runFlowDeploy(ctx, args[1:], stdout, stderr, checker)
	case "-h", "--help", "help":
		printFlowHelp(stdout)
		return nil
	default:
		fmt.Fprintf(stderr, i18n.T("unknown flow subcommand: %s\n", "未知 flow 子命令: %s\n"), args[0])
		printFlowHelp(stderr)
		return fmt.Errorf("unknown flow subcommand: %s", args[0])
	}
}

func runFlowList(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var keyword, flowType string
	var jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--keyword", "-k":
			if i+1 < len(args) {
				keyword = args[i+1]
				i++
			}
		case "--type", "-t":
			if i+1 < len(args) {
				flowType = args[i+1]
				i++
			}
		case "--source":
			flowType = flowTypeSource
		case "--event":
			flowType = flowTypeEvent
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		}
	}

	body, _ := json.Marshal(map[string]string{
		"keyword":  keyword,
		"flowType": flowType,
	})

	resp, err := doFlowAPI(ctx, "/openapi/v1/flow/list", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}

	var result struct {
		List []struct {
			Id                 int64  `json:"id"`
			FlowName           string `json:"flowName"`
			FlowType           string `json:"flowType"`
			FlowStatus         string `json:"flowStatus"`
			Description        string `json:"description"`
			IsFavorite         int64  `json:"isFavorite"`
			CurrentVersionName string `json:"currentVersionName"`
		} `json:"list"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		return nil
	}
	if len(result.List) == 0 {
		checker.Emit("", false, stdout, stderr)
		fmt.Fprintln(stdout, i18n.T("No flows found.", "暂无 Flow。"))
		return nil
	}
	fmt.Fprintf(stdout, "%-6s  %-12s  %-26s  %-8s  %s\n",
		i18n.T("ID", "ID"),
		i18n.T("Type", "类型"),
		i18n.T("Name", "名称"),
		i18n.T("Status", "状态"),
		i18n.T("Description", "说明"),
	)
	fmt.Fprintln(stdout, strings.Repeat("-", 80))
	for _, f := range result.List {
		fav := ""
		if f.IsFavorite == 1 {
			fav = " ★"
		}
		fmt.Fprintf(stdout, "%-6d  %-12s  %-26s  %-8s  %s%s\n",
			f.Id, f.FlowType, f.FlowName, f.FlowStatus, f.Description, fav)
	}
	checker.Emit("", false, stdout, stderr)
	return nil
}

func runFlowGet(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var id int64
	var jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--id":
			if i+1 < len(args) {
				n, _ := strconv.ParseInt(args[i+1], 10, 64)
				id = n
				i++
			}
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			if id == 0 {
				n, _ := strconv.ParseInt(args[i], 10, 64)
				id = n
			}
		}
	}

	if id == 0 {
		return fmt.Errorf(i18n.T(
			"specify a Flow ID via --id <id> or as a positional argument",
			"请通过 --id <id> 或直接传入 ID 指定 Flow",
		))
	}

	body, _ := json.Marshal(map[string]int64{"id": id})
	resp, err := doFlowAPI(ctx, "/openapi/v1/flow/get", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}

	var f struct {
		Id                 int64  `json:"id"`
		FlowId             string `json:"flowId"`
		FlowName           string `json:"flowName"`
		FlowType           string `json:"flowType"`
		FlowStatus         string `json:"flowStatus"`
		Description        string `json:"description"`
		IsFavorite         int64  `json:"isFavorite"`
		CurrentVersionName string `json:"currentVersionName"`
		CurrentVersionType string `json:"currentVersionType"`
	}
	if err := json.Unmarshal([]byte(resp), &f); err != nil {
		fmt.Fprintln(stdout, resp)
		return nil
	}
	fav := i18n.T("no", "否")
	if f.IsFavorite == 1 {
		fav = i18n.T("yes", "是")
	}
	fmt.Fprintf(stdout, "%-16s %d\n", i18n.T("ID:", "ID:"), f.Id)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("FlowId:", "FlowId:"), f.FlowId)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Name:", "名称:"), f.FlowName)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Type:", "类型:"), f.FlowType)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Status:", "状态:"), f.FlowStatus)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Description:", "说明:"), f.Description)
	fmt.Fprintf(stdout, "%-16s %s\n", i18n.T("Favorite:", "收藏:"), fav)
	fmt.Fprintf(stdout, "%-16s %s (%s)\n",
		i18n.T("Version:", "当前版本:"), f.CurrentVersionName, f.CurrentVersionType)
	checker.Emit("", false, stdout, stderr)
	return nil
}

func runFlowCreate(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var flowName, flowType, description, template string
	var jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 < len(args) {
				flowName = args[i+1]
				i++
			}
		case "--type", "-t":
			if i+1 < len(args) {
				flowType = args[i+1]
				i++
			}
		case "--source":
			flowType = flowTypeSource
		case "--event":
			flowType = flowTypeEvent
		case "--desc", "--description":
			if i+1 < len(args) {
				description = args[i+1]
				i++
			}
		case "--template":
			if i+1 < len(args) {
				template = args[i+1]
				i++
			}
		case "--template-file":
			if i+1 < len(args) {
				raw, err := os.ReadFile(args[i+1])
				if err != nil {
					return fmt.Errorf(i18n.T("failed to read template file: %w", "读取模板文件失败: %w"), err)
				}
				template = string(raw)
				i++
			}
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		}
	}

	if flowName == "" {
		return fmt.Errorf(i18n.T(
			"flow name is required (--name)",
			"请通过 --name 指定 Flow 名称",
		))
	}
	if flowType == "" {
		return fmt.Errorf(i18n.T(
			"flow type is required: use --type SourceFlow|EventFlow, or --source / --event",
			"请通过 --type SourceFlow|EventFlow（或 --source / --event）指定 Flow 类型",
		))
	}

	payload := map[string]string{
		"flowName":    flowName,
		"flowType":    flowType,
		"description": description,
		"template":    template,
	}
	body, _ := json.Marshal(payload)
	resp, err := doFlowAPI(ctx, "/openapi/v1/flow/create", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}

	var result struct {
		Id int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, stderr)
		return nil
	}
	fmt.Fprintf(stdout, i18n.T("✓ Flow created, ID: %d\n", "✓ Flow 创建成功，ID: %d\n"), result.Id)
	checker.Emit("", false, stdout, stderr)
	return nil
}

func runFlowUpdate(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var id int64
	var flowName, description, template string
	var isFavorite int64 = -1
	var jsonOutput, debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--id":
			if i+1 < len(args) {
				n, _ := strconv.ParseInt(args[i+1], 10, 64)
				id = n
				i++
			}
		case "--name", "-n":
			if i+1 < len(args) {
				flowName = args[i+1]
				i++
			}
		case "--desc", "--description":
			if i+1 < len(args) {
				description = args[i+1]
				i++
			}
		case "--template":
			if i+1 < len(args) {
				template = args[i+1]
				i++
			}
		case "--template-file":
			if i+1 < len(args) {
				raw, err := os.ReadFile(args[i+1])
				if err != nil {
					return fmt.Errorf(i18n.T("failed to read template file: %w", "读取模板文件失败: %w"), err)
				}
				template = string(raw)
				i++
			}
		case "--favorite":
			isFavorite = 1
		case "--unfavorite":
			isFavorite = 0
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			if id == 0 {
				n, _ := strconv.ParseInt(args[i], 10, 64)
				id = n
			}
		}
	}

	if id == 0 {
		return fmt.Errorf(i18n.T(
			"specify a Flow ID via --id <id> or as a positional argument",
			"请通过 --id <id> 或直接传入 ID 指定 Flow",
		))
	}

	payload := map[string]interface{}{"id": id}
	if flowName != "" {
		payload["flowName"] = flowName
	}
	if description != "" {
		payload["description"] = description
	}
	if template != "" {
		payload["template"] = template
	}
	if isFavorite >= 0 {
		payload["isFavorite"] = isFavorite
	}

	body, _ := json.Marshal(payload)
	resp, err := doFlowAPI(ctx, "/openapi/v1/flow/update", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}
	_ = resp
	fmt.Fprintf(stdout, i18n.T("✓ Flow %d updated\n", "✓ Flow %d 更新成功\n"), id)
	checker.Emit("", false, stdout, stderr)
	return nil
}

func runFlowDelete(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var ids []int64
	var jsonOutput, debug, confirmed bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--id":
			if i+1 < len(args) {
				n, _ := strconv.ParseInt(args[i+1], 10, 64)
				ids = append(ids, n)
				i++
			}
		case "--yes", "-y":
			confirmed = true
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			for _, part := range strings.Split(args[i], ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				n, err := strconv.ParseInt(part, 10, 64)
				if err == nil {
					ids = append(ids, n)
				}
			}
		}
	}

	if len(ids) == 0 {
		return fmt.Errorf(i18n.T(
			"specify at least one Flow ID via --id <id> or as positional arguments (comma-separated)",
			"请通过 --id <id> 或直接传入 ID（支持多个，逗号分隔）指定要删除的 Flow",
		))
	}

	// High-risk gate: deleting a Flow stops the Node-RED container and is irreversible.
	summary := i18n.T(
		fmt.Sprintf("Delete Flow(s) %v — this will STOP the Node-RED container(s) and cannot be undone.", ids),
		fmt.Sprintf("删除 Flow %v — 将停止对应的 Node-RED 容器，操作不可逆。", ids),
	)
	if err := highrisk.Guard(confirmed, "flow delete", summary); err != nil {
		return err
	}

	body, _ := json.Marshal(map[string][]int64{"ids": ids})
	resp, err := doFlowAPI(ctx, "/openapi/v1/flow/delete", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}
	_ = resp
	if len(ids) == 1 {
		fmt.Fprintf(stdout, i18n.T("✓ Flow %d deleted\n", "✓ Flow %d 已删除\n"), ids[0])
	} else {
		fmt.Fprintf(stdout, i18n.T("✓ %d flows deleted\n", "✓ 已删除 %d 个 Flow\n"), len(ids))
	}
	checker.Emit("", false, stdout, stderr)
	return nil
}

func runFlowData(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var id int64
	var outFile string
	var debug bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--id":
			if i+1 < len(args) {
				n, _ := strconv.ParseInt(args[i+1], 10, 64)
				id = n
				i++
			}
		case "--out", "-o":
			if i+1 < len(args) {
				outFile = args[i+1]
				i++
			}
		case "--debug":
			debug = true
		default:
			if id == 0 {
				n, _ := strconv.ParseInt(args[i], 10, 64)
				id = n
			}
		}
	}

	if id == 0 {
		return fmt.Errorf(i18n.T(
			"specify a Flow ID via --id <id> or as a positional argument",
			"请通过 --id <id> 或直接传入 ID 指定 Flow",
		))
	}

	body, _ := json.Marshal(map[string]int64{"id": id})
	resp, err := doFlowAPI(ctx, "/openapi/v1/flow/flowdata", string(body), debug)
	if err != nil {
		return outputError(stderr, true, err)
	}

	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(resp), 0644); err != nil {
			return fmt.Errorf(i18n.T("failed to write file: %w", "写入文件失败: %w"), err)
		}
		fmt.Fprintf(stdout, i18n.T("✓ Flow data saved to %s\n", "✓ Flow 数据已保存到 %s\n"), outFile)
		checker.Emit("", false, stdout, stderr)
		return nil
	}

	// data command always outputs raw JSON — inject notice into it.
	checker.Emit(resp, true, stdout, stderr)
	return nil
}

func runFlowDeploy(ctx context.Context, args []string, stdout, stderr io.Writer, checker *notice.Checker) error {
	var id int64
	var flowsJSON string
	var jsonOutput, debug, confirmed bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--id":
			if i+1 < len(args) {
				n, _ := strconv.ParseInt(args[i+1], 10, 64)
				id = n
				i++
			}
		case "--flows-json":
			if i+1 < len(args) {
				flowsJSON = args[i+1]
				i++
			}
		case "--flows-file", "-f":
			if i+1 < len(args) {
				raw, err := os.ReadFile(args[i+1])
				if err != nil {
					return fmt.Errorf(i18n.T("failed to read flows file: %w", "读取 flows 文件失败: %w"), err)
				}
				flowsJSON = string(raw)
				i++
			}
		case "--yes", "-y":
			confirmed = true
		case "--json":
			jsonOutput = true
		case "--debug":
			debug = true
		default:
			if id == 0 {
				n, _ := strconv.ParseInt(args[i], 10, 64)
				id = n
			}
		}
	}

	if id == 0 {
		return fmt.Errorf(i18n.T(
			"specify a Flow ID via --id <id> or as a positional argument",
			"请通过 --id <id> 或直接传入 ID 指定 Flow",
		))
	}
	if flowsJSON == "" {
		return fmt.Errorf(i18n.T(
			"provide Node-RED canvas JSON via --flows-json '<json>' or --flows-file <file>",
			"请通过 --flows-json '<json>' 或 --flows-file <file> 提供 Node-RED 画布数据",
		))
	}

	// High-risk gate: deploy replaces ALL Node-RED nodes — existing configuration is overwritten.
	summary := i18n.T(
		fmt.Sprintf("Deploy canvas to Flow %d — ALL existing Node-RED nodes will be REPLACED. Back up with 'tier0 flow data --id %d --out backup.json' first.", id, id),
		fmt.Sprintf("部署画布到 Flow %d — 将替换该 Node-RED 实例的所有节点配置。建议先执行 'tier0 flow data --id %d --out backup.json' 备份。", id, id),
	)
	if err := highrisk.Guard(confirmed, "flow deploy", summary); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"id":        id,
		"flowsJson": flowsJSON,
	}
	body, _ := json.Marshal(payload)
	resp, err := doFlowAPI(ctx, "/openapi/v1/flow/deploy", string(body), debug)
	if err != nil {
		return outputError(stderr, jsonOutput, err)
	}

	if jsonOutput {
		checker.Emit(resp, true, stdout, stderr)
		return nil
	}

	var result struct {
		FlowId string `json:"flowId"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Fprintln(stdout, resp)
		checker.Emit("", false, stdout, stderr)
		return nil
	}
	fmt.Fprintf(stdout, i18n.T(
		"✓ Flow %d deployed, Node-RED FlowId: %s\n",
		"✓ Flow %d 部署成功，Node-RED FlowId: %s\n",
	), id, result.FlowId)
	checker.Emit("", false, stdout, stderr)
	return nil
}

// doFlowAPI loads the saved profile and calls the Flow API.
func doFlowAPI(ctx context.Context, endpoint, body string, debug bool) (string, error) {
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

// flowOutputError is a local alias so flow subcommand handlers can call
// outputError (defined in root.go) in the same package without circular refs.
// errors.As is used inside outputError to unwrap *apierr.APIError.
var _ = errors.As // ensure errors import is used

func printFlowHelp(w io.Writer) {
	fmt.Fprintln(w, i18n.T(
		"Usage: tier0 flow <subcommand> [flags]",
		"用法: tier0 flow <子命令> [选项]",
	))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T(
		"Manage Node-RED Flows in a Workspace (SourceFlow / EventFlow).",
		"管理 Workspace 中的 Node-RED Flow（SourceFlow / EventFlow）",
	))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Subcommands:", "子命令:"))
	fmt.Fprintln(w, i18n.T("  list            List all flows", "  list            列出所有 Flow"))
	fmt.Fprintln(w, i18n.T("  get             Get flow details", "  get             获取 Flow 详情"))
	fmt.Fprintln(w, i18n.T("  create          Create a new flow", "  create          创建新 Flow"))
	fmt.Fprintln(w, i18n.T("  update          Update flow metadata", "  update          更新 Flow 元数据"))
	fmt.Fprintln(w, i18n.T("  delete          Delete flow(s)", "  delete          删除 Flow"))
	fmt.Fprintln(w, i18n.T("  data            Get Node-RED canvas JSON", "  data            获取 Node-RED 画布 JSON 数据"))
	fmt.Fprintln(w, i18n.T("  deploy          Deploy Node-RED canvas JSON", "  deploy          部署 Node-RED 画布 JSON"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flow types:", "Flow 类型:"))
	fmt.Fprintln(w, i18n.T(
		"  SourceFlow      Connects protocols, collects data, publishes MQTT",
		"  SourceFlow      连接协议采集数据并发布 MQTT 的 Node-RED 实例",
	))
	fmt.Fprintln(w, i18n.T(
		"  EventFlow       Processes business data downstream",
		"  EventFlow       对业务数据进行二次处理的 Node-RED 实例",
	))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (list):", "选项 (list):"))
	fmt.Fprintln(w, i18n.T("  --keyword, -k <kw>        Filter by name keyword", "  --keyword, -k <kw>        按名称关键词过滤"))
	fmt.Fprintln(w, i18n.T("  --type, -t <type>         Filter by type (SourceFlow|EventFlow)", "  --type, -t <type>         按类型过滤 (SourceFlow|EventFlow)"))
	fmt.Fprintln(w, i18n.T("  --source                  Shorthand for --type SourceFlow", "  --source                  仅列出 SourceFlow"))
	fmt.Fprintln(w, i18n.T("  --event                   Shorthand for --type EventFlow", "  --event                   仅列出 EventFlow"))
	fmt.Fprintln(w, i18n.T("  --json                    Output as JSON", "  --json                    以 JSON 格式输出"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (get):", "选项 (get):"))
	fmt.Fprintln(w, i18n.T("  --id <id>                 Flow ID", "  --id <id>                 Flow ID"))
	fmt.Fprintln(w, i18n.T("  --json                    Output as JSON", "  --json                    以 JSON 格式输出"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (create):", "选项 (create):"))
	fmt.Fprintln(w, i18n.T("  --name, -n <name>         Flow name (required)", "  --name, -n <name>         Flow 名称（必填）"))
	fmt.Fprintln(w, i18n.T("  --type, -t <type>         Flow type (required)", "  --type, -t <type>         Flow 类型（必填）"))
	fmt.Fprintln(w, i18n.T("  --source                  Set type to SourceFlow", "  --source                  类型设为 SourceFlow"))
	fmt.Fprintln(w, i18n.T("  --event                   Set type to EventFlow", "  --event                   类型设为 EventFlow"))
	fmt.Fprintln(w, i18n.T("  --desc <description>      Description", "  --desc <description>      描述"))
	fmt.Fprintln(w, i18n.T("  --template <json>         Initial template JSON string", "  --template <json>         初始模板 JSON 字符串"))
	fmt.Fprintln(w, i18n.T("  --template-file <file>    Read initial template from file", "  --template-file <file>    从文件读取初始模板 JSON"))
	fmt.Fprintln(w, i18n.T("  --json                    Output as JSON", "  --json                    以 JSON 格式输出"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (update):", "选项 (update):"))
	fmt.Fprintln(w, i18n.T("  --id <id>                 Flow ID (required)", "  --id <id>                 Flow ID（必填）"))
	fmt.Fprintln(w, i18n.T("  --name, -n <name>         New name", "  --name, -n <name>         新名称"))
	fmt.Fprintln(w, i18n.T("  --desc <description>      New description", "  --desc <description>      新描述"))
	fmt.Fprintln(w, i18n.T("  --template <json>         New template JSON string", "  --template <json>         更新模板 JSON 字符串"))
	fmt.Fprintln(w, i18n.T("  --template-file <file>    Read new template from file", "  --template-file <file>    从文件读取模板 JSON"))
	fmt.Fprintln(w, i18n.T("  --favorite                Mark as favorite", "  --favorite                标记为收藏"))
	fmt.Fprintln(w, i18n.T("  --unfavorite              Remove from favorites", "  --unfavorite              取消收藏"))
	fmt.Fprintln(w, i18n.T("  --json                    Output as JSON", "  --json                    以 JSON 格式输出"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (delete):", "选项 (delete):"))
	fmt.Fprintln(w, i18n.T("  --id <id>                 Flow ID (repeatable for multiple)", "  --id <id>                 Flow ID（可重复指定多个）"))
	fmt.Fprintln(w, i18n.T("  --yes, -y                 Confirm high-risk operation (required)", "  --yes, -y                 确认高风险操作（必填）"))
	fmt.Fprintln(w, i18n.T("  --json                    Output as JSON", "  --json                    以 JSON 格式输出"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (data):", "选项 (data):"))
	fmt.Fprintln(w, i18n.T("  --id <id>                 Flow ID", "  --id <id>                 Flow ID"))
	fmt.Fprintln(w, i18n.T("  --out, -o <file>          Save output to file", "  --out, -o <file>          将结果保存到文件"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Flags (deploy):", "选项 (deploy):"))
	fmt.Fprintln(w, i18n.T("  --id <id>                 Flow ID (required)", "  --id <id>                 Flow ID（必填）"))
	fmt.Fprintln(w, i18n.T("  --flows-json '<json>'     Node-RED canvas JSON string", "  --flows-json '<json>'     Node-RED 画布 JSON 字符串"))
	fmt.Fprintln(w, i18n.T("  --flows-file, -f <file>   Read Node-RED canvas JSON from file (recommended)", "  --flows-file, -f <file>   从文件读取 Node-RED 画布 JSON（推荐）"))
	fmt.Fprintln(w, i18n.T("  --yes, -y                 Confirm high-risk operation (required)", "  --yes, -y                 确认高风险操作（必填）"))
	fmt.Fprintln(w, i18n.T("  --json                    Output as JSON", "  --json                    以 JSON 格式输出"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Common flags:", "通用选项:"))
	fmt.Fprintln(w, i18n.T("  --debug                   Print HTTP request/response details", "  --debug                   输出 HTTP 请求/响应详情"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Examples:", "示例:"))
	fmt.Fprintln(w, "  tier0 flow list")
	fmt.Fprintln(w, "  tier0 flow list --source")
	fmt.Fprintln(w, i18n.T(
		"  tier0 flow list --event --json         list EventFlow as JSON",
		"  tier0 flow list --event --json         以 JSON 列出 EventFlow",
	))
	fmt.Fprintln(w, "  tier0 flow get --id 1")
	fmt.Fprintln(w, i18n.T(
		"  tier0 flow create --name my-source --source --desc \"Protocol collector\"",
		"  tier0 flow create --name my-source --source --desc \"协议采集\"",
	))
	fmt.Fprintln(w, "  tier0 flow create --name my-event --event")
	fmt.Fprintln(w, "  tier0 flow update --id 1 --name new-name --favorite")
	fmt.Fprintln(w, i18n.T(
		"  tier0 flow delete --id 1 --id 2       delete multiple flows",
		"  tier0 flow delete --id 1 --id 2       删除多个 Flow",
	))
	fmt.Fprintln(w, i18n.T(
		"  tier0 flow data --id 1 --out flows.json   export canvas data",
		"  tier0 flow data --id 1 --out flows.json   导出画布数据到文件",
	))
	fmt.Fprintln(w, i18n.T(
		"  tier0 flow deploy --id 1 -f flows.json    deploy from file",
		"  tier0 flow deploy --id 1 -f flows.json    从文件部署 Node-RED 画布",
	))
}
