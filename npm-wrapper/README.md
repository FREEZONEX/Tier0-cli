# @tier0/cli

Tier0 CLI npm wrapper.

## Install

Recommended:

```bash
npx @tier0/cli@latest install
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
- Extracts the versioned Skill from the CLI Release and copies it globally to detected agents such as Codex, Claude Code, and Cursor.
- Keeps independent Skill updates available through `tier0 skills update`, which also resynchronizes detected agents.
- Supports Linux, macOS, and Windows on x86_64 and arm64.
