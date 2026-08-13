---
name: tier0-flow-protocol-opcda
description: "OPC-DA Flow guide: poll OPC-DA items from Windows/DCOM environments and publish values into Tier0 UNS."
---

# OPC-DA - Polling to Tier0 UNS

Use this guide when the user must collect from legacy OPC-DA systems.

## Important Limitation

OPC-DA is based on Windows DCOM. The Node-RED runtime must run on a Windows host or in an environment that can access the target Windows OPC-DA server. A Linux container usually cannot connect directly to OPC-DA.

## Required Rules

1. Confirm Windows/DCOM connectivity before generating a Flow.
2. Get exact OPC server Class ID and Item IDs from the user or an OPC client tool.
3. Query available OPC-DA nodes with `tier0 flow nodes --source --json`.
4. Browse target UNS metadata before mapping fields.
5. Back up before deploy and preserve Tier0 MQTT broker config nodes.

## Architecture

```text
[inject timer] -> [opcda read] -> [function: map item values] -> [mqtt out]
```

## Common Settings

| Setting | Meaning |
| --- | --- |
| Host | OPC-DA server host |
| ProgID / Class ID | Exact OPC server identifier |
| Item ID | Vendor-specific tag path |
| Poll interval | Timer interval |
| Quality handling | Drop bad quality values unless user asks otherwise |

## Mapping Pattern

```js
const items = Array.isArray(msg.payload) ? msg.payload : [msg.payload];
const out = [];

for (const item of items) {
  if (item.quality && !String(item.quality).includes("Good")) continue;
  if (item.itemId === "Channel1.Device1.Temp") {
    out.push({
      topic: "Plant/Line1/Metric/Temperature",
      payload: JSON.stringify({ temperature: Number(item.value) })
    });
  }
}

return out;
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
