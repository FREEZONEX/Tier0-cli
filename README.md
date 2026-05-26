# Tier0 CLI

Tier0 平台命令行工具。

## 安装

### 方式一：npm / npx（推荐，跨平台）

需要 Node.js >= 16。

**一键安装 CLI + Agent Skills（推荐）：**
```bash
npx @tier0/cli@latest install
```

一条命令完成：① 下载对应平台的 Go 二进制到 `~/.tier0/bin/`；② 自动安装 Cursor / Claude Agent Skills。

**全局安装（安装后直接使用 `tier0` 命令）：**
```bash
npm install -g @tier0/cli
```

**特点**：
- 无需 sudo、无需重启终端
- 支持 Linux / macOS / Windows（x86_64 / arm64）

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

## 卸载 / Uninstall

### 一键卸载（推荐）

```bash
npx @tier0/cli@latest uninstall
```

同时移除：CLI 二进制（`~/.tier0/bin/`）、本地 skills 文档（`~/.tier0/skills/`）、Cursor / Claude Agent Skills。

**保留配置文件，彻底清除（含登录凭证）：**
```bash
npx @tier0/cli@latest uninstall --purge
```

**只卸载 CLI，保留 Agent Skills：**
```bash
npx @tier0/cli@latest uninstall --keep-skills
```

### npm 全局卸载

```bash
npm uninstall -g @tier0/cli
```

> `preuninstall` 钩子会自动执行上述清理，等效于 `npx uninstall`。如需跳过清理（CI 环境），可设置环境变量 `TIER0_SKIP_UNINSTALL=1`。

### 手动卸载

```bash
# 删除二进制
rm -rf ~/.tier0/bin

# 删除本地 skills 文档
rm -rf ~/.tier0/skills

# 删除 Agent Skills（如已安装）
npx skills remove FREEZONEX/Tier0-skill

# 可选：删除登录凭证
rm -f ~/.tier0/config.json
```

**Windows (PowerShell)：**
```powershell
Remove-Item -Recurse -Force "$env:USERPROFILE\.tier0\bin"
Remove-Item -Recurse -Force "$env:USERPROFILE\.tier0\skills"
npx skills remove FREEZONEX/Tier0-skill
```

## 版本历史 / Changelog

| Version | Date | Notes |
|---------|------|-------|
| [v0.4.6](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.4.6) | 2026-05-26 | npm 包更名为 `@tier0/cli`；一键安装/卸载 CLI + Agent Skills；修复 login 轮询类型错误 |
| [v0.3.0](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.3.0) | 2026-05-20 | Add `flow` command (Node-RED SourceFlow/EventFlow management); bilingual UI (en/zh), default English |
| [v0.2.4](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.4) | 2026-05-18 | 修复 `login --setup-code` 轮询时 WorkspaceID 类型不匹配导致的 JSON 解码失败 |
| [v0.2.3](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.3) | 2026-05-18 | 新增跨平台一键安装脚本（install.sh / install.ps1），支持 macOS/Linux/Windows 自动安装 |
| [v0.2.2](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.2) | 2026-05-18 | 修复 `login` 读取配置文件中的 baseURL；Skill 文档明确 config 必须在 login 之前 |
| [v0.2.1](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.1) | 2026-05-18 | 新增 `config --base-url` 设置私有化地址；config 支持读取 `TIER0_BASE_URL` 环境变量 |
| [v0.2.0](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.2.0) | 2026-05-18 | 新增 `upgrade`、`skills` 子命令；Release 包预装 skills 文档 |
| [v0.1.0](https://github.com/FREEZONEX/Tier0-cli/releases/tag/v0.1.0) | 2026-05-17 | 初始版本：Device Flow 认证、UNS API 调用、config 管理 |

## 发布流程

### 两层分发说明

| 层级 | 来源 | 作用 |
|------|------|------|
| npm 包 `@tier0/cli` | npm registry | 提供 `bin/tier0` Node 入口脚本 |
| `tier0` 二进制 | GitHub Releases | 由 `install.js` 按平台下载到 `~/.tier0/bin/` |

- npm 包版本与 Go 二进制版本**强制同步**：每次 `release.sh` 都同时发 GitHub Release + `npm publish`，版本号始终一致。

### 发布（一键）

```bash
export GITHUB_TOKEN=ghp_xxxxxxxx   # GitHub PAT，需要 repo 权限
export NPM_TOKEN=npm_xxxxxxxx       # npm Access Token，需要 @tier0 org 发布权限
bash scripts/release.sh v0.4.6
```

脚本自动完成：
1. 交叉编译所有平台二进制
2. 打包并上传到 GitHub Release
3. 同步 `npm-wrapper/package.json` 版本号
4. `npm publish --access public`

### 使用已有 npm login 会话（无 NPM_TOKEN）

```bash
npm login   # 交互登录，登录态持久化到 ~/.npmrc
export GITHUB_TOKEN=ghp_xxxxxxxx
bash scripts/release.sh v0.4.6
```

### 首次发布前确认

```bash
# 确认 npm 登录态
npm whoami

# 确认 @tier0 org 发布权限
npm access list packages @tier0
```
