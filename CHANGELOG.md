# Changelog

All notable changes to Tier0 CLI are documented here.

---

## [v0.5.0] — 2026-06-01

### ✨ Features

- **结构化错误分类体系**：全面引入 8 类语义化错误，每类对应稳定的 shell exit code，AI Agent 和脚本可直接按 `error.type` 分支处理

  | `error.type` | exit | 场景 |
  |---|---|---|
  | `validation` | 2 | 参数错误、Flag 缺失 |
  | `authentication` | 3 | API Key 未配置或过期 |
  | `authorization` | 3 | 权限不足 |
  | `config` | 3 | 配置文件缺失或 base URL 未设置 |
  | `network` | 4 | 网络超时、连接失败（可自动重试） |
  | `api` | 1 | 服务端错误 |
  | `internal` | 5 | CLI 内部异常 |
  | `confirmation` | 10 | 高危操作需 `--yes` 确认 |

- **统一 JSON 错误 envelope**：所有错误路径（API 错误、网络错误、配置错误、认证错误）现在输出同一结构，新增 `hint_command` 和 `retryable` 字段

  ```json
  {
    "ok": false,
    "error": {
      "type": "authentication",
      "code": 401,
      "message": "API Key is missing or expired.",
      "hint": "Authenticate first.",
      "hint_command": "tier0 login",
      "retryable": false
    }
  }
  ```

- **网络错误自动标记可重试**：`network` 类错误携带 `"retryable": true`，AI Agent 可据此驱动自动重试逻辑

- **请求追踪头**：所有 API 请求自动携带 `X-Tier0-Source: tier0-cli`，便于后端日志按来源过滤

- **auth whoami**：新增 `tier0 auth whoami`，查看当前 API Key 绑定的用户、Workspace、Key 类型、角色和权限

- **flow nodes**：新增 `tier0 flow nodes --source|--event`，查询 SourceFlow / EventFlow 当前实际可用的 Node-RED 节点类型

### 🛠 Improvements

- **login Device Flow 错误分类**：登录轮询失败（网络抖动 / setup code 过期 / 授权被拒 / 10 分钟超时）现在分别归入对应 Category，exit code 语义化
- **Cobra 错误静默**：根命令加 `SilenceErrors: true`，避免同一错误被 Cobra + HandleCommandError + main 三层重复打印到 stderr
- **exit code 精确化**：认证/配置/权限类错误统一 exit 3；网络错误 exit 4；之前所有非高危错误一律 exit 1

---

## [v0.4.16] — 2026-05-27

### 🛠 Improvements

- **release**：新增 `scripts/release.sh` 自动化发布脚本，支持一键打包 + 发布 GitHub Release + npm publish

---

## [v0.4.15] — 2026-05-27

### ✨ Features

- **uns create `--type` 简化**：`--type` 现在只接受 `path`（目录）和 `topic`（数据点），旧值 `METRIC` / `ACTION` / `STATE` / `file` / `folder` 仍可用但会打印废弃警告
- **topicType 自动推导**：无需手动指定 `--topic-type`，CLI 自动从路径倒数第二段（`Metric` / `Action` / `State`）推导，更简洁

  ```bash
  # 路径含 Metric 自动推导 topicType=METRIC
  tier0 uns create --topic Plant/Line1/Metric/Temperature --type topic \
    --fields '[{"name":"value","type":"float","unit":"°C"}]'
  ```

### 🐛 Bug Fixes

- **uns create**：修复多层路径展开时中间 folder 节点未正确生成的问题
- **uns create**：修复 `--file` 接受裸数组 `[...]` 时解析失败的问题（现在同时支持 `{"namespace":[...]}` 和裸数组两种格式）

---

## [v0.4.14] — 2026-05-27

### ✨ Features

- **uns create `--parent`**：新增 `--parent` 标志，支持在已有路径前缀下只建子节点，避免重复写完整路径

  ```bash
  tier0 uns create --parent Plant/Line1 --topic Metric/Temperature --type topic
  ```

- **uns create 批量错误上报**：`--file` 批量创建时逐项检查 `data.results[i].success`，部分失败不再静默，会完整报告哪些节点创建失败及原因
- **CheckResponse 批量校验**：新增通用批量响应校验，适用于 uns read / write / history / create 等所有返回 `data.results[]` 的接口

---

## [v0.4.13] — 2026-05-26

### ✨ Features

- **upgrade**：升级策略全面改进，与 lark-cli 对齐：
  - 版本检查优先走 npm registry（无限速），失败自动降级到 npmmirror.com，最后兜底 GitHub API
  - npm 可用时直接运行 `npm install -g @tier0/cli@<version>`（不再直接操作二进制文件），npm 失败自动降级到 GitHub 直链下载
  - npm install 失败时自动切换 `--registry https://registry.npmmirror.com` 重试，对中国用户友好

---

## [v0.4.12] — 2026-05-26

### ✨ Features

- **login**：Agent 登录流程优化，无需等待用户回复即可自动轮询。推荐流程：先 `tier0 login --no-wait --json` 获取 URL 展示给用户，立即接 `tier0 login --setup-code <code>` 自动轮询，用户浏览器授权后自动保存 API Key

### 🐛 Bug Fixes

- **login**：`--no-wait` 非 JSON 模式现在也会输出 `setup_code` 字段，方便 agent 解析

---

## [v0.4.11] — 2026-05-26

### 🐛 Bug Fixes

- **uns create / write / update / delete / restore**：修复写操作在后端返回 HTTP 200 + `{"code":非零}` 时仍打印"创建成功"的假阳性问题，现在会正确报错并显示后端错误信息
- **flow create / update / deploy / delete**：同上，补充 `CheckOK` 校验，避免业务层错误被忽略

---

## [v0.4.10] — 2026-05-26

### ✨ Features

- **config**：新增 `--api-key` 标志，可直接设置 API Key（替代 `tier0 login` Device Flow）
  ```bash
  tier0 config --api-key sk-per-xxxxxx
  ```

---

## [v0.4.9] — 2026-05-26

### ✨ Features

- 新增 `tier0 uninstall` Go 命令（`--purge` 彻底清除含凭证，`--keep-skills` 保留 Agent Skills）

### 🐛 Bug Fixes

- **install**：版本检测改为直接读 `package.json`，不再调 GitHub API `/releases/latest`，彻底解决安装旧版本的问题（参考 Lark CLI 方案）
- **release.sh**：GitHub Release JSON payload 改用 Node.js `JSON.stringify` 生成，修复 bash `printf \n` 导致的 HTTP 400
- **release.sh**：GitHub Release 失败时自动跳过 `npm publish`，防止发布不完整的版本

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
