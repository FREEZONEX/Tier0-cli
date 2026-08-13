---
name: tier0-flow
description: "Tier0 Flow and Node-RED HTTP ingress operations: list, create, update, delete, export, and deploy SourceFlow/EventFlow canvas JSON, or expose and call user-defined http in endpoints through /flow/source or /flow/event. Do not use for Node-RED Admin API access."
---

# tier0-flow

Use this skill for Tier0 Node-RED Flow management.

## Use When

- The user wants to list, inspect, create, update, or delete Flows.
- The user wants to export Node-RED canvas JSON.
- The user wants to deploy or replace a Node-RED canvas.
- The user asks about SourceFlow or EventFlow state.
- The user wants to expose or call a webhook or application endpoint implemented with Node-RED `http in` and `http response` nodes.

## Do Not Use When

- The user wants current UNS topic values. Use `uns/references/read.md`.
- The user wants to directly drag nodes in the Node-RED UI. The CLI imports and exports JSON; UI editing happens in a browser.
- The user wants protocol mapping details. Read the matching protocol reference first.
- The user wants Node-RED Admin API access. The gateway blocks Admin paths; use the CLI Flow management commands instead.

## Non-Negotiable Rules

1. List before acting when the Flow ID is unknown.
2. CLI commands use integer `id`; Node-RED `flowId` is not a CLI identifier.
3. Before deploy, export a backup: `tier0 flow data --id <id> --out backup.json`.
4. Preview agent-generated mutations with `--dry-run --json`; this is required before deploy and delete.
5. `flow deploy` and `flow delete` require `--yes` after user confirmation.
6. Deleting a Flow stops the related Node-RED container.
7. Do not construct deploy payloads before reading `references/deploy.md`.
8. Preserve the backend-created Tier0 `mqtt-broker` config node. Do not create or replace it.
9. Use `/flow/{source|event}/**` only for user-defined `http in` endpoints. Never use it for Flow management or Node-RED Admin APIs.
10. Call `http in` endpoints with `tier0 api`; do not add or assume a separate `flow invoke` command.

## Flow Types

| Type | Meaning | Typical Use |
| --- | --- | --- |
| `SourceFlow` | Collects industrial protocol data and publishes MQTT / UNS | Modbus, OPC-UA, OPC-DA, MQTT bridge |
| `EventFlow` | Processes business data or subscribes to MQTT | Alarms, transformations, actions, archival |

## Routing

| Intent | Read | Risk |
| --- | --- | --- |
| Query available node types | `references/nodes.md` | Low |
| List or inspect Flow | `references/list.md` | Low |
| Create Flow | `references/create.md` | Low |
| Update Flow metadata | `references/update.md` | Low |
| Delete Flow | `references/delete.md` | High, requires `--yes` |
| Export Node-RED canvas | `references/data.md` | Low |
| Deploy Node-RED canvas | `references/deploy.md` | High, requires backup and `--yes` |
| Expose or call an `http in` endpoint or webhook | `references/http-endpoints.md` | Depends on the endpoint's business effect |

## Protocol References

Read the matching file before generating or editing Flow JSON:

| Intent | Read |
| --- | --- |
| Modbus TCP/RTU to UNS | `references/protocols/modbus.md` |
| OPC-UA subscription to UNS | `references/protocols/opcua.md` |
| OPC-DA polling to UNS | `references/protocols/opcda.md` |
| External MQTT broker to UNS | `references/protocols/mqtt-bridge.md` |
| UNS to PostgreSQL archive | `references/protocols/postgresql.md` |
| Template index | `references/protocols/README.md` |

## Common Commands

```bash
tier0 flow list
tier0 flow list --source --json
tier0 flow get --id 1 --json
tier0 flow create --name "modbus-collector" --source --desc "Modbus TCP collector"
tier0 flow create --name "alert-handler" --event --desc "Temperature alarm processor"
tier0 flow data --id 1 --out flows.json
tier0 flow deploy --id 1 -f flows.json --dry-run --json
# after user confirmation
tier0 flow deploy --id 1 -f flows.json --yes
```

## Node-RED Canvas Workflow

```bash
tier0 flow list
tier0 flow data --id <id> --out backup.json
# edit or generate flows.json
tier0 flow deploy --id <id> -f flows.json --dry-run --json
# show the preview and impact; wait for user confirmation
tier0 flow deploy --id <id> -f flows.json --yes
```

When generating `flows.json`, include the existing backend-created Tier0 `mqtt-broker` config node from the exported data. Node-RED credentials are stored against the node ID; replacing that node can cause anonymous MQTT connections and authentication failure.

`tier0 flow data --out flows.json` writes a deployable Node-RED `flows` array.
The file can be edited, previewed with `tier0 flow deploy -f flows.json --dry-run --json`,
and deployed with `--yes` after user confirmation.

## Structured Errors

With `--json`, read failures from stderr and branch on `error.type`,
`error.subtype`, and `error.param`. Fix `invalid_argument` at the named parameter
instead of retrying unchanged. Exit code 10 with `confirmation_required` means
the user must approve the high-risk operation before `--yes` is added.
