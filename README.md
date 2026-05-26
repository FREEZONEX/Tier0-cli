# Tier0 CLI

Tier0 平台命令行工具。

## 安装

### 方式一：npm / npx（推荐，跨平台）

需要 Node.js >= 16。

**一行安装（推荐）：**
```bash
npx @tier0/cli@latest install
```

**全局安装（安装后直接使用 `tier0` 命令）：**
```bash
npm install -g @tier0/cli
```

**特点**：
- 首次运行自动下载对应平台的 Go 二进制
- 无需 sudo、无需重启终端
- 支持 Linux / macOS / Windows

**npx 直接运行（无需全局安装）：**
```bash
npx @tier0/cli@latest login
```

### 方式二：一键脚本

**macOS / Linux：**

```bash
curl -sL https://raw.githubusercontent.com/FREEZONEX/Tier0-cli/main/install.sh | bash
```

**Windows (PowerShell)：**

```powershell
Invoke-RestMethod -Uri https://raw.githubusercontent.com/FREEZONEX/Tier0-cli/main/install.ps1 | Invoke-Expression
```

脚本自动检测平台架构，下载对应 Release 包，安装到 `~/.tier0/bin`，自动配置 PATH。

### 方式三：手动下载

从 [GitHub Releases](https://github.com/FREEZONEX/Tier0-cli/releases) 下载对应平台的预编译包，解压后将二进制放到 PATH 中。

### 方式四：go install（仅二进制，不含 skills）

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

### 管理 Flow（Node-RED）

每个 Workspace 包含两类 Node-RED 容器，通过 `tier0 flow` 子命令统一管理：

| 类型 | 说明 |
|------|------|
| `SourceFlow` | 连接协议采集数据并发布 MQTT |
| `EventFlow`  | 对业务数据进行二次处理 |

```bash
# 列出所有 Flow
tier0 flow list

# 只看 SourceFlow / EventFlow
tier0 flow list --source
tier0 flow list --event

# 按名称关键词过滤，JSON 输出
tier0 flow list --keyword "modbus" --json

# 查看 Flow 详情
tier0 flow get --id 1

# 创建 Flow
tier0 flow create --name "modbus-collector" --source --desc "Modbus 数据采集"
tier0 flow create --name "alert-handler"    --event

# 更新 Flow（名称、描述、收藏）
tier0 flow update --id 1 --name "new-name" --favorite

# 删除 Flow（支持多个 ID）
tier0 flow delete --id 1 --id 2
tier0 flow delete 1,2,3

# 导出 Node-RED 画布 JSON 到文件
tier0 flow data --id 1 --out flows.json

# 部署 Node-RED 画布（从文件，推荐）
tier0 flow deploy --id 1 -f flows.json

# 部署 Node-RED 画布（内联 JSON）
tier0 flow deploy --id 1 --flows-json '[{"id":"abc","type":"tab","label":"Flow 1"}]'

# 所有子命令均支持 --debug 查看 HTTP 详情
tier0 flow list --debug
```

运行 `tier0 flow help` 查看完整选项说明。

### 查看配置

```bash
tier0 config
```

## 语言 / Language

The CLI defaults to **English**. Switch to Chinese with:

```bash
# Persist to config file
tier0 config --lang zh

# One-off override via environment variable
TIER0_LANG=zh tier0 flow list
```

Switch back to English:

```bash
tier0 config --lang en
```

The active language is stored in `~/.tier0/config.json` as `"lang": "zh"` (mirrors how `lark-cli` handles it).
Priority: `TIER0_LANG` env var > config file > default (`en`).

## 环境变量 / Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TIER0_BASE_URL` | Platform base URL | `https://tier0.dev` |
| `TIER0_API_KEY` | API Key (overrides config file) | — |
| `TIER0_LANG` | UI language: `en` \| `zh` | `en` |

## 认证流程

1. CLI 生成 `setupCode`
2. 打开浏览器：`https://tier0.dev/cli-auth?setup=<code>`
3. 用户登录并选择 Workspace
4. 后端创建 Personal API Key
5. CLI 轮询获取 API Key
6. 保存到 `~/.tier0/config.json`

## 配置

```bash
# 查看当前配置
tier0 config

# 设置私有化平台地址（写入配置文件，持久生效）
tier0 config --base-url https://tier0-eks-frontend.tier0.dev

# 登录时会自动使用配置文件中的地址
tier0 login
```

> **优先级**：`--base-url` 参数 > 环境变量 `TIER0_BASE_URL` > 配置文件 > 默认地址

## 版本历史 / Changelog

| Version | Date | Notes |
|---------|------|-------|
| [v0.3.0](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.3.0) | 2026-05-20 | Add `flow` command (Node-RED SourceFlow/EventFlow management); bilingual UI (en/zh), default English |
| [v0.2.4](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.4) | 2026-05-18 | 修复 `login --setup-code` 轮询时 WorkspaceID 类型不匹配导致的 JSON 解码失败 |
| [v0.2.3](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.3) | 2026-05-18 | 新增跨平台一键安装脚本（install.sh / install.ps1），支持 macOS/Linux/Windows 自动安装 |
| [v0.2.2](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.2) | 2026-05-18 | 修复 `login` 读取配置文件中的 baseURL；Skill 文档明确 config 必须在 login 之前 |
| [v0.2.1](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.1) | 2026-05-18 | 新增 `config --base-url` 设置私有化地址；config 支持读取 `TIER0_BASE_URL` 环境变量 |
| [v0.2.0](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.0) | 2026-05-18 | 新增 `upgrade`、`skills` 子命令；Release 包预装 skills 文档 |
| [v0.1.0](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.1.0) | 2026-05-17 | 初始版本：Device Flow 认证、UNS API 调用、config 管理 |

## 发布流程

### 构建 Release 包

```bash
# 构建所有平台并打包（输出到 dist/release-vX.X.X/packages/）
bash scripts/release.sh v0.2.1

# 单独打包 skills
bash scripts/package-skill.sh ./dist/skills --version v0.2.1
```

### 发布到 GitHub

```bash
# 方式 1：脚本自动发布（需设置 GITHUB_TOKEN）
export GITHUB_TOKEN=ghp_xxxxxxxx
bash scripts/release.sh v0.2.1

# 方式 2：使用 gh CLI
cd dist/release-v0.2.1/packages
gh release create v0.2.1 --repo FREEZONEX/Tier0-cli --title "tier0-cli v0.2.1" --notes "..." *
```
