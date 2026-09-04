---
name: tier0-flow-nodes
description: "List available Node-RED node types for a Tier0 Flow before generating canvas JSON."
---

# flow nodes

Use `flow nodes` before generating Flow JSON that depends on optional Node-RED nodes.

## Commands

```bash
tier0 flow nodes --source --json
tier0 flow nodes --event --json
tier0 flow nodes --type SourceFlow --json
```

## Rules

- Node availability depends on the Node-RED runtime image.
- Query nodes before using non-core node types.
- If a required node is missing, tell the user that the runtime needs the corresponding Node-RED package installed.

## Common Node Types

| Purpose | Node type |
| --- | --- |
| Inject timer | `inject` |
| JavaScript transform | `function` |
| Debug output | `debug` |
| MQTT input | `mqtt in` |
| MQTT output | `mqtt out` |
| MQTT broker config | `mqtt-broker` |
| HTTP ingress | `http in` |
| HTTP response | `http response` |
| HTTP request | `http request` |
| Modbus read | Usually from `node-red-contrib-modbus`; verify with `flow nodes` |
| OPC-UA client | Usually from `node-red-contrib-opcua`; verify with `flow nodes` |
| PostgreSQL | Usually from a PostgreSQL Node-RED package; verify with `flow nodes` |

## Important Config Node Rule

For Tier0 MQTT output, do not create a new Tier0 `mqtt-broker` config node. Export the existing Flow with `flow data` and reuse the backend-created config node.
