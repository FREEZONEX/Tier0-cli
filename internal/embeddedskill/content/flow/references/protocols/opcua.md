---
name: tier0-flow-protocol-opcua
description: "OPC-UA Flow guide: subscribe to OPC-UA nodes, parse DataValue payloads, and publish values into Tier0 UNS."
---

# OPC-UA - Subscription to Tier0 UNS

Use this guide when the user wants to collect data from an OPC-UA server.

## Required Rules

1. Query available nodes with `tier0 flow nodes --source --json`.
2. Confirm endpoint URL, security policy, and node IDs with the user or source system.
3. Browse target UNS topic metadata before writing mapping code.
4. Back up before deploy.
5. Preserve the backend-created Tier0 `mqtt-broker` config node.

## Architecture

```text
[inject/start] -> [OPC-UA client subscription] -> [function: map DataValue] -> [mqtt out]
```

## Common OPC-UA Settings

| Setting | Meaning |
| --- | --- |
| Endpoint | OPC-UA server URL, for example `opc.tcp://host:4840` |
| Security policy | `None`, `Basic256Sha256`, etc. |
| Security mode | `None`, `Sign`, or `SignAndEncrypt` |
| Node ID | Source variable, for example `ns=2;s=Channel.Device.Tag` |
| Subscription interval | Sampling interval |

## DataValue Mapping Pattern

Different OPC-UA nodes emit different payload shapes. Write defensive code:

```js
const payload = msg.payload;
const value =
  payload?.value?.value ??
  payload?.value ??
  payload;

msg.topic = "Plant/Line1/Metric/Temperature";
msg.payload = JSON.stringify({ temperature: Number(value) });
return msg;
```

For multiple tags, map by `msg.topic`, source node ID, or a property emitted by the OPC-UA node:

```js
const source = msg.topic || msg.nodeId || "";
const value = msg.payload?.value?.value ?? msg.payload?.value ?? msg.payload;

const mapping = {
  "ns=2;s=Line1.Temp": {
    topic: "Plant/Line1/Metric/Temperature",
    field: "temperature"
  },
  "ns=2;s=Line1.Speed": {
    topic: "Plant/Line1/Metric/Speed",
    field: "speed"
  }
};

const target = mapping[source];
if (!target) return null;

msg.topic = target.topic;
msg.payload = JSON.stringify({ [target.field]: Number(value) });
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

`templates/opcua-subscribe.json` is a structural example. Do not deploy it unchanged. Replace endpoint, node IDs, mapping code, topic paths, and preserve backend-created Tier0 MQTT config nodes.
