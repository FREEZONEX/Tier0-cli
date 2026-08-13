---
name: tier0-uns-update
description: "Update Tier0 UNS node metadata and topic field definitions. This does not write current values."
---

# uns update

Use `update` to change metadata or field definitions for an existing UNS node.

Do not use this command to write live values. Use `write.md` for data.

## Command

```bash
tier0 uns update \
  --path Plant/Line1/Metric/Temperature \
  --display-name "Temperature" \
  --description "Line 1 temperature" \
  --alias "line1_temp" \
  --update-mask displayName,description,alias
```

Update fields:

```bash
tier0 uns update \
  --path Plant/Line1/Metric/Temperature \
  --fields '[{"name":"temperature","type":"float"},{"name":"unit","type":"string"}]' \
  --update-mask fields
```

Use a file for complex field definitions:

```bash
tier0 uns update --path Plant/Line1/Metric/Temperature --fields-file fields.json --update-mask fields
```

## Rules

- `--path` is required.
- Use either `--fields` or `--fields-file`, not both.
- Provide at least one field to update; a path-only update is rejected.
- Use `--update-mask` to explicitly name the metadata fields being changed, such as `description`, `displayName`, `alias`, or `fields`.
- This command updates node metadata, not VQT data.
- Field updates may affect future writes and reads; verify schema before changing production topics.
- For current values, use `uns write`.

## Recommended Flow

```bash
tier0 uns browse --path Plant/Line1/Metric --include-metadata --json
tier0 uns update --path Plant/Line1/Metric/Temperature --description "Line 1 temperature" --update-mask description --dry-run --json
tier0 uns update --path Plant/Line1/Metric/Temperature --description "Line 1 temperature" --update-mask description
tier0 uns browse --path Plant/Line1/Metric --include-metadata --json
```

Inspect the dry-run body before execution. To clear a description, use the
shell-safe flag below; it keeps the empty string in the request without relying
on PowerShell empty-argument behavior:

```bash
tier0 uns update --path Plant/Line1/Metric/Temperature --clear-description --update-mask description --dry-run --json
tier0 uns update --path Plant/Line1/Metric/Temperature --clear-description --update-mask description
```
