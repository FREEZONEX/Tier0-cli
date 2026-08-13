---
name: tier0-uns
description: "Tier0 UNS operations: browse, read, write, history, search, create, update, delete, and restore namespace nodes and topics."
---

# tier0-uns

Use this skill for Tier0 Unified Namespace data-plane and namespace-management tasks.

## Use When

- The user asks to browse namespace paths or discover topics.
- The user asks to read current topic values.
- The user asks to consume UNS values continuously or in real time.
- The user asks to write values into UNS topics.
- The user asks for historical values or aggregates.
- The user asks to create, update, delete, or restore namespace nodes.

## Do Not Use When

- The user wants to manage Node-RED SourceFlow/EventFlow containers. Use `flow/SKILL.md`.
- The user wants API key diagnostics. Use `auth/whoami.md`.
- The user wants service connectivity only. Use `info/info.md`.

## Non-Negotiable Rules

1. `read`, `write`, and `history` require full topic leaf paths.
2. Middle folders cannot be read or written; use `browse`.
3. Topic paths must include `Metric`, `Action`, or `State` immediately before the leaf.
4. `topicType` is derived from that folder; do not rely on `--topic-type` to create the folder.
5. Write payload `value` must be an object matching the topic fields. The CLI rejects scalar and array values.
6. For batch responses, check `data.success` and every `data.results[i].success`.
7. Use `--path` for delete. `--topic` is a deprecated alias only for compatibility.
8. `Metric` topics require `--fields` at creation. Declare `--fields` on `Action`/`State` topics too — without them the topic has no visible schema in UNS.
9. Preview agent-generated `write`, `create`, `update`, `delete`, and `restore` requests with `--dry-run --json`.
10. JSON is strict. Use file flags for complex payloads and never combine an inline JSON flag with its file alternative.
11. `uns read` is a snapshot API. For continuous/event-driven data, do not build an OpenAPI polling loop; read `references/realtime.md` and use MQTT/EventFlow when the required runtime and credentials are available.

## Routing

| Intent | Read |
| --- | --- |
| Browse namespace folders | `references/browse.md` |
| Read current values | `references/read.md` |
| Consume values continuously or in real time | `references/realtime.md` |
| Write current values | `references/write.md` |
| Query history or aggregates | `references/history.md` |
| Search topics by keyword or path prefix | `references/search.md` |
| Create nodes or import a tree | `references/create.md` |
| Update metadata or fields | `references/update.md` |
| Delete nodes | `references/delete.md` |
| Restore soft-deleted nodes | `references/restore.md` |

## Topic Path Rules

Valid examples:

```text
Plant/Line1/Metric/Temperature
Plant/Line1/State/MachineStatus
Machine/Action/Start
```

Invalid:

```text
Plant/Line1/Temperature
```

The invalid path is missing the type folder before the leaf.

## Common Commands

```bash
tier0 uns browse --path /
tier0 uns search --keyword Temperature --json
tier0 uns read Plant/Line1/Metric/Temperature --json
tier0 uns write --topic Plant/Line1/Metric/Temperature --value '{"temperature":27.5}'
tier0 uns history -t Plant/Line1/Metric/Temperature --start -1h --json
tier0 uns create --topic Plant/Line1/Metric/Temperature --type topic --fields '[{"name":"temperature","type":"float"}]'
tier0 uns delete --path Plant/Line1/Metric/Temperature --hard --dry-run --json
# after user confirmation
tier0 uns delete --path Plant/Line1/Metric/Temperature --hard --yes
```

## Mutation Preflight

Append `--dry-run --json` to the final mutation command and inspect
`data.api[0].method`, `url`, and `body`. A preview does not authenticate, send a
request, or prove business success. Remove `--dry-run` only after the preview
matches the user's intent.

For delete and restore, show the preview and destructive impact to the user,
wait for confirmation, then execute with `--yes`.

## Structured Errors

With `--json`, read failures from stderr and branch on `error.type`,
`error.subtype`, and `error.param`. Fix `invalid_argument` at the named parameter
instead of retrying unchanged. Exit code 10 with `confirmation_required` means
the user must approve delete or restore before `--yes` is added.

## Batch Response Handling

UNS batch APIs can return HTTP 200 while one or more business items fail.

Required checks:

```js
if (resp.data?.success === false) {
  throw new Error("UNS batch operation failed");
}
for (const result of resp.data?.results ?? []) {
  if (result.success === false) {
    throw new Error(result.message || result.topic || result.path || "UNS item failed");
  }
}
```
