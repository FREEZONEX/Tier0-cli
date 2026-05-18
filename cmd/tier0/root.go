package tier0

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/auth"
	"github.com/FREEZONEX/Tier0-cli/internal/client"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/FREEZONEX/Tier0-cli/internal/version"
)

// Execute 执行根命令
func Execute() error {
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
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", cmd)
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "tier0 — Tier0 平台命令行工具")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "用法: tier0 <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "命令:")
	fmt.Fprintln(w, "  login             Device Flow 登录授权")
	fmt.Fprintln(w, "  api <endpoint>    调用 API 接口")
	fmt.Fprintln(w, "  config            查看/管理配置")
	fmt.Fprintln(w, "  skills            管理 Skills（list/update/version）")
	fmt.Fprintln(w, "  upgrade           升级 CLI 到最新版本")
	fmt.Fprintln(w, "  generate-skills   生成 Skills 文档")
	fmt.Fprintln(w, "  version           显示版本")
	fmt.Fprintln(w, "  help              显示帮助")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "环境变量:")
	fmt.Fprintln(w, "  TIER0_BASE_URL    平台地址 (默认: https://tier0.dev/)")
	fmt.Fprintln(w, "  TIER0_API_KEY     API Key")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "示例:")
	fmt.Fprintln(w, "  tier0 config --base-url https://tier0-eks-frontend.tier0.dev")
	fmt.Fprintln(w, "  tier0 login")
	fmt.Fprintln(w, "  tier0 login --no-wait")
	fmt.Fprintln(w, "  tier0 api /openapi/v1/uns/read --body '{\"topics\":[\"demo\"]}'")
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

	// --setup-code 模式：直接轮询
	if setupCode != "" {
		return runLoginPoll(ctx, baseURL, setupCode, jsonMode, stdout, stderr)
	}

	// 生成 setupCode
	setupCode = auth.GenerateSetupCode()
	consoleURL := auth.BuildConsoleURL(baseURL, setupCode)

	// --no-wait 模式
	if noWait {
		if jsonMode {
			fmt.Fprintf(stdout, `{"status":"authorization_required","verification_url":"%s","setup_code":"%s","expires_in":600}`+"\n", consoleURL, setupCode)
		} else {
			fmt.Fprintf(stdout, "请在浏览器中完成授权：%s\n", consoleURL)
			fmt.Fprintf(stdout, "授权完成后执行: tier0 login --setup-code %s\n", setupCode)
		}
		return nil
	}

	// 默认模式：显示 URL + 阻塞轮询
	fmt.Fprintln(stdout, "请在浏览器中完成授权：")
	fmt.Fprintln(stdout, consoleURL)
	fmt.Fprintln(stdout, "\n正在等待授权...（每5秒检测一次，最多10分钟）")

	return runLoginPoll(ctx, baseURL, setupCode, jsonMode, stdout, stderr)
}

func runLoginPoll(ctx context.Context, baseURL, setupCode string, jsonMode bool, stdout, stderr io.Writer) error {
	result, err := auth.PollSetupCheck(ctx, baseURL, setupCode, func(current, total int, done bool, pollErr error) {
		if jsonMode || done {
			return
		}
		if pollErr != nil {
			if current == 0 {
				fmt.Fprintf(stdout, "\r  正在检测...（第 %d/%d 次）网络暂时不稳定，继续等待...\n", current+1, total)
			}
			return
		}
		if current%6 == 0 && current > 0 {
			remainingMin := (total - current) * 5 / 60
			fmt.Fprintf(stdout, "\r  正在等待授权...（第 %d/%d 次检测，剩余约 %d 分钟）", current+1, total, remainingMin)
		}
	})
	if err != nil {
		return outputError(stderr, jsonMode, err)
	}

	// 保存配置
	profile := config.Profile{
		BaseURL: baseURL,
		APIKey:  result.APIKey,
	}
	if err := config.SaveProfile(profile); err != nil {
		return outputError(stderr, jsonMode, fmt.Errorf("保存配置失败: %w", err))
	}

	if jsonMode {
		fmt.Fprintf(stdout, `{"event":"authorization_complete","api_key":"%s"}`+"\n", result.APIKey)
	} else {
		fmt.Fprintln(stdout, "\n✓ 授权成功！")
		fmt.Fprintf(stdout, "API Key: %s...（已保存）\n", result.APIKey[:8])
		fmt.Fprintln(stdout, "✓ 初始化完成，您现在可以使用 tier0 api 命令了。")
	}
	return nil
}

func runAPI(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "用法: tier0 api <endpoint> [--body '<json>'] [--method GET|POST]")
		return fmt.Errorf("missing endpoint")
	}

	endpoint := args[0]
	var body string
	var method string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--body":
			if i+1 < len(args) {
				body = args[i+1]
				i++
			}
		case "--method":
			if i+1 < len(args) {
				method = args[i+1]
				i++
			}
		}
	}

	profile, err := config.LoadProfile()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	if profile.APIKey == "" {
		return fmt.Errorf("未找到 API Key，请先运行 tier0 login")
	}

	c := client.New(profile.BaseURL, profile.APIKey)
	resp, err := c.DoAPI(ctx, endpoint, method, body)
	if err != nil {
		return fmt.Errorf("API 调用失败: %w", err)
	}

	fmt.Fprintln(stdout, resp)
	return nil
}

func runConfig(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var setBaseURL string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base-url":
			if i+1 < len(args) {
				setBaseURL = strings.TrimRight(args[i+1], "/")
				i++
			}
		}
	}

	// 设置模式
	if setBaseURL != "" {
		profile, _ := config.LoadProfile()
		profile.BaseURL = setBaseURL
		if err := config.SaveProfile(profile); err != nil {
			return fmt.Errorf("保存配置失败: %w", err)
		}
		fmt.Fprintf(stdout, "✓ BaseURL 已设置为: %s\n", setBaseURL)
		return nil
	}

	// 查看模式
	profile, err := config.LoadProfile()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 优先使用环境变量，其次配置文件，最后默认值
	baseURL, _ := resolveBaseURL("")
	if profile.BaseURL != "" {
		baseURL = profile.BaseURL
	}
	fmt.Fprintf(stdout, "BaseURL: %s\n", baseURL)
	if profile.APIKey != "" {
		fmt.Fprintf(stdout, "API Key: %s...\n", profile.APIKey[:8])
	} else {
		fmt.Fprintln(stdout, "API Key: (未设置)")
	}
	return nil
}

func runGenerateSkills(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fmt.Fprintln(stdout, "Skills 生成器（待实现）")
	return nil
}

func resolveBaseURL(baseURLArg string) (string, error) {
	if baseURLArg != "" {
		return baseURLArg, nil
	}
	if envURL := os.Getenv("TIER0_BASE_URL"); envURL != "" {
		return strings.TrimRight(envURL, "/"), nil
	}
	// 最后读取配置文件中的 baseURL
	profile, _ := config.LoadProfile()
	if profile.BaseURL != "" {
		return strings.TrimRight(profile.BaseURL, "/"), nil
	}
	return "https://tier0.dev", nil
}

func outputError(stderr io.Writer, jsonMode bool, err error) error {
	if jsonMode {
		fmt.Fprintf(stderr, `{"event":"error","error":"%s"}`+"\n", err.Error())
	} else {
		fmt.Fprintf(stderr, "\n✗ %v\n", err)
	}
	return err
}
