---
name: tier0
description: "Tier0 platform operations entry point: CLI setup, authentication, routing to UNS, MQTT, Flow, managed files, project and platform member queries, service info, and API key diagnostics skills."
metadata:
  requires:
    bins: ["tier0"]
  cliHelp: "tier0 --help"
---

# Tier0

This is the shared entry point for Tier0 CLI work. Keep this file focused on setup, authentication, and routing. Task-specific behavior belongs in the matching sub skill or reference file.

## Use When

Use this skill when the user asks about:

- installing or configuring Tier0 CLI
- authenticating with Tier0
- choosing the right Tier0 skill or command family
- UNS, MQTT, Flow, Node-RED, service connectivity, or API key permissions
- querying project or platform members, roles, user statuses, or role-bound applications

## Setup

Recommended install:

```bash
npx -y @tier0/cli@latest install
```

The explicit `install` command requires the bundled Tier0 Skill to install
successfully. A CLI-only install is incomplete for agent use.

Alternative install:

```bash
curl -fsSL https://raw.githubusercontent.com/FREEZONEX/Tier0-cli/main/install.sh | bash
```

If using the alternative installer, install skills separately:

```bash
npx skills add FREEZONEX/Tier0-skill
```

## Configuration

SaaS uses `https://tier0.dev` by default.

Private deployments must be configured before login:

```bash
tier0 config --base-url <tier0-base-url>
```

If `base-url` changes, run login again or set an API key for that instance. Existing keys are tied to the previous instance and Workspace.

## Authentication

After installation, always check authentication before the first platform
operation:

```bash
tier0 auth whoami --json
```

If it succeeds, reuse the existing login and do not prompt again. If it returns
an authentication/config error, start the browser device flow below.

Preferred when an API key is already available:

```bash
tier0 config --api-key <api-key>
```

Browser device flow:

```bash
tier0 login --no-wait --json
```

Show the returned `verification_url` to the user. Then poll with:

```bash
tier0 login --setup-code <setup_code>
```

The polling command blocks until browser authorization completes or times out.
Never hide the `verification_url` or ask the user to copy an API key from the
browser page.

## Command Execution Contract

For agent-generated mutation commands, validate the final request with
`--dry-run --json` before executing it. This applies to:

- `tier0 api`
- `tier0 uns write/create/update/delete/restore`
- `tier0 flow create/update/delete/deploy`
- `tier0 mqtt auth create/delete` and `tier0 mqtt publish`

A preview does not require or expose an API key, contact Tier0, or perform the operation. It
returns a stable envelope. API-backed mutations use `data.api`:

```json
{
  "ok": true,
  "dry_run": true,
  "data": {
    "api": [
      {"method": "POST", "url": "https://tier0.dev/openapi/v1/...", "body": {}}
    ]
  }
}
```

MQTT publish previews use `data.mqtt` and never include the MQTT username or
password. Read `mqtt/SKILL.md` before creating credentials or opening a stream.

Inspect `method`, `url`, and `body`. Never report a dry-run as a successful
business operation. Execute the same command without `--dry-run` only after the
preview matches the user's intent. For high-risk delete, restore, and deploy
operations, preview first, show the impact to the user, wait for confirmation,
then add `--yes`.

With `--json`, failures are written to stderr. Branch on
`error.type`, `error.subtype`, and `error.param`, not message text. Exit code 10
with `confirmation_required` still means user confirmation is required.

JSON is strict. The CLI does not repair malformed input. Prefer `--file`,
`--body-file`, `--template-file`, or `--flows-file` for complex payloads, and do
not pass both an inline JSON flag and its file alternative.

## Routing

Read the target sub skill or reference before executing a task-specific command.

| User goal | Read |
| --- | --- |
| Browse, search, read, write, history, create, update, delete, or restore UNS nodes/topics | `uns/SKILL.md` |
| Create/delete MQTT credentials, publish messages, or subscribe continuously | `mqtt/SKILL.md` |
| List, inspect, create, update, delete, export, or deploy Node-RED Flows | `flow/SKILL.md` |
| List/filter project members on a deployment where the endpoint is enabled | `launchpad/members.md` |
| List/filter platform members on a deployment where the endpoint is enabled | `platform/members.md` |
| Upload, download, get URL, or delete files in object storage | `files/SKILL.md` |
| Check service connectivity and gateway info | `info/info.md` |
| Check API key identity and permissions | `auth/whoami.md` |

## Core Concepts

| Concept | Meaning |
| --- | --- |
| Workspace | Tenant boundary for all resources |
| Project | Workspace-scoped application context identified by name in Launchpad APIs |
| UNS | Unified Namespace, a tree of paths and data topics |
| Path | Folder-like namespace segment |
| Topic | Full leaf path, such as `Plant/Line1/Metric/Temperature` |
| Flow | Node-RED instance managed by Tier0 |

Flow names and UNS paths are often manually kept similar, but current APIs do not expose a guaranteed relation field. When the user asks about a named device or data point, check both UNS and Flow unless they explicitly ask for only one side.

For continuous or event-driven UNS data, do not repeatedly poll `uns read` or
the OpenAPI. Read `uns/references/realtime.md`; Tier0 CLI read commands are for
snapshots and bounded queries.

## Shell Notes

Quote wildcard topics in shells that expand `*`, `+`, or `#`:

```bash
tier0 mqtt subscribe --credential agent --topic 'Plant/+/Metric/Temperature' --count 1 --json
```

For complex JSON, prefer files or stdin over fragile shell quoting.

## Updates

The CLI checks for updates in the background at most once every 24 hours. Upgrade manually with:

```bash
tier0 upgrade
tier0 skills update
```
