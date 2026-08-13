---
name: tier0-flow-delete
description: "Delete Tier0 Flows. High-risk operation: stops the related Node-RED container and requires --yes."
---

# flow delete

Use `flow delete` to delete one or more Flows.

## Commands

```bash
tier0 flow delete --id 1 --dry-run --json
tier0 flow delete --id 1 --id 2 --dry-run --json
# after user confirmation
tier0 flow delete --id 1 --yes
tier0 flow delete --id 1 --id 2 --yes
tier0 flow delete 1 2 --yes
```

## Rules

- Confirm the Flow exists with `flow get` before deleting.
- Tell the user deletion stops the related Node-RED container.
- Use `--yes` only after the user confirms.
- Prefer deleting one Flow at a time unless the user explicitly requests batch deletion.
- For a batch delete, preview the complete ID list; never preview fewer IDs than the confirmed execution.

## Recommended Flow

```bash
tier0 flow get --id 1 --json
tier0 flow delete --id 1 --dry-run --json
# tell the user that the Node-RED container will stop and wait for confirmation
tier0 flow delete --id 1 --yes
```

Dry-run does not require `--yes` and does not delete the Flow. Confirm that the
preview targets the intended integer Flow ID before asking the user.
