# @tier0/cli

Tier0 CLI npm wrapper.

## Install

Recommended:

```bash
npx @tier0/cli@latest
```

Global install:

```bash
npm install -g @tier0/cli
tier0 --help
```

Run directly:

```bash
npx @tier0/cli@latest --help
```

## How It Works

- Detects the current OS and CPU architecture.
- Downloads the matching Go binary from Tier0 CLI GitHub Releases.
- Caches the binary under `~/.tier0/bin/`.
- Installs Agent Skills from `FREEZONEX/Tier0-skill`.
- Supports Linux, macOS, and Windows on x86_64 and arm64.
