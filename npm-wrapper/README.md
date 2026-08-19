# @tier0/cli

Tier0 CLI npm wrapper.

## Install

Recommended:

```bash
npx -y @tier0/cli@latest install
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

## MQTT Quick Start

```bash
tier0 mqtt auth create --name agent --save agent --random-suffix=true
tier0 mqtt subscribe --credential agent --topic 'Plant/+/State/Status' --timeout 60s --json
tier0 mqtt publish --credential agent --topic Plant/Line1/State/Status --file payload.json --json-message --qos 1
tier0 mqtt auth delete --credential agent --yes
```

## Uninstall

```bash
npx @tier0/cli@latest uninstall
npx @tier0/cli@latest uninstall --purge
npx @tier0/cli@latest uninstall --remove-skills
```

The Agent Skill and credentials are kept by default. Use `--remove-skills` to
delete the Skill and `--purge` to delete credentials.

## How It Works

- Detects the current OS and CPU architecture.
- Downloads the matching Go binary from Tier0 CLI GitHub Releases.
- Caches the binary under `~/.tier0/bin/`.
- Materializes the trusted Skill baseline embedded in the verified CLI binary and copies it globally to detected agents such as Codex, Claude Code, and Cursor.
- Keeps independent Skill updates available through `tier0 skills update`, which also resynchronizes detected agents.
- Supports Linux, macOS, and Windows on x86_64 and arm64.
