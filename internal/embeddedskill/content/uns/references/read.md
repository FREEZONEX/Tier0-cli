---
name: tier0-uns-read
description: "Read current VQT values from Tier0 UNS topics. Supports positional topic arguments and --topic."
---

# uns read

Use `read` to get current values for one or more full topic paths.

## Command

```bash
tier0 uns read Plant/Line1/Metric/Temperature --json
tier0 uns read --topic Plant/Line1/Metric/Temperature --json
tier0 uns read Plant/Line1/Metric/Temperature Plant/Line1/Metric/Humidity --json
tier0 uns read --topic 'Plant/+/Metric/Temperature' --json
```

## Rules

- The argument must be a complete topic leaf path.
- Folders such as `Plant/Line1` cannot be read.
- Positional topics and repeated `--topic` flags are both accepted.
- Use `--include-metadata` when the response must include topic metadata.
- Current VQT values are returned without `--include-leaf-value`. That flag only
  duplicates the VQT under `metadata.payload` when `--include-metadata` is also
  set; it is a no-op by itself for `uns read`.
- `GoodNoData` means the topic has no current cached value.

## Wildcard Requests

The server removes wildcard patterns that are fully covered by a broader
pattern and returns each expanded topic once. Callers should still avoid
redundant patterns so request intent stays clear. For example, the first pattern
already covers the second one because every `+` matches exactly one segment:

```text
+/+/+/+/+/+/Metric/Telemetry
SmartCity/+/+/+/+/+/Metric/Telemetry
```

Keep only the broader pattern unless the narrower pattern has a different
depth. Quote wildcard arguments so the shell does not interpret them.

## Response Shape

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "success": true,
    "results": [
      {
        "topic": "Plant/Line1/Metric/Temperature",
        "success": true,
        "result": {
          "value": { "temperature": 27.5 },
          "quality": "Good",
          "timeStamp": 1733382000000
        }
      }
    ]
  }
}
```

## Required Batch Checks

Check both `data.success` and each `data.results[i].success`. HTTP 200 does not guarantee every topic succeeded.
Read VQT fields from `data.results[i].result`, not directly from the result item.

## When to Use Something Else

- Use `browse.md` to inspect folders.
- Use `history.md` for time ranges.
- Use `search.md` when the exact topic path is unknown.
- Use `realtime.md` for continuous/event-driven values; do not poll this command.
