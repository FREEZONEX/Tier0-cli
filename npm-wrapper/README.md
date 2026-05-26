# @tier0/cli

Tier0 CLI npm wrapper — 一键安装 CLI 和 Agent Skills。

## 安装

**一键安装（推荐）：**
```bash
npx @tier0/cli@latest install
```

一条命令完成：
1. 下载对应平台的 `tier0` Go 二进制到 `~/.tier0/bin/`
2. 自动安装 Cursor / Claude Agent Skills（`FREEZONEX/Tier0-skill`）

**全局安装：**
```bash
npm install -g @tier0/cli
```

**npx 直接运行（无需安装）：**
```bash
npx @tier0/cli@latest login
```

## 使用

全局安装后，`tier0` 命令立即可用：

```bash
tier0 version
tier0 config --base-url https://your-domain.com
tier0 login
tier0 api /openapi/v1/uns/browse --body '{path:/}'
```

**Windows (PowerShell)：**
```powershell
npx @tier0/cli@latest install
tier0 version
```

## 机制

- 检测当前平台与架构，从 [GitHub Release](https://github.com/FREEZONEX/Tier0-cli/releases) 下载对应的 Go 二进制
- 二进制缓存到 `~/.tier0/bin/`，后续调用直接使用本地缓存
- 安装完成后自动调用 `npx skills add FREEZONEX/Tier0-skill` 安装 Agent Skills（失败时非致命，可手动补装）
- 支持 Linux、macOS、Windows（x86_64 / arm64）
