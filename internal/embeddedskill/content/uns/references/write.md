---
name: tier0-uns-write
description: "Write current values into Tier0 UNS topics. Values must be JSON objects matching topic fields."
---

# uns write

Use `write` to publish a value to one or more UNS topics.

## Command

```bash
tier0 uns write --topic Plant/Line1/Metric/Temperature --value '{"temperature":27.5}'
tier0 uns write --topic Plant/Line1/Metric/Temperature --file payload.json
tier0 uns write --topic Plant/Line1/Metric/Temperature --value '{"temperature":27.5}' --qos 1 --retain
```

## Preflight

Preview the exact topic, payload, QoS, and retain settings before publishing:

```bash
tier0 uns write --topic Plant/Line1/Metric/Temperature --file payload.json --dry-run --json
```

Inspect the request body, then execute the same command without `--dry-run`.

## Rules

- `--topic` is required.
- Use either `--value` or `--file`, not both.
- `--qos` must be `0`, `1`, or `2`.
- The value must be a JSON object. Do not send a bare number or string.
- Field names should match the topic schema.
- Check batch business success inside `data.success` and `data.results`.

## Payload Example

```json
{
  "temperature": 27.5,
  "unit": "C"
}
```

## PowerShell

Prefer files for complex JSON:

```powershell
tier0 uns write --topic 'Plant/Line1/Metric/Temperature' --file payload.json
```

## Troubleshooting

| Symptom | Likely Cause | Fix |
| --- | --- | --- |
| Schema or field error | Value keys do not match topic fields | Run `uns browse --include-metadata` and adjust keys |
| Invalid JSON | Shell quoting issue | Use `--file` |
| No current value after write | Topic path is wrong or backend rejected item | Check `data.results[i].success` |
