---
name: tier0-uns-history
description: "Query Tier0 UNS historical time-series values and aggregates. Read before using time range or aggregation flags."
---

# uns history

Use `history` for historical values and aggregate queries.

## Command

```bash
tier0 uns history -t Plant/Line1/Metric/Temperature --start -1h --json
tier0 uns history -t Plant/Line1/Metric/Temperature --start -24h --end now --fn avg --interval 1h --json
tier0 uns history -t Plant/Line1/Metric/Temperature --start 2026-01-01T00:00:00Z --end 2026-01-02T00:00:00Z --json
```

## Time Formats

| Format | Example |
| --- | --- |
| Relative | `-1h`, `-30m`, `-7d`, `-1w` |
| Absolute ISO 8601 | `2026-01-01T00:00:00Z` |
| Keyword | `now` |

## Aggregation

```bash
tier0 uns history \
  -t Plant/Line1/Metric/Temperature \
  --start -24h \
  --end now \
  --interval 1h \
  --fn avg \
  --field temperature \
  --json
```

Common functions: `avg`, `max`, `min`, `sum`, `count`.

## Rules

- `--topic` / `-t` and `--start` are required.
- Use full topic paths.
- Use `--field` when a topic value object has multiple numeric fields.
- Check batch business success inside `data.success` and `data.results`.

## When Not to Use

- Current values: use `read.md`.
- Topic discovery: use `browse.md` or `search.md`.
