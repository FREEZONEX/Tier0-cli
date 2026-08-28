---
name: tier0-uns-history
description: "Query Tier0 UNS historical time-series values and aggregates. Read before using time range or aggregation flags."
---

# uns history

Use `history` for historical values and aggregate queries.

## Command

```bash
tier0 uns history -t Plant/Line1/Metric/Temperature --start -1h --json
tier0 uns history -t Plant/Line1/Metric/Temperature --start -24h --end now --count-mode none --json
tier0 uns history -t Plant/Line1/Metric/Temperature --start -7d --end now --auto-sparse --json
tier0 uns history -t Plant/Line1/Metric/Temperature --start -24h --end now --interval 1h --aggregate-field temperature=avg --json
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

Common functions: `avg`, `max`, `min`, `sum`, `count`, `first`, `last`.

Use repeatable `--aggregate-field name=function` for multiple fields:

```bash
tier0 uns history \
  -t Plant/Area1/Meter01/Metric/Daily \
  --start 2026-06-01T00:00:00Z \
  --end 2026-06-08T00:00:00Z \
  --interval 8d \
  --aggregate-field electricity=last \
  --aggregate-field water=last \
  --aggregate-field heat=last \
  --count-mode none \
  --auto-sparse \
  --json
```

Do not combine `--aggregate-field` with the legacy `--field`/`--fn` pair. A
field can occur only once in one request. If the function is omitted, numeric
fields default to `avg` and other fields default to `last`.

## Performance Parameter Selection

| Scenario | Recommended CLI parameters | Caller behavior |
|---|---|---|
| Raw pages without an exact total | `--count-mode none --page N --size M` | Treat `size` as the limit **per topic**, batch 50–100 topics per request, keep total HTTP concurrency at 2, and follow each item's `meta.hasMore` |
| UI must display an exact total/page count | Omit `--count-mode` or use `--count-mode exact` | Accept the additional exact `COUNT` cost |
| Trend chart with bounded points | `--count-mode none --auto-sparse` | `--auto-sparse` omits both `page` and `size`; do not combine it with either explicit flag |
| One aggregate for a complete window | `--count-mode none --interval <full-window>` plus only the required aggregate fields | Return at most one aggregate bucket per topic |
| Cumulative Daily/Monthly usage | Run the same window twice with `first` and `last` | Compute `last - first` in the application; do not use `avg` or `sum` as usage |

The compatible CLI default still sends `page=1`, `size=100`, and uses exact
counting. With `--count-mode none`, the response has `data.total=-1`,
`data.totalExact=false`, and `result.meta.rawTotal=-1`.

Skipping exact counts reduces database work only; it does **not** reduce the
response body. Reduce the per-topic `size`, use `--auto-sparse` or aggregation,
and batch topics when response size is the bottleneck. Never treat `size` as a
request-wide limit.

## Cumulative Meter Readings

Daily or Monthly frozen registers are cumulative readings. Query the same
window twice, once with `first` and once with `last`, then calculate
`last - first` in the application. Do not use `avg` or `sum` as usage, and do
not add a platform statistics table for this application-level calculation.

Use one interval that covers the complete window so each topic returns at most
one boundary bucket:

```bash
tier0 uns history \
  -t Plant/Area1/Meter01/Metric/Daily \
  --start 2026-06-01T00:00:00Z \
  --end 2026-06-08T00:00:00Z \
  --interval 8d \
  --aggregate-field electricity=first \
  --aggregate-field water=first \
  --aggregate-field heat=first \
  --count-mode none \
  --auto-sparse \
  --json

tier0 uns history \
  -t Plant/Area1/Meter01/Metric/Daily \
  --start 2026-06-01T00:00:00Z \
  --end 2026-06-08T00:00:00Z \
  --interval 8d \
  --aggregate-field electricity=last \
  --aggregate-field water=last \
  --aggregate-field heat=last \
  --count-mode none \
  --auto-sparse \
  --json
```

If `last < first`, treat it as a possible register reset or device replacement;
do not hide it with `abs()` or blindly clamp it to zero.

## Rules

- `--topic` / `-t` and `--start` are required.
- Use full topic paths.
- Use `--field` when a topic value object has multiple numeric fields.
- Check batch business success inside `data.success` and `data.results`.

## When Not to Use

- Current values: use `read.md`.
- Topic discovery: use `browse.md` or `search.md`.
