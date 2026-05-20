package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
)

// Client HTTP API 客户端
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New 创建 API 客户端
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// DoAPI 调用 API 接口
func (c *Client) DoAPI(ctx context.Context, endpoint, method, body string, debug bool) (string, error) {
	if method == "" {
		method = http.MethodPost
	}

	url := c.baseURL + endpoint
	body = fixJSON(body)
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewReader([]byte(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[debug] ---------- HTTP Request ----------\n")
		fmt.Fprintf(os.Stderr, "[debug] %s %s\n", req.Method, req.URL.String())
		for key, values := range req.Header {
			for _, v := range values {
				if strings.EqualFold(key, "x-api-key") {
					v = v[:min(len(v), 8)] + "..."
				}
				fmt.Fprintf(os.Stderr, "[debug] %s: %s\n", key, v)
			}
		}
		if body != "" {
			fmt.Fprintf(os.Stderr, "[debug] Body: %s\n", body)
		}
		fmt.Fprintf(os.Stderr, "[debug] ----------------------------------\n")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[debug] ---------- HTTP Response ---------\n")
		fmt.Fprintf(os.Stderr, "[debug] Status: %d %s\n", resp.StatusCode, resp.Status)
		if len(respBody) > 4096 {
			fmt.Fprintf(os.Stderr, "[debug] Body: %s... (%d bytes truncated)\n", string(respBody[:4096]), len(respBody))
		} else {
			fmt.Fprintf(os.Stderr, "[debug] Body: %s\n", string(respBody))
		}
		fmt.Fprintf(os.Stderr, "[debug] ----------------------------------\n")
	}

	if resp.StatusCode >= 400 {
		return "", apierr.New(resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

// fixJSON 尝试修复 PowerShell 等环境导致的 JSON 引号丢失
// 例如 {path:/} → {"path":"/"}
func fixJSON(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return body
	}
	// 已经是合法 JSON，无需修复
	if json.Valid([]byte(trimmed)) {
		return trimmed
	}

	// 修复 Object: 给无引号的 key 和 string value 加上引号
	// 步骤 1: 给 key 加引号  {key: → {"key":
	keyRe := regexp.MustCompile(`([{,]\s*)([A-Za-z_][A-Za-z0-9_]*)\s*:`)
	fixed := keyRe.ReplaceAllString(trimmed, `$1"$2":`)

	// 步骤 2: 给无引号的 string value 加引号（排除数字、布尔、null）
	// 匹配 :value 或 ,value 后面跟 } 或 , 的情况
	valRe := regexp.MustCompile(`(:\s*)([^"{}\[\],\s\d][^,}\]]*)([,}\]])`)
	fixed = valRe.ReplaceAllString(fixed, `$1"$2"$3`)

	// 步骤 3: 处理数组中的无引号 string
	arrValRe := regexp.MustCompile(`([,\[]\s*)([^"{}\[\],\s\d][^,\]]*)([,\]])`)
	fixed = arrValRe.ReplaceAllString(fixed, `$1"$2"$3`)

	// 兜底：如果修复后仍不合法，返回原始值（让后端报错提示）
	if json.Valid([]byte(fixed)) {
		return fixed
	}
	return trimmed
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
