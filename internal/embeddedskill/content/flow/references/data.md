---
name: tier0-flow-data
description: "Export Node-RED canvas JSON for a Tier0 Flow. Required before deploy as a backup and to preserve config nodes."
---

# flow data

Use `flow data` to export Node-RED canvas JSON.

## Commands

```bash
tier0 flow data --id 1 --json
tier0 flow data --id 1 --out flows.json
```

## Rules

- Use integer Flow `id`, not Node-RED `flowId`.
- Always run this before `flow deploy`.
- Save a backup before modifying any existing canvas.
- Preserve config nodes from the exported JSON, especially Tier0 `mqtt-broker`.
- `--json` prints the full API response.
- `--out` writes the deployable Node-RED `flows` array so it can be edited and passed directly to `flow deploy -f`.

## Recommended Workflow

```bash
tier0 flow list --json
tier0 flow data --id <id> --out backup.json
cp backup.json flows.json
# edit flows.json
tier0 flow deploy --id <id> -f flows.json --dry-run --json
# show the replacement impact and wait for user confirmation
tier0 flow deploy --id <id> -f flows.json --yes
```

## Notes

Node-RED may not export plaintext credentials. Credentials are stored internally by config node ID. If you replace a config node, the stored credentials may no longer match.
