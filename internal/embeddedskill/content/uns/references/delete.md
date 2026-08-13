---
name: tier0-uns-delete
description: "Delete Tier0 UNS nodes. Supports soft delete and hard delete; hard delete is irreversible."
---

# uns delete

Use `delete` to remove UNS nodes.

## Command

```bash
tier0 uns delete --path Plant/Line1/Metric/Temperature --dry-run --json
tier0 uns delete --path Plant/Line1/Metric/Temperature --hard --dry-run --json
# after user confirmation
tier0 uns delete --path Plant/Line1/Metric/Temperature --yes
tier0 uns delete --path Plant/Line1/Metric/Temperature --hard --yes
```

## Rules

- Use `--path`.
- `--topic` is a deprecated compatibility alias; prefer `--path`.
- Confirm destructive impact with the user before adding `--yes`.
- `--hard` is irreversible.
- Use `restore.md` only for soft-deleted nodes.

## Recommended Flow

```bash
tier0 uns browse --path Plant/Line1/Metric --include-metadata --json
tier0 uns delete --path Plant/Line1/Metric/Temperature --dry-run --json
# show whether this is a soft or hard delete and wait for user confirmation
tier0 uns delete --path Plant/Line1/Metric/Temperature --yes
```

Dry-run does not require `--yes` and does not delete the node. For `--hard`,
make the irreversible flag visible in both the preview and confirmation.
