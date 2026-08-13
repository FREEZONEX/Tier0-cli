---
name: tier0-flow-list
description: "List and inspect Tier0 SourceFlow and EventFlow instances."
---

# flow list / flow get

Use these commands to find Flow IDs and inspect Flow metadata.

## Commands

```bash
tier0 flow list
tier0 flow list --source --json
tier0 flow list --event --json
tier0 flow list --keyword line1 --json
tier0 flow get --id 1 --json
```

## Rules

- Always list first when the ID is unknown.
- CLI commands use integer `id`.
- Do not use Node-RED `flowId` as a CLI identifier.
- If the user asks about a named device or data point, also check UNS if relevant.

## Typical Flow

```bash
tier0 flow list --keyword Line1 --json
tier0 flow get --id <id> --json
tier0 flow data --id <id> --out flows.json
```

## Fields

Common response fields:

| Field | Meaning |
| --- | --- |
| `id` | Integer Flow ID used by CLI commands |
| `flowId` | Node-RED internal tab or flow ID |
| `flowName` | Display name |
| `flowType` | `SourceFlow` or `EventFlow` |
| `flowStatus` | Runtime status |
| `description` | Metadata description |
| `isFavorite` | Favorite marker |
