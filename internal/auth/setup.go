package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/errs"
)

const (
	setupCodeChars  = "ABCDEFGHJKMNPQRSTUVWXYZ23456789" // 排除易混淆字符
	setupCodeLength = 8
	pollInterval    = 5 * time.Second
	maxPollCount    = 120 // 10 分钟
)

// SetupResult CLI 绑定结果
type SetupResult struct {
	APIKey string
}

// GenerateSetupCode 生成随机绑定码
func GenerateSetupCode() string {
	b := make([]byte, setupCodeLength)
	for i := range b {
		b[i] = setupCodeChars[rand.Intn(len(setupCodeChars))]
	}
	return string(b)
}

// BuildConsoleURL 构造控制台授权页面 URL
func BuildConsoleURL(baseURL, setupCode string) string {
	consoleURL := strings.TrimRight(baseURL, "/")
	// 如果 baseURL 包含 /api/ 路径，去掉它
	if idx := strings.LastIndex(consoleURL, "/api/"); idx > 0 {
		consoleURL = consoleURL[:idx]
	}
	return fmt.Sprintf("%s/cli-auth?setup=%s", consoleURL, setupCode)
}

// PollSetupCheck 轮询绑定状态
func PollSetupCheck(ctx context.Context, baseURL, setupCode string, onPoll func(current, total int, done bool, err error)) (SetupResult, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	url := strings.TrimRight(baseURL, "/") + "/api/core/cli-auth/status"

	for i := 0; i < maxPollCount; i++ {
		select {
		case <-ctx.Done():
			netErr := errs.New(errs.CategoryNetwork, 0, "login cancelled").WithRetryable()
			if onPoll != nil {
				onPoll(i, maxPollCount, false, netErr)
			}
			return SetupResult{}, netErr
		default:
		}

		body, _ := json.Marshal(map[string]string{"setupCode": setupCode})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return SetupResult{}, errs.New(errs.CategoryInternal, 0, "build request: "+err.Error())
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			netErr := errs.New(errs.CategoryNetwork, 0, "network error: "+err.Error()).WithRetryable()
			if onPoll != nil {
				onPoll(i, maxPollCount, false, netErr)
			}
			time.Sleep(pollInterval)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			cfgErr := errs.New(errs.CategoryConfig, 404,
				"CLI auth endpoint not found ("+url+"). The server may not support Device Flow login.").
				WithHint("Make sure the Tier0 server is up to date.", "")
			if onPoll != nil {
				onPoll(i, maxPollCount, false, cfgErr)
			}
			return SetupResult{}, cfgErr
		}

		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Status        string `json:"status"`
				APIKey        string `json:"apiKey"`
				WorkspaceID   string `json:"workspaceID"`
				WorkspaceName string `json:"workspaceName"`
				ExpiresAt     string `json:"expiresAt"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			netErr := errs.New(errs.CategoryNetwork, 0, "failed to decode response: "+err.Error()).WithRetryable()
			if onPoll != nil {
				onPoll(i, maxPollCount, false, netErr)
			}
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()

		if result.Code != 200 {
			apiErr := errs.New(errs.CategoryAPI, result.Code, "setup-check failed: "+result.Msg)
			if onPoll != nil {
				onPoll(i, maxPollCount, false, apiErr)
			}
			return SetupResult{}, apiErr
		}

		switch result.Data.Status {
		case "completed":
			if onPoll != nil {
				onPoll(i, maxPollCount, true, nil)
			}
			return SetupResult{APIKey: result.Data.APIKey}, nil
		case "expired":
			authErr := errs.New(errs.CategoryAuthentication, 0,
				"setup code has expired").
				WithHint("Start a new login flow.", "tier0 login")
			if onPoll != nil {
				onPoll(i, maxPollCount, false, authErr)
			}
			return SetupResult{}, authErr
		case "denied":
			authErr := errs.New(errs.CategoryAuthorization, 0,
				"authorization was denied by the user")
			if onPoll != nil {
				onPoll(i, maxPollCount, false, authErr)
			}
			return SetupResult{}, authErr
		}

		// pending — continue polling
		if onPoll != nil {
			onPoll(i, maxPollCount, false, nil)
		}
		time.Sleep(pollInterval)
	}

	timeoutErr := errs.New(errs.CategoryAuthentication, 0,
		"login timed out after 10 minutes").
		WithHint("Re-run to start a new Device Flow.", "tier0 login")
	if onPoll != nil {
		onPoll(maxPollCount, maxPollCount, false, timeoutErr)
	}
	return SetupResult{}, timeoutErr
}
