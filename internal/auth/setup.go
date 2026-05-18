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
	client := &http.Client{Timeout: 10 * time.Second}
	url := strings.TrimRight(baseURL, "/") + "/api/core/cli-auth/status"

	for i := 0; i < maxPollCount; i++ {
		select {
		case <-ctx.Done():
			if onPoll != nil {
				onPoll(i, maxPollCount, false, ctx.Err())
			}
			return SetupResult{}, ctx.Err()
		default:
		}

		body, _ := json.Marshal(map[string]string{"setupCode": setupCode})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return SetupResult{}, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			if onPoll != nil {
				onPoll(i, maxPollCount, false, fmt.Errorf("网络错误: %w", err))
			}
			time.Sleep(pollInterval)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			err := fmt.Errorf("后端尚未支持 CLI 绑定功能（接口 %s 返回 404）", url)
			if onPoll != nil {
				onPoll(i, maxPollCount, false, err)
			}
			return SetupResult{}, err
		}

		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Status        string `json:"status"`
				APIKey        string `json:"apiKey"`
				WorkspaceID   int64  `json:"workspaceID"`
				WorkspaceName string `json:"workspaceName"`
				ExpiresAt     int64  `json:"expiresAt"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			if onPoll != nil {
				onPoll(i, maxPollCount, false, fmt.Errorf("解析响应失败: %w", err))
			}
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()

		if result.Code != 200 {
			err := fmt.Errorf("setup-check failed: %s", result.Msg)
			if onPoll != nil {
				onPoll(i, maxPollCount, false, err)
			}
			return SetupResult{}, err
		}

		switch result.Data.Status {
		case "completed":
			if onPoll != nil {
				onPoll(i, maxPollCount, true, nil)
			}
			return SetupResult{APIKey: result.Data.APIKey}, nil
		case "expired":
			err := fmt.Errorf("绑定码已过期，请重新运行 tier0 login")
			if onPoll != nil {
				onPoll(i, maxPollCount, false, err)
			}
			return SetupResult{}, err
		case "denied":
			err := fmt.Errorf("绑定被拒绝")
			if onPoll != nil {
				onPoll(i, maxPollCount, false, err)
			}
			return SetupResult{}, err
		}

		// pending，继续轮询
		if onPoll != nil {
			onPoll(i, maxPollCount, false, nil)
		}
		time.Sleep(pollInterval)
	}

	err := fmt.Errorf("绑定超时（%d 分钟），请重新运行 tier0 login", maxPollCount*5/60)
	if onPoll != nil {
		onPoll(maxPollCount, maxPollCount, false, err)
	}
	return SetupResult{}, err
}
