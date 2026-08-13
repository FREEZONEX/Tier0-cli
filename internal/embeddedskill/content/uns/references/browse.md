---
name: tier0-uns-browse
description: "Browse the Tier0 UNS namespace tree. Use for folder/path discovery and child node listing."
---

# uns browse

Use `browse` to list children under a namespace folder.

Do not use `browse` to read current values. Use `read.md` for full topic paths.

## Command

```bash
tier0 uns browse --path /
tier0 uns browse --path Plant/Line1 --json
tier0 uns browse --path Plant/Line1 --include-metadata --include-leaf-value --json
```

## Rules

- `--path` is a folder path. Use `/` for namespace root.
- If the user gives a leaf topic and asks for its current value, use `uns read`.
- Use `--include-metadata` when fields, topic type, alias, or description are needed.
- Use `--include-leaf-value` only when the backend supports returning leaf values in browse output.

## PowerShell

```powershell
tier0 uns browse --path 'Plant/Line1' --json
```

## Typical Flow

```bash
tier0 uns browse --path / --json
tier0 uns browse --path Plant --json
tier0 uns browse --path Plant/Line1 --include-metadata --json
```
