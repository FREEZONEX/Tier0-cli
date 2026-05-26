# Changelog

All notable changes to Tier0 CLI are documented here.

---

## [v0.4.6] — 2026-05-26

### ✨ Features

**一键安装 CLI + Agent Skills**

```bash
npx @tier0/cli@latest install
```

- npm 包从 `@freezonex/tier0-cli` 更名为 **`@tier0/cli`**
- 新增 `install` 命令：一条命令同时下载 `tier0` 二进制到 `~/.tier0/bin/`，并自动安装 Cursor / Claude Agent Skills（`FREEZONEX/Tier0-skill`）
- `npm install -g @tier0/cli` 的 `postinstall` 钩子同样自动安装 Agent Skills

**一键卸载**

```bash
npx @tier0/cli@latest uninstall            # 卸载 CLI + Skills，保留 config.json
npx @tier0/cli@latest uninstall --purge    # 彻底清除（含登录凭证）
npx @tier0/cli@latest uninstall --keep-skills  # 只卸载 CLI 二进制
```

- `npm uninstall -g @tier0/cli` 通过 `preuninstall` 钩子自动触发清理
- CI 环境可设置 `TIER0_SKIP_UNINSTALL=1` 跳过清理

**发布流程同步**

- `scripts/release.sh` 在发 GitHub Release 时同步执行 `npm publish`，两侧版本号始终一致
- 支持 `NPM_TOKEN` 环境变量认证
- 修复 GitHub Release 已存在（HTTP 422）时自动获取现有 release 的 `upload_url` 并追加上传

### 🐛 Bug Fixes

- **login**：修复 `--setup-code` 轮询时 `workspaceID` / `expiresAt` 字段 JSON 类型不匹配导致的解码失败
- **login**：修复轮询过程中网络错误无任何反馈的问题
- **release.sh**：修复 Windows Git Bash 下 Node.js 路径解析错误（`/d/repo/` 被解析为 `D:\d\repo\`）

---

## [v0.4.4] — 2026-05-22

- `tier0 uns history`：修复相对时间解析及 API 字段名称错误
- `tier0 uns history`：将 `--topics` 标志统一改为 `--topic`（与其他子命令一致）
- 修复 `--json` 输出缺失及 `flow list` / `flow get` 返回空数据的问题

---

## [v0.3.0] — 2026-05-20

### ✨ Features

- 新增 `tier0 flow` 子命令，支持 Node-RED Flow 全生命周期管理：
  - `flow list` / `flow get` / `flow create` / `flow update` / `flow delete`
  - `flow data`（导出画布 JSON）/ `flow deploy`（部署画布）
- 支持 SourceFlow（协议采集）和 EventFlow（业务处理）两种 Flow 类型
- 双语 UI（中文 / 英文），默认英文；支持 `tier0 config --lang zh` 切换
- 新增 `TIER0_LANG` 环境变量临时覆盖语言
- 高风险操作（`flow deploy` / `flow delete`）需 `--yes` 确认，退出码 10 + 结构化 JSON 错误

---

## [v0.2.4] — 2026-05-18

- 修复 `login --setup-code` 轮询时 `workspaceID` 类型不匹配导致的 JSON 解码失败

---

## [v0.2.3] — 2026-05-18

- 新增跨平台一键安装脚本 `install.sh`（macOS/Linux）和 `install.ps1`（Windows）
- 自动检测平台架构，下载对应 Release 包，安装到 `~/.tier0/bin/`，自动配置 PATH

---

## [v0.2.2] — 2026-05-18

- 修复 `login` 未读取配置文件中的 `baseURL` 的问题
- Skill 文档明确：私有化部署必须先 `config --base-url` 再 `login`

---

## [v0.2.1] — 2026-05-18

- 新增 `tier0 config --base-url` 设置私有化平台地址（持久化到配置文件）
- `config` 命令支持读取 `TIER0_BASE_URL` 环境变量

---

## [v0.2.0] — 2026-05-18

- 新增 `tier0 upgrade`：自升级 CLI 二进制
- 新增 `tier0 skills list` / `skills update` / `skills version`：管理 AI Agent Skills 文档
- Release 包预装 skills 文档到 `skill/` 目录，安装时自动同步到 `~/.tier0/skills/`

---

## [v0.1.0] — 2026-05-17

- 初始版本
- Device Flow 认证（`tier0 login`）
- UNS API 代理（`tier0 api`）
- 配置管理（`tier0 config`）
- 配置文件：`~/.tier0/config.json`
