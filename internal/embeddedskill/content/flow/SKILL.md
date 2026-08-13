---
name: tier0-flow
description: "Tier0 Flow management for Node-RED: list, create, update, delete, export, and deploy SourceFlow and EventFlow canvas JSON."
---

# tier0-flow

Use this skill for Tier0 Node-RED Flow management.

## Use When

- The user wants to list, inspect, create, update, or delete Flows.
- The user wants to export Node-RED canvas JSON.
- The user wants to deploy or replace a Node-RED canvas.
- The user asks about SourceFlow or EventFlow state.

## Do Not Use When

- The user wants current UNS topic values. Use `uns/references/read.md`.
- The user wants to directly drag nodes in the Node-RED UI. The CLI imports and exports JSON; UI editing happens in a browser.
- The user wants protocol mapping details. Read the matching protocol reference first.

## Non-Negotiable Rules

1. List before acting when the Flow ID is unknown.
2. CLI commands use integer `id`; Node-RED `flowId` is not a CLI identifier.
3. Before deploy, export a backup: `tier0 flow data --id <id> --out backup.json`.
4. Preview agent-generated mutations with `--dry-run --json`; this is required before deploy and delete.
5. `flow deploy` and `flow delete` require `--yes` after user confirmation.
6. Deleting a Flow stops the related Node-RED container.
7. Do not construct deploy payloads before reading `references/deploy.md`.
8. Preserve the backend-created Tier0 `mqtt-broker` config node. Do not create or replace it.

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

## 可调用接口速查（curl）

> AI 直接按此表写 curl 即可。host = `TIER0_API_HOST`，鉴权 `Authorization: Bearer <TIER0_API_KEY>`。

### A. 平台管理接口（`POST /openapi/v1/flow/*`，CLI 已封装，亦可 curl）

| Path | 请求体 | 返回 |
|---|---|---|
| `POST /openapi/v1/flow/list` | `{"keyword":"","flowType":"source"\|"event"}`（flowType 不传返回全部） | `{code,msg,data:{list:[]}}` |
| `POST /openapi/v1/flow/get` | `{"id":1}` | `{code,msg,data:{...FlowInfo}}` |
| `POST /openapi/v1/flow/create` | `{"flowName":"x","flowType":"source"\|"event","description":"","template":""}` | `{code,msg,data:{id}}` |
| `POST /openapi/v1/flow/update` | `{"id":1,"flowName":"","description":"","template":"","isFavorite":0}` | `{code,msg,data:{success}}` |
| `POST /openapi/v1/flow/delete` | `{"ids":[1]}` | `{code,msg,data:{success}}` |
| `POST /openapi/v1/flow/flowdata` | `{"id":1}` | `{code,msg,data:{rev,flows}}` |
| `POST /openapi/v1/flow/nodes` | `{"flowType":"source"\|"event"}` | `{code,msg,data:{nodes:[]}}` |
| `POST /openapi/v1/flow/deploy` | `{"id":1,"flowsJson":"[...]"}` | `{code,msg,data:{flowId}}` |

```bash
curl -H "Authorization: Bearer $TIER0_API_KEY" -H "Content-Type: application/json" \
  -d '{"flowType":"event"}' "http://$TIER0_API_HOST/openapi/v1/flow/list"
```

### B. Node-RED 原生接口（`/flow/{source|event}/**`，高级/运行时直读）

| Method | Path | 说明 |
|---|---|---|
| GET | `/flow/{source\|event}/flows` | 原生 flows（含运行时 rev，非平台合成） |
| GET | `/flow/{source\|event}/flow/:id` | 单个 flow 节点与 configs |
| GET | `/flow/{source\|event}/nodes` | 已安装节点类型列表 |
| GET | `/flow/{source\|event}/settings`、`/status` | 运行时设置 / 状态 |
| POST | `/flow/{source\|event}/flows` | 全量部署 |
| POST | `/flow/{source\|event}/flow` | 创建 flow |
| PUT | `/flow/{source\|event}/flow/:id` | 更新 flow |
| DELETE | `/flow/{source\|event}/flow/:id` | 删除 flow |

```bash
curl -H "Authorization: Bearer $TIER0_API_KEY" \
  "http://$TIER0_API_HOST/flow/source/flows"      # 读 sourceflow 原生 flows
curl -H "Authorization: Bearer $TIER0_API_KEY" \
  "http://$TIER0_API_HOST/flow/event/nodes"       # 读 eventflow 节点类型
```

- 鉴权：`Authorization: Bearer <TIER0_API_KEY>`（或 `X-API-Key`）；full_access 读写全开，read_only 只读；
- 与 `tier0 flow data` 的关系：备份/导出仍用 CLI（平台侧数据），原生接口是运行时直读；
- 风险：写操作（部署/更新/删除）绕过平台校验与版本快照，改动前先 `tier0 flow data --out backup.json`；
- 这些路径非 CLI 内置命令，仅限高级/自动化场景，默认仍用 `tier0 flow ...`。
