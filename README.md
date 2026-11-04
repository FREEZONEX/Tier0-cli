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

With no saved base URL, explicitly setting a key binds it to the default
`https://tier0.dev`. If `TIER0_BASE_URL` is set, provide `--base-url` explicitly
with the key to select its origin. An already saved base URL takes precedence.

The browser login issues a Personal Key. An already issued Personal, Service,
or App Key can be configured explicitly; the server determines its identity
and effective permissions. `tier0 auth whoami` and `tier0 doctor` use the v2
identity endpoint, falling back to v1 only when v2 returns HTTP 404. `doctor`
does the same for the info endpoint. Ordinary API requests carrying a key
reject cross-origin redirects; file downloads can follow cross-origin HTTPS
redirects without forwarding credentials, subject to the limitations below. App-bound
operations may need additional server-verified authorization that this CLI
does not provide; configuring an App Key does not bypass those checks.

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

tier0 mqtt auth create --name agent --save agent --random-suffix=true
tier0 mqtt publish --credential agent --topic Plant/Line1/State/Status --file payload.json --json-message --qos 1
tier0 mqtt subscribe --credential agent --topic 'Plant/+/State/Status' --count 10 --timeout 60s --json
tier0 mqtt auth delete --credential agent --yes

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
Flow `create/update/delete/deploy`, MQTT credential create/delete, and MQTT
publish. API-backed JSON previews use this envelope:

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

MQTT publish previews use `data.mqtt` with the broker, topic, QoS, retain flag,
payload byte count, and credential profile name. MQTT usernames and passwords
are never included.

Headers and API keys are never included. Request body flags and files must
contain valid JSON; the CLI no longer guesses at or rewrites malformed JSON.

## Structured Errors

With `--json`, command validation failures are written to stderr with stable
`type`, `subtype`, and `param` fields:

```json
{"ok":false,"error":{"type":"validation","subtype":"invalid_argument","param":"--qos","message":"--qos must be 0, 1, or 2"}}
```

Automation should branch on these fields rather than matching message text.

## Browsing Cloud and Enterprise namespaces

`uns browse` first uses the Enterprise v1 browse contract. If that endpoint returns HTTP 404, it queries the Cloud v2 namespace endpoint. Cloud returns its native paginated `data.results` response, rather than Enterprise's `data.tree`; its IDs, kinds, and metadata have Cloud semantics. Cloud paths identify directories by exact name, including literal `.` and `..`, and the result lists their children. Use `--cloud --parent-id <id>` to list a known directory by its opaque Cloud ID; it cannot be combined with `--path`. Path resolution uses Cloud substring search and checks every matching page for an exact, unique directory name. It starts at 200 entries per page and restarts with smaller pages on Cloud `RESPONSE_TOO_LARGE`. The default limit is 1000 pages per segment per attempt; `--path-max-pages` with `--cloud` can set 1–10000. It fails explicitly when the limit prevents a complete uniqueness check. Cloud pagination is per request and is not an atomic snapshot across pages. `--cloud` selects v2 directly; `--page` and `--size` require `--cloud` (size 1–200). Cloud does not support the v1 `--max-depth` values other than 1, `--include-metadata`, or `--include-leaf-value` through this command; requests using them fail explicitly. Cloud App Keys can be denied by the server's namespace policy. Other UNS commands may still target v1 and require a compatible server.

## Cloud files and stream programs

`assets url --cloud --file-path <path>` uses the Cloud v2 file URL contract. Without `--cloud` the command uses Enterprise v1; neither route is automatically retried against the other. Only request URLs for files already owned by the Workspace, and treat signed URLs as secrets.

`flow list --cloud` lists Cloud stream programs and returns the native paginated Cloud response. `--page` and `--size` select a page (size 1–200), and `--keyword` maps to the Cloud search field; Enterprise `--source`, `--event`, and `--type` are rejected in this mode because SourceFlow/EventFlow are not Cloud Stream types. Other `flow` commands still implement Enterprise Node-RED contracts; use the documented Cloud Source/Stream APIs through `tier0 api` instead of treating them as equivalent or retrying a write under another API.

## Flow Types

| Type | Meaning |
| --- | --- |
| `SourceFlow` | Connects industrial protocols, collects device data, and publishes MQTT / UNS data |
| `EventFlow` | Processes business data, alarms, transformations, and downstream actions |

## Configuration

Configuration is stored at `~/.tier0/config.json`. MQTT credentials saved with
`mqtt auth create --save` use individual user-only files under `~/.tier0/mqtt/`.

OpenAPI commands, `auth whoami`, and `assets download` use the API key and base
URL saved in the configuration file. `TIER0_API_KEY` is not supported: setting
it does not override the saved key or authenticate a request. Configure a key
with `tier0 login` or `tier0 config --api-key` instead.
Treat command arguments, shell history, and logs as potentially visible when
passing a key directly. The existing completed `login --json` response includes
the issued Personal Key and must be handled as secret output.

Changing `config --base-url` to a different HTTP origin (scheme, host, or port)
clears the saved API key. Supply `--api-key` in the same command to bind a new
key explicitly; changes within the same origin retain the key.

`assets download` uses the v2 endpoint, falling back to v1 only when the gateway
returns HTTP 404 before a redirect. It follows public or presigned cross-origin
storage redirects only over HTTPS, without forwarding request headers or the API
key. HTTPS-to-HTTP downgrades and more than five redirects are rejected. Treat
presigned URLs as secrets. The gateway must return a 302 redirect. The current
gateway contract supplies no proof that a redirect or its response is actually
object storage: an intermediary or SSO service can return a login page as a
successful download. The CLI cannot distinguish that response from an HTML file.

Downloads written with `-o` use a temporary file in the destination directory.
The final destination must be absent or a regular file, never a symbolic link or
special file. On Unix, replacement preserves ordinary rwx mode bits, but not
ownership, ACLs, extended attributes, special bits, or other hard links. On
Windows, existing destinations are refused to avoid silently replacing their
ACLs; choose a new destination. Creating a new destination requires filesystem
support for hard links. Concurrent changes to a destination are not supported.
Only completed transfers are installed; stdout downloads are streamed directly.

Base URL resolution differs by command. `login` uses its `--base-url` flag,
then `TIER0_BASE_URL`, the saved base URL, and finally `https://tier0.dev`.
Dry-run previews resolve `TIER0_BASE_URL` before the saved URL, while actual
OpenAPI requests use the saved URL. Do not use a dry-run URL as evidence of an
actual request target when they differ. When a key is saved, `doctor` uses its
saved base URL and refuses cross-origin redirects; a key without a saved base
URL cannot be used for diagnostics. Without a saved key, `doctor` can use
`TIER0_BASE_URL` for an unauthenticated connectivity check.

Tier0 CLI output is English-only. The legacy `--lang en` flag is accepted for compatibility.

## Environment Variables

| Variable | Meaning |
| --- | --- |
| `TIER0_BASE_URL` | Override the base URL for login, dry-run previews, MQTT profile selection, and doctor connectivity checks without a saved key; not for authenticated doctor or ordinary OpenAPI requests |
| `TIER0_MQTT_BROKER` | MQTT broker used without a saved credential profile |
| `TIER0_MQTT_CLIENT_ID` | MQTT client ID used without a saved credential profile |
| `TIER0_MQTT_USERNAME` | MQTT username used without a saved credential profile |
| `TIER0_MQTT_PASSWORD` | MQTT password used without a saved credential profile |
| `TIER0_MQTT_RANDOM_SUFFIX` | Add a random client-ID suffix when set to `true` or `1` |
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
go test ./cmd -run '^TestSkillExampleCommands$'
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
