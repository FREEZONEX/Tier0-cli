---
name: tier0-flow-deploy
description: "Deploy Node-RED canvas JSON to a Tier0 Flow. High-risk operation: backs up first and requires --yes."
---

# flow deploy

Use `flow deploy` to replace the Node-RED canvas JSON for a Flow.

This is high risk because it replaces all nodes in that Node-RED instance.

## Required Workflow

```bash
tier0 flow list --json
tier0 flow data --id <id> --out backup.json
# create or edit flows.json
tier0 flow deploy --id <id> -f flows.json --dry-run --json
# show the preview and replacement impact; wait for user confirmation
tier0 flow deploy --id <id> -f flows.json --yes
```

## Commands

```bash
tier0 flow deploy --id 1 -f flows.json --dry-run --json
tier0 flow deploy --id 1 --flows-json '<json>' --dry-run --json
# after user confirmation
tier0 flow deploy --id 1 -f flows.json --yes
tier0 flow deploy --id 1 --flows-json '<json>' --yes
```

## Rules

- Always back up with `flow data` before deploy.
- Always preview the final deploy file with `--dry-run --json` before asking for confirmation.
- Use integer Flow `id`, not Node-RED `flowId`.
- Use either `--flows-json` or `--flows-file`, not both.
- `--yes` is required after user confirmation.
- Preserve backend-created config nodes from the exported canvas.
- Node-RED may remap tab IDs and return a new internal `flowId`; that is normal.
- `flow deploy -f` accepts the deployable `flows` array exported by `flow data --out`.
- For compatibility, `flow deploy` also accepts older full API envelopes and `data` objects, and extracts `flows` before sending the deployment.

## Tier0 MQTT Broker Config

Backend-created Flows include a Tier0 `mqtt-broker` config node with credentials stored by Node-RED.

When building `flows.json`:

- Copy the existing Tier0 `mqtt-broker` config node from `backup.json`.
- Keep the same node `id`.
- Keep `broker`, `port`, and `clientid` from the export.
- Do not add plaintext credentials.
- Do not replace it with a newly generated config node.

If this rule is violated, MQTT output can connect anonymously and be rejected by EMQX.

## Exit Code 10

Without `--yes`, the CLI exits with code 10 and returns `confirmation_required`. Show the risk summary to the user; retry with `--yes` only after agreement.

`--dry-run` is the pre-confirmation path: it returns the request preview without
requiring `--yes` or replacing any Node-RED nodes.
