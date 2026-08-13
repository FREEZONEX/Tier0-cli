---
name: tier0-flow-protocol-mqtt-bridge
description: "MQTT bridge Flow guide: subscribe to an external MQTT broker, transform payloads, and publish to Tier0 UNS through the backend-created Tier0 MQTT broker config."
---

# MQTT Bridge - External MQTT to Tier0 UNS

Use this guide when external devices or systems already publish MQTT messages and the user wants to ingest them into Tier0 UNS.

## Do Not Use When

- The source device should be collected directly through Modbus or OPC-UA. Use the corresponding protocol guide.
- The user only wants to read current UNS values. Use `uns/references/read.md`.

## Required Rules

1. Back up before deploy: `tier0 flow data --id <id> --out backup.json`.
2. Query the target UNS topic fields before writing transformation code.
3. UNS write payloads must be objects, not bare numbers or strings.
4. Use a separate config node for the external MQTT broker.
5. Reuse the backend-created Tier0 `mqtt-broker` config node from `flow data`. Do not replace it.

## Tier0 MQTT Broker Config

Tier0 EMQX requires authentication and rejects anonymous connections. Backend Flow creation initializes a Tier0 `mqtt-broker` config node with `clientid`, `username`, and `password`. Node-RED stores credentials internally by node ID and does not export plaintext passwords reliably.

Agent rules:

- Find the existing Tier0 `mqtt-broker` config node in `backup.json`.
- Preserve its `id`, `broker`, `port`, and `clientid`.
- Omit credentials or keep the exported credential shape unchanged.
- Do not write custom `credentials.user` or `credentials.password`.
- Do not create a new Tier0 broker config node.

## Architecture

```text
[external mqtt-broker config]
[tier0 mqtt-broker config from backend]

[mqtt in] -> [function: transform payload] -> [mqtt out]
                 |
              [debug]
```

## External mqtt-broker Fields

| Field | Meaning |
| --- | --- |
| `broker` | External broker host |
| `port` | `1883` or `8883` |
| `clientid` | Client ID; may be blank if auto-generated |
| `usetls` | TLS toggle |
| `credentials` | Only for the external broker when needed |

## mqtt in

Subscribe to the external topic:

```json
{
  "type": "mqtt in",
  "topic": "devices/+/data",
  "qos": "0",
  "broker": "external-broker-node-id"
}
```

## Transform Function

The function converts external payloads to Tier0 UNS topic and payload.

Example for JSON payload:

```js
const data = typeof msg.payload === "string" ? JSON.parse(msg.payload) : msg.payload;
msg.topic = "Plant/Line1/Metric/Temperature";
msg.payload = JSON.stringify({ temperature: data.value });
return msg;
```

Example for a numeric payload:

```js
const value = Number(msg.payload);
msg.topic = "Plant/Line1/Metric/Temperature";
msg.payload = JSON.stringify({ temperature: value });
return msg;
```

Example splitting one payload into multiple UNS topics:

```js
const data = typeof msg.payload === "string" ? JSON.parse(msg.payload) : msg.payload;
return [
  {
    topic: "Plant/Line1/Metric/Temperature",
    payload: JSON.stringify({ temperature: data.temperature })
  },
  {
    topic: "Plant/Line1/Metric/Humidity",
    payload: JSON.stringify({ humidity: data.humidity })
  }
];
```

## mqtt out

Use the preserved Tier0 broker config node:

```json
{
  "type": "mqtt out",
  "topic": "",
  "qos": "0",
  "retain": "false",
  "broker": "<existing-tier0-mqtt-broker-config-id>"
}
```

Leave `topic` blank so `msg.topic` controls the UNS path.

## Deployment Flow

```bash
tier0 uns browse --path Plant/Line1/Metric --include-metadata --json
tier0 flow list --source --json
tier0 flow data --id <id> --out backup.json
# generate flows.json while preserving Tier0 mqtt-broker config
tier0 flow deploy --id <id> -f flows.json --dry-run --json
# wait for user confirmation
tier0 flow deploy --id <id> -f flows.json --yes
```

## Common Issues

| Symptom | Likely Cause | Fix |
| --- | --- | --- |
| MQTT out connects anonymously | Tier0 broker config node was replaced | Reuse the exported backend-created config node ID |
| UNS receives nothing | Payload transform is wrong | Add debug before mqtt out and inspect `msg.topic` / `msg.payload` |
| Write rejected | Payload does not match UNS schema | Browse metadata and adjust field names |
| External broker fails | Host, port, TLS, or external credentials are wrong | Verify external broker settings |
