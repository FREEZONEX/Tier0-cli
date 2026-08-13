---
name: tier0-flow-protocol-modbus
description: "Modbus TCP/RTU Flow guide: poll registers, transform values, and publish them into Tier0 UNS."
---

# Modbus - Device Data to Tier0 UNS

Use this guide when the user wants to collect data from Modbus TCP or RTU devices.

## Required Rules

1. Query available nodes with `tier0 flow nodes --source --json`.
2. Browse target UNS topics with metadata before mapping fields.
3. Back up before deploy with `tier0 flow data --id <id> --out backup.json`.
4. Preserve backend-created Tier0 `mqtt-broker` config nodes.
5. Publish UNS payloads as objects.

## Architecture

```text
[inject timer] -> [modbus read] -> [function: map registers] -> [mqtt out]
```

## Common Modbus Settings

| Setting | Meaning |
| --- | --- |
| Host | PLC or gateway IP |
| Port | Usually `502` for TCP |
| Unit ID | Modbus slave/unit identifier |
| Function code | Holding/input register or coil type |
| Address | Register start address |
| Quantity | Number of registers |
| Poll interval | Inject repeat interval |

## Mapping Function Pattern

```js
const registers = Array.isArray(msg.payload) ? msg.payload : msg.payload?.data;
if (!Array.isArray(registers)) {
  node.warn("Unexpected Modbus payload");
  return null;
}

const raw = registers[0];
msg.topic = "Plant/Line1/Metric/Temperature";
msg.payload = JSON.stringify({ temperature: raw / 10 });
return msg;
```

## Deployment Flow

```bash
tier0 uns browse --path Plant/Line1/Metric --include-metadata --json
tier0 flow nodes --source --json
tier0 flow data --id <id> --out backup.json
# generate flows.json
tier0 flow deploy --id <id> -f flows.json --dry-run --json
# wait for user confirmation
tier0 flow deploy --id <id> -f flows.json --yes
```

## Template

`templates/modbus-tcp-read.json` is a structural example. Do not deploy it unchanged. Replace node IDs, server settings, register mapping, UNS topic paths, and preserve the existing Tier0 MQTT broker config from the exported Flow.
