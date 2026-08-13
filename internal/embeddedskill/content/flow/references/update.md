---
name: tier0-flow-update
description: "Update Tier0 Flow metadata such as name, description, template, and favorite state."
---

# flow update

Use `flow update` to change Flow metadata. It does not deploy Node-RED canvas JSON.

## Commands

```bash
tier0 flow update --id 1 --name "line1-collector"
tier0 flow update --id 1 --desc "Line 1 Modbus collector"
tier0 flow update --id 1 --favorite
tier0 flow update --id 1 --unfavorite
```

## Preflight

Preview the final metadata change before executing it:

```bash
tier0 flow update --id 1 --desc "Line 1 Modbus collector" --dry-run --json
```

On PowerShell, use `--desc=` when an empty string must remain in the preview;
`--desc ""` may be removed by the shell. The public SaaS update service
currently ignores an empty Flow description, so do not claim that this clears
the description. Verify the returned Flow after every update.

## Rules

- Use integer Flow `id`, not Node-RED `flowId`.
- Do not use the template flags as a substitute for canvas deployment. The
  public SaaS template schema is not documented; use `flow data` and
  `flow deploy` for Node-RED canvas JSON.
- `--favorite` and `--unfavorite` are mutually exclusive.
- Provide at least one field to update.
- Use `flow deploy` for canvas JSON.
- Use `flow data` before changing deployable content.
- Re-run `flow get --id <id> --json` and verify the changed fields.
