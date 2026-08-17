# Tier0 CLI

Tier0 platform command-line tool.

## Install

Recommended, cross-platform, requires Node.js >= 16:

```bash
npx -y @tier0/cli@latest install
```

This installs the Go `tier0` binary into `~/.tier0/bin/`, materializes its trusted embedded Skill baseline into `~/.tier0/skills/`, and copies it globally to detected agents such as Codex, Claude Code, and Cursor. Release archives no longer carry a duplicate `skill/` directory.

Then run `tier0 auth whoami --json`. If authentication is missing, run
`tier0 login --no-wait --json` and open the returned `verification_url`.

Skills can still be updated independently of the CLI. `tier0 skills update` downloads the latest `FREEZONEX/Tier0-skill` content into `~/.tier0/skills/` and resynchronizes detected agents.

Use `tier0 skills status` to inspect provenance and health. `tier0 skills install` repairs a missing or damaged embedded baseline without overwriting an independently updated remote Skill; `tier0 skills install --force` explicitly resets to the baseline compiled into the current CLI.

Global install:

```bash
npm install -g @tier0/cli
tier0 --help
```

Run without global install:

```bash
npx @tier0/cli@latest --help
```

Shell installer:

```bash
curl -fsSL https://raw.githubusercontent.com/FREEZONEX/Tier0-cli/main/install.sh | bash
```

Windows PowerShell:

```powershell
iwr https://raw.githubusercontent.com/FREEZONEX/Tier0-cli/main/install.ps1 | iex
```

Manual binaries are available from [GitHub Releases](https://github.com/FREEZONEX/Tier0-cli/releases).

## Authentication

Interactive browser flow:

```bash
tier0 login
```

Agent-friendly flow:

```bash
tier0 login --no-wait --json
tier0 login --setup-code <code>
```

Direct API key configuration:

```bash
tier0 config --api-key sk-per-xxxxxx
```

Private deployment:

```bash
tier0 config --base-url http://127.0.0.1:8088
tier0 login
```

Run `config --base-url` before `login`; otherwise the authorization URL may target the wrong instance.

## Common Commands

```bash
tier0 config
tier0 doctor
tier0 auth whoami --json
tier0 info

tier0 uns browse --path /
tier0 uns read Plant/Line1/Metric/Temperature --json
tier0 uns write --topic Plant/Line1/Metric/Temperature --value '{"temperature":27.5}'
tier0 uns history -t Plant/Line1/Metric/Temperature --start -1h --json

tier0 flow list
tier0 flow nodes --id 1 --json
tier0 flow create --name "modbus-collector" --source --desc "Modbus collector"
tier0 flow data --id 1 --out flows.json
tier0 flow deploy --id 1 -f flows.json --yes
```

`flow data --out` writes a deployable Node-RED `flows` array. `flow deploy -f`
also accepts older full API envelope files and extracts the `data.flows` array
automatically.

## Request Previews

Use `--dry-run` to validate and preview a write request without requiring an API
key, contacting Tier0, or requiring `--yes` for a high-risk operation:

```bash
tier0 uns write --topic demo --value '{"value":1}' --dry-run
tier0 flow deploy --id 1 --flows-file flows.json --dry-run --json
tier0 api /openapi/v1/uns/write --body-file body.json --dry-run --json
```

Request previews are supported by `api`, UNS `write/create/update/delete/restore`,
and Flow `create/update/delete/deploy`. JSON previews use one stable envelope:

```json
{
  "ok": true,
  "dry_run": true,
  "data": {
    "api": [
      {"method": "POST", "url": "https://tier0.dev/openapi/v1/uns/write", "body": {}}
    ]
  }
}
```

Headers and API keys are never included. Request body flags and files must
contain valid JSON; the CLI no longer guesses at or rewrites malformed JSON.

## Structured Errors

With `--json`, command validation failures are written to stderr with stable
`type`, `subtype`, and `param` fields:

```json
{"ok":false,"error":{"type":"validation","subtype":"invalid_argument","param":"--qos","message":"--qos must be 0, 1, or 2"}}
```

Automation should branch on these fields rather than matching message text.

## Flow Types

| Type | Meaning |
| --- | --- |
| `SourceFlow` | Connects industrial protocols, collects device data, and publishes MQTT / UNS data |
| `EventFlow` | Processes business data, alarms, transformations, and downstream actions |

## Configuration

Configuration is stored at `~/.tier0/config.json`.

Priority:

1. Command flags such as `--base-url`
2. Environment variables such as `TIER0_BASE_URL`
3. Config file
4. Default `https://tier0.dev`

Tier0 CLI output is English-only. The legacy `--lang en` flag is accepted for compatibility.

## Environment Variables

| Variable | Meaning |
| --- | --- |
| `TIER0_BASE_URL` | Override platform base URL |
| `TIER0_API_KEY` | Override API key |
| `TIER0_SKIP_UNINSTALL` | Skip npm uninstall cleanup hook |

## Uninstall

```bash
npx @tier0/cli@latest uninstall
npx @tier0/cli@latest uninstall --purge
npx @tier0/cli@latest uninstall --remove-skills
npx @tier0/cli@latest uninstall --purge --remove-skills
```

Agent Skills are kept by default so they can be updated or reused
independently. Add `--remove-skills` to remove the globally installed `tier0`
Skill from detected agents. Add `--purge` to also delete credentials.

Global npm uninstall:

```bash
npm uninstall -g @tier0/cli
```

Manual cleanup:

```bash
rm -rf ~/.tier0/bin/tier0 ~/.tier0/skills
npx -y --package=skills -- skills remove tier0 -y -g
```

## Development

Run tests:

```bash
go test ./...
```

Skill example validation scans the sibling `Tier0-skill/` repository, extracts `tier0 ...` examples, and validates command paths and flags against the Cobra command tree.

```bash
go test ./cmd -run TestSkillExamples
```

Refresh the compiled baseline from the latest `Tier0-skill/main` commit:

```bash
bash scripts/sync-embedded-skill.sh
bash scripts/sync-embedded-skill.sh --check
```

For local Skill development, opt in to a local checkout explicitly:

```bash
bash scripts/sync-embedded-skill.sh --local ../Tier0-skill
```

The sync records the full source commit in `_source.json`. Formal releases also
sync from GitHub by default and stop if that changes the checked-in snapshot;
review, commit, and push the generated snapshot before rerunning the release.

## Release

The npm package and Go binary versions are kept in sync by `scripts/release.sh`.

Build one local package without GitHub or npm credentials:

```bash
BUILD_ONLY=1 TARGET_PLATFORMS=windows/amd64 bash scripts/release.sh vX.Y.Z
```

Check the latest GitHub Skill snapshot without building or publishing:

```bash
PREFLIGHT_ONLY=1 bash scripts/release.sh vX.Y.Z
```

```bash
export GITHUB_TOKEN=ghp_xxxxxxxx
npm login
bash scripts/release.sh vX.Y.Z
```

For automation, set `NPM_TOKEN` instead of running `npm login`. Before creating the GitHub Release, the script verifies that the embedded Skill mirror is current, runs npm tests, validates the packed file list, and verifies npm authentication. The release process then cross-compiles self-contained binaries, uploads GitHub Release assets, updates `npm-wrapper/package.json`, and publishes npm.
