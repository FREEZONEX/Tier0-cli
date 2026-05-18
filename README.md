# Tier0 CLI

Tier0 平台命令行工具。

## 安装

### 方式一：下载 Release 包（推荐）

从 [GitHub Releases](https://github.com/FREEZONEX/Tier0-cli/releases) 下载对应平台的预编译包：

```bash
# Linux x86_64
curl -LO https://github.com/FREEZONEX/Tier0-cli/releases/latest/download/tier0-cli-Linux-x86_64.tar.gz
tar -xzf tier0-cli-Linux-x86_64.tar.gz
sudo mv linux-amd64/tier0 /usr/local/bin/

# macOS Apple Silicon
curl -LO https://github.com/FREEZONEX/Tier0-cli/releases/latest/download/tier0-cli-macOS-arm64.tar.gz
tar -xzf tier0-cli-macOS-arm64.tar.gz
sudo mv darwin-arm64/tier0 /usr/local/bin/

# Windows (PowerShell)
# 下载 tier0-cli-Windows-x86_64.zip 并解压，将 tier0.exe 添加到 PATH
```

Release 包已包含 skills 文档，开箱即用。

### 方式二：go install（仅二进制，不含 skills）

```bash
go install github.com/FREEZONEX/Tier0-cli@latest
```

> 注意：`go install` 仅安装二进制文件，不含 skills 文档。

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

## 发布流程

### 构建 Release 包

```bash
# 构建所有平台并打包（输出到 dist/release-vX.X.X/packages/）
bash scripts/release.sh v0.2.0

# 单独打包 skills
bash scripts/package-skill.sh ./dist/skills --version v0.2.0
```

### 发布到 GitHub

```bash
# 方式 1：脚本自动发布（需设置 GITHUB_TOKEN）
export GITHUB_TOKEN=ghp_xxxxxxxx
bash scripts/release.sh v0.2.0

# 方式 2：使用 gh CLI
cd dist/release-v0.2.0/packages
gh release create v0.2.0 --repo FREEZONEX/Tier0-cli --title "tier0-cli v0.2.0" --notes "..." *
```
