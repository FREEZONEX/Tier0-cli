# @tier0/cli

Tier0 CLI npm wrapper — 无需手动下载，一行命令即用。

## 安装

**一行安装（推荐）：**
```bash
npx @tier0/cli@latest install
```

**全局安装：**
```bash
npm install -g @tier0/cli
```

**不安装直接运行（npx）：**
```bash
npx @tier0/cli@latest login
```

## 使用

全局安装后，`tier0` 命令立即可用：

```bash
# 查看版本
tier0 version

# 私有化部署先配置地址
tier0 config --base-url https://your-domain.com

# 登录授权
tier0 login

# 调用 API
tier0 api /openapi/v1/uns/browse --body '{path:/}'
```

**Windows (PowerShell)：**
```powershell
npx @tier0/cli@latest install

# 立即可用，无需重启终端
tier0 version
```

## 机制

- `npx @tier0/cli@latest install` 会从 [GitHub Release](https://github.com/FREEZONEX/Tier0-cli/releases) 下载最新 Go 二进制并安装到 `~/.tier0/bin/`
- 二进制缓存到 `~/.tier0/bin/`，后续调用直接使用本地缓存
- 支持 Linux、macOS、Windows（x86_64 / arm64）
