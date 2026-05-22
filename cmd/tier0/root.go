package tier0

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/auth"
	"github.com/FREEZONEX/Tier0-cli/internal/client"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

// Execute is the CLI entry point.
func Execute() error {
	// Resolve language as early as possible so every subsequent message
	// is already in the right language.
	initLang()

	args := os.Args[1:]
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}

	ctx := context.Background()
	cmd := args[0]

	switch cmd {
	case "login":
		return runLogin(ctx, args[1:], os.Stdout, os.Stderr)
	case "api":
		return runAPI(ctx, args[1:], os.Stdout, os.Stderr)
	case "uns":
		return runUNS(ctx, args[1:], os.Stdout, os.Stderr)
	case "flow":
		return runFlow(ctx, args[1:], os.Stdout, os.Stderr)
	case "config":
		return runConfig(ctx, args[1:], os.Stdout, os.Stderr)
	case "generate-skills":
		return runGenerateSkills(ctx, args[1:], os.Stdout, os.Stderr)
	case "skills":
		return runSkills(ctx, args[1:], os.Stdout, os.Stderr)
	case "upgrade":
		return runUpgrade(ctx, args[1:], os.Stdout, os.Stderr)
	case "--version", "-v", "version":
		fmt.Fprintf(os.Stdout, "tier0 version %s\n", version.BuildVersion)
		return nil
	case "--help", "-h", "help":
		printUsage(os.Stdout)
		return nil
	default:
		fmt.Fprintf(os.Stderr, i18n.T("unknown command: %s\n", "未知命令: %s\n"), cmd)
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// initLang loads the stored language preference and activates it.
// Priority: TIER0_LANG env > config file > default (en).
func initLang() {
	if envLang := os.Getenv("TIER0_LANG"); envLang != "" {
		i18n.SetLang(envLang)
		return
	}
	profile, err := config.LoadProfile()
	if err == nil && profile.Lang != "" {
		i18n.SetLang(profile.Lang)
	}
	// Default is already "en" inside the i18n package.
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, i18n.T(
		"tier0 — Tier0 Platform CLI",
		"tier0 — Tier0 平台命令行工具",
	))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Usage: tier0 <command> [flags]", "用法: tier0 <command> [flags]"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Commands:", "命令:"))
	fmt.Fprintln(w, i18n.T(
		"  login             Authenticate via Device Flow",
		"  login             Device Flow 登录授权",
	))
	fmt.Fprintln(w, i18n.T(
		"  api <endpoint>    Call an API endpoint directly (raw)",
		"  api <endpoint>    直接调用 API 接口（裸调）",
	))
	fmt.Fprintln(w, i18n.T(
		"  uns               Manage UNS nodes and data (browse/read/write/search/history/...)",
		"  uns               管理 UNS 节点与数据（browse/read/write/search/history/...）",
	))
	fmt.Fprintln(w, i18n.T(
		"  flow              Manage Node-RED Flows (list/get/create/update/delete/data/deploy)",
		"  flow              管理 Node-RED Flow（list/get/create/update/delete/data/deploy）",
	))
	fmt.Fprintln(w, i18n.T(
		"  config            View or update configuration",
		"  config            查看/管理配置",
	))
	fmt.Fprintln(w, i18n.T(
		"  skills            Manage Skills (list/update/version)",
		"  skills            管理 Skills（list/update/version）",
	))
	fmt.Fprintln(w, i18n.T(
		"  upgrade           Upgrade CLI to the latest version",
		"  upgrade           升级 CLI 到最新版本",
	))
	fmt.Fprintln(w, i18n.T(
		"  version           Show version",
		"  version           显示版本",
	))
	fmt.Fprintln(w, i18n.T(
		"  help              Show help",
		"  help              显示帮助",
	))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Environment variables:", "环境变量:"))
	fmt.Fprintln(w, i18n.T(
		"  TIER0_BASE_URL    Platform base URL (default: https://tier0.dev/)",
		"  TIER0_BASE_URL    平台地址 (默认: https://tier0.dev/)",
	))
	fmt.Fprintln(w, i18n.T(
		"  TIER0_API_KEY     API Key (overrides config file)",
		"  TIER0_API_KEY     API Key（优先于配置文件）",
	))
	fmt.Fprintln(w, i18n.T(
		"  TIER0_LANG        UI language: en (default) | zh",
		"  TIER0_LANG        界面语言: en（默认）| zh",
	))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("Examples:", "示例:"))
	fmt.Fprintln(w, "  tier0 config --base-url https://tier0-eks-frontend.tier0.dev")
	fmt.Fprintln(w, "  tier0 config --lang zh")
	fmt.Fprintln(w, "  tier0 login")
	fmt.Fprintln(w, "  tier0 login --no-wait")
	fmt.Fprintln(w, "  tier0 uns browse")
	fmt.Fprintln(w, "  tier0 uns read Plant/Line1/Metric/Temperature")
	fmt.Fprintln(w, `  tier0 uns write --topic Plant/Line1/Metric/Temperature --value '{"temp":27.5}'`)
	fmt.Fprintln(w, "  tier0 uns search --keyword temp --type thing")
	fmt.Fprintln(w, "  tier0 flow list --source")
	fmt.Fprintln(w, "  tier0 flow create --name my-source --source")
	fmt.Fprintln(w, "  tier0 flow deploy --id 1 -f flows.json")
}

func runLogin(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var (
		noWait     bool
		setupCode  string
		jsonMode   bool
		baseURLArg string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-wait":
			noWait = true
		case "--setup-code":
			if i+1 < len(args) {
				setupCode = args[i+1]
				i++
			}
		case "--json":
			jsonMode = true
		case "--base-url":
			if i+1 < len(args) {
				baseURLArg = strings.TrimRight(args[i+1], "/")
				i++
			}
		}
	}

	baseURL, err := resolveBaseURL(baseURLArg)
	if err != nil {
		return outputError(stderr, jsonMode, err)
	}

	if setupCode != "" {
		return runLoginPoll(ctx, baseURL, setupCode, jsonMode, stdout, stderr)
	}

	setupCode = auth.GenerateSetupCode()
	consoleURL := auth.BuildConsoleURL(baseURL, setupCode)

	if noWait {
		if jsonMode {
			fmt.Fprintf(stdout, `{"status":"authorization_required","verification_url":"%s","setup_code":"%s","expires_in":600}`+"\n", consoleURL, setupCode)
		} else {
			fmt.Fprintf(stdout, i18n.T(
				"Please complete authorization in your browser: %s\n",
				"请在浏览器中完成授权：%s\n",
			), consoleURL)
			fmt.Fprintf(stdout, i18n.T(
				"After authorization, run: tier0 login --setup-code %s\n",
				"授权完成后执行: tier0 login --setup-code %s\n",
			), setupCode)
		}
		return nil
	}

	fmt.Fprintln(stdout, i18n.T(
		"Please complete authorization in your browser:",
		"请在浏览器中完成授权：",
	))
	fmt.Fprintln(stdout, consoleURL)
	fmt.Fprintln(stdout, i18n.T(
		"\nWaiting for authorization... (polling every 5s, up to 10 minutes)",
		"\n正在等待授权...（每5秒检测一次，最多10分钟）",
	))

	return runLoginPoll(ctx, baseURL, setupCode, jsonMode, stdout, stderr)
}

func runLoginPoll(ctx context.Context, baseURL, setupCode string, jsonMode bool, stdout, stderr io.Writer) error {
	result, err := auth.PollSetupCheck(ctx, baseURL, setupCode, func(current, total int, done bool, pollErr error) {
		if jsonMode || done {
			return
		}
		if pollErr != nil {
			if current == 0 {
				fmt.Fprintf(stdout, i18n.T(
					"\r  Polling... (%d/%d) Network hiccup, retrying...\n",
					"\r  正在检测...（第 %d/%d 次）网络暂时不稳定，继续等待...\n",
				), current+1, total)
			}
			return
		}
		if current%6 == 0 && current > 0 {
			remainingMin := (total - current) * 5 / 60
			fmt.Fprintf(stdout, i18n.T(
				"\r  Waiting for authorization... (check %d/%d, ~%d min remaining)",
				"\r  正在等待授权...（第 %d/%d 次检测，剩余约 %d 分钟）",
			), current+1, total, remainingMin)
		}
	})
	if err != nil {
		return outputError(stderr, jsonMode, err)
	}

	profile := config.Profile{
		BaseURL: baseURL,
		APIKey:  result.APIKey,
	}
	if err := config.SaveProfile(profile); err != nil {
		return outputError(stderr, jsonMode, fmt.Errorf(i18n.T(
			"failed to save config: %w",
			"保存配置失败: %w",
		), err))
	}

	if jsonMode {
		fmt.Fprintf(stdout, `{"event":"authorization_complete","api_key":"%s"}`+"\n", result.APIKey)
	} else {
		fmt.Fprintln(stdout, i18n.T("\n✓ Authorization successful!", "\n✓ 授权成功！"))
		fmt.Fprintf(stdout, i18n.T(
			"API Key: %s... (saved)\n",
			"API Key: %s...（已保存）\n",
		), result.APIKey[:8])
		fmt.Fprintln(stdout, i18n.T(
			"✓ Setup complete. You can now use tier0 api commands.",
			"✓ 初始化完成，您现在可以使用 tier0 api 命令了。",
		))
	}
	return nil
}

func runAPI(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, i18n.T(
			"Usage: tier0 api <endpoint> [--body '<json>'] [--body-file FILE] [--method GET|POST] [--debug]",
			"用法: tier0 api <endpoint> [--body '<json>'] [--body-file FILE] [--method GET|POST] [--debug]",
		))
		return fmt.Errorf("missing endpoint")
	}

	// Start background version check before any I/O.
	checker := notice.Start()

	endpoint := args[0]
	var body string
	var bodyFile string
	var method string
	var debug bool
	var jsonMode bool

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--body":
			if i+1 < len(args) {
				body = args[i+1]
				i++
			}
		case "--body-file":
			if i+1 < len(args) {
				bodyFile = args[i+1]
				i++
			}
		case "--method":
			if i+1 < len(args) {
				method = args[i+1]
				i++
			}
		case "--debug":
			debug = true
		case "--json":
			jsonMode = true
		}
	}

	if bodyFile != "" {
		raw, err := os.ReadFile(bodyFile)
		if err != nil {
			return fmt.Errorf(i18n.T("failed to read body file: %w", "读取 body 文件失败: %w"), err)
		}
		body = string(raw)
	}

	profile, err := config.LoadProfile()
	if err != nil {
		return fmt.Errorf(i18n.T("failed to load config: %w", "加载配置失败: %w"), err)
	}

	if profile.APIKey == "" {
		return outputError(stderr, jsonMode, apierr.New(401, `{"code":401,"msg":"API Key not found"}`))
	}

	c := client.New(profile.BaseURL, profile.APIKey)
	resp, err := c.DoAPI(ctx, endpoint, method, body, debug)
	if err != nil {
		return outputError(stderr, jsonMode, err)
	}

	// Emit notice: in JSON mode inject into resp; in plain mode print to stderr.
	checker.Emit(resp, jsonMode, stdout, stderr)
	if !jsonMode {
		fmt.Fprintln(stdout, resp)
	}
	return nil
}

func runConfig(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var setBaseURL string
	var setLang string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base-url":
			if i+1 < len(args) {
				setBaseURL = strings.TrimRight(args[i+1], "/")
				i++
			}
		case "--lang":
			if i+1 < len(args) {
				setLang = strings.ToLower(strings.TrimSpace(args[i+1]))
				i++
			}
		}
	}

	// Write mode
	if setBaseURL != "" || setLang != "" {
		if setLang != "" && setLang != "en" && setLang != "zh" {
			return fmt.Errorf(i18n.T(
				"unsupported language %q, use: en | zh",
				"不支持的语言 %q，可选: en | zh",
			), setLang)
		}

		profile, _ := config.LoadProfile()
		if setBaseURL != "" {
			profile.BaseURL = setBaseURL
		}
		if setLang != "" {
			profile.Lang = setLang
		}
		if err := config.SaveProfile(profile); err != nil {
			return fmt.Errorf(i18n.T("failed to save config: %w", "保存配置失败: %w"), err)
		}
		if setBaseURL != "" {
			fmt.Fprintf(stdout, i18n.T("✓ BaseURL set to: %s\n", "✓ BaseURL 已设置为: %s\n"), setBaseURL)
		}
		if setLang != "" {
			// Apply immediately so the confirmation itself uses the new lang.
			i18n.SetLang(setLang)
			fmt.Fprintf(stdout, i18n.T("✓ Language set to: %s\n", "✓ 语言已设置为: %s\n"), setLang)
		}
		return nil
	}

	// Read mode
	profile, err := config.LoadProfile()
	if err != nil {
		return fmt.Errorf(i18n.T("failed to load config: %w", "加载配置失败: %w"), err)
	}

	baseURL, _ := resolveBaseURL("")
	if profile.BaseURL != "" {
		baseURL = profile.BaseURL
	}
	lang := profile.Lang
	if lang == "" {
		lang = "en"
	}

	fmt.Fprintf(stdout, i18n.T("BaseURL:  %s\n", "BaseURL:  %s\n"), baseURL)
	fmt.Fprintf(stdout, i18n.T("Language: %s\n", "语言:     %s\n"), lang)
	if profile.APIKey != "" {
		fmt.Fprintf(stdout, i18n.T("API Key:  %s...\n", "API Key:  %s...\n"), profile.APIKey[:8])
	} else {
		fmt.Fprintln(stdout, i18n.T("API Key:  (not set)", "API Key:  (未设置)"))
	}
	return nil
}

func runGenerateSkills(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fmt.Fprintln(stdout, i18n.T("Skills generator (not implemented yet)", "Skills 生成器（待实现）"))
	return nil
}

func resolveBaseURL(baseURLArg string) (string, error) {
	if baseURLArg != "" {
		return baseURLArg, nil
	}
	if envURL := os.Getenv("TIER0_BASE_URL"); envURL != "" {
		return strings.TrimRight(envURL, "/"), nil
	}
	profile, _ := config.LoadProfile()
	if profile.BaseURL != "" {
		return strings.TrimRight(profile.BaseURL, "/"), nil
	}
	return "https://tier0.dev", nil
}

func outputError(stderr io.Writer, jsonMode bool, err error) error {
	var ae *apierr.APIError
	if errors.As(err, &ae) {
		if jsonMode {
			fmt.Fprintln(stderr, ae.Format())
		} else {
			fmt.Fprintf(stderr, "\n✗ %s\n", ae.Message)
			if ae.Hint != "" {
				fmt.Fprintf(stderr, i18n.T("  → %s\n", "  → %s\n"), ae.Hint)
			}
			if ae.HintCommand != "" {
				fmt.Fprintf(stderr, i18n.T("  Run: %s\n", "  执行: %s\n"), ae.HintCommand)
			}
		}
		return err
	}
	if jsonMode {
		msg := strings.ReplaceAll(err.Error(), `"`, `\"`)
		fmt.Fprintf(stderr, `{"ok":false,"error":{"code":0,"message":"%s"}}`+"\n", msg)
	} else {
		fmt.Fprintf(stderr, "\n✗ %v\n", err)
	}
	return err
}
