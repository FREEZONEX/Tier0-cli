---
name: tier0-uns-create
description: "Create Tier0 UNS namespace folders and topics. Topic leaf paths must include Metric, Action, or State as the segment before the leaf."
---

# uns create

Use `create` to create namespace folders, single topics, or a batch namespace tree.

## Critical Path Rule

For topic nodes, the segment immediately before the leaf must be one of:

- `Metric`
- `Action`
- `State`

The backend derives `topicType` from that segment.

Valid:

```text
Plant/Line1/Metric/Temperature
Plant/Line1/State/MachineStatus
Machine/Action/Start
```

Invalid:

```text
Plant/Line1/Temperature
```

The CLI and backend do not automatically insert the type folder.

## Preflight

Preview the complete namespace request before creating nodes:

```bash
tier0 uns create --topic Plant/Line1/Metric/Temperature --type topic --fields '[{"name":"temperature","type":"float"}]' --dry-run --json
tier0 uns create --topic Plant/Line1/Metric/Temperature --type topic --fields-file fields.json --dry-run --json
tier0 uns create --file namespace.json --dry-run --json
```

Verify the generated namespace tree in the request body, then execute the same
command without `--dry-run`.

## Single Folder

```bash
tier0 uns create --topic Plant/Line1 --type path
```

## Fields (Schema) Rule

- `Metric` topics: `--fields` is **required**. Missing fields fails with `schema required for metric`.
- Use `--fields-file` instead of inline JSON when shell quoting is fragile.
- `Action`/`State` topics: `--fields` is optional but **strongly recommended**. Without it the topic has no visible schema in UNS and consumers cannot discover the payload contract. Declare the flat payload keys as fields; cover nested structures with an example payload in `--description`.

## Single Topic (Metric)

```bash
tier0 uns create \
  --topic Plant/Line1/Metric/Temperature \
  --type topic \
  --display-name "Temperature" \
  --description "Line 1 temperature" \
  --fields '[{"name":"temperature","type":"float"},{"name":"unit","type":"string"}]'
```

## Action / State Topic

Declare fields for the payload keys, plus an example payload in the description:

```bash
tier0 uns create \
  --topic Plant/Line1/Action/StartBatch \
  --type topic \
  --description 'Start batch command. Example: {"batch_id":"B-001","recipe":"dark_choco","qty":500}' \
  --fields '[{"name":"batch_id","type":"string"},{"name":"recipe","type":"string"},{"name":"qty","type":"int"}]'
```

```bash
tier0 uns create \
  --topic Plant/Line1/State/BatchStatus \
  --type topic \
  --description 'Batch status report. Example: {"batch_id":"B-001","status":"running","progress":42}' \
  --fields '[{"name":"batch_id","type":"string"},{"name":"status","type":"string"},{"name":"progress","type":"int"}]'
```

## Parent Prefix

```bash
tier0 uns create \
  --parent Plant/Line1/Metric \
  --topic Temperature \
  --type topic \
  --fields '[{"name":"temperature","type":"float"}]'
```

## Batch File

For trees or multiple nodes, use `--file`.
Do not combine `--file` with inline node flags such as `--topic`, `--type`,
`--parent`, `--fields`, or metadata flags.

```bash
tier0 uns create --file namespace.json --json
```

Preferred file shape:

Every namespace node uses `name` for its local segment. Do not use `path`;
`path` is a command flag for addressing existing nodes, not a batch-node field.

```json
{
  "namespace": [
    {
      "name": "Plant",
      "type": "PATH",
      "children": [
        {
          "name": "Line1",
          "type": "PATH",
          "children": [
            {
              "name": "Metric",
              "type": "PATH",
              "children": [
                {
                  "name": "Temperature",
                  "type": "TOPIC",
                  "fields": [
                    { "name": "temperature", "type": "float" }
                  ]
                }
              ]
            },
            {
              "name": "Action",
              "type": "PATH",
              "children": [
                {
                  "name": "StartBatch",
                  "type": "TOPIC",
                  "description": "Start batch command. Example: {\"batch_id\":\"B-001\",\"qty\":500}",
                  "fields": [
                    { "name": "batch_id", "type": "string" },
                    { "name": "qty", "type": "int" }
                  ]
                }
              ]
            },
            {
              "name": "State",
              "type": "PATH",
              "children": [
                {
                  "name": "BatchStatus",
                  "type": "TOPIC",
                  "description": "Batch status report. Example: {\"batch_id\":\"B-001\",\"status\":\"running\"}",
                  "fields": [
                    { "name": "batch_id", "type": "string" },
                    { "name": "status", "type": "string" }
                  ]
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}
```

## Batch Response Handling

`uns create --file` uses a batch API. HTTP 200 and outer `code: 200` do not prove all nodes were created.

Required checks:

```js
if (resp.data?.success === false) {
  throw new Error("UNS create batch failed");
}
for (const result of resp.data?.results ?? []) {
  if (result.success === false) {
    throw new Error(result.message || result.path || "UNS create item failed");
  }
}
```

## Field Types

Common field types:

| Type | Use |
| --- | --- |
| `float` | Numeric process values |
| `int` | Counts, codes, discrete numeric states |
| `string` | Text state, mode, operator input |
| `bool` | Boolean state |

## Modeling Guidance

If the product requires current readable values for every state or alarm, create those values as topics that the backend caches. Current backend behavior may vary by topic type; verify with `uns read` after creating and writing the topic.

## PowerShell

Prefer `--fields-file` for a single topic or `--file` for a namespace tree.
Do not rely on inline JSON quoting for arrays:

```powershell
tier0 uns create --topic Plant/Line1/Metric/Temperature --type topic --fields-file fields.json --dry-run --json
tier0 uns create --file namespace.json --json
```
