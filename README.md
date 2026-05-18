# Tier0 CLI

Tier0 平台命令行工具。

## 安装

```bash
go install github.com/FREEZONEX/Tier0-cli@latest
```

## 用法

### 登录授权

```bash
# 交互模式（默认）
tier0 login

# AI 友好模式（输出 URL 后退出，不阻塞）
tier0 login --no-wait

# 指定平台地址
tier0 login --base-url https://tier0.dev
```

### 调用 API

```bash
# 读取 UNS 节点
tier0 api /openapi/v1/uns/read --body '{"topics":["demo"]}'

# 浏览命名空间
tier0 api /openapi/v1/uns/browse --body '{"path":"/"}'
```

### 查看配置

```bash
tier0 config
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `TIER0_BASE_URL` | 平台地址 | `https://tier0.dev` |
| `TIER0_API_KEY` | API Key（优先级高于配置文件） | - |

## 认证流程

1. CLI 生成 `setupCode`
2. 打开浏览器：`https://tier0.dev/cli-auth?setup=<code>`
3. 用户登录并选择 Workspace
4. 后端创建 Personal API Key
5. CLI 轮询获取 API Key
6. 保存到 `~/.tier0/config.json`
