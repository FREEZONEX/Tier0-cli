---
name: tier0-uns-realtime
description: "Choose MQTT/EventFlow instead of OpenAPI polling for continuous or event-driven Tier0 UNS consumption."
---

# Continuous UNS data

Use the access mode that matches the request:

| Need | Use |
| --- | --- |
| One current snapshot | `tier0 uns read` |
| A bounded time range or aggregate | `tier0 uns history` |
| Continuous/event-driven values | MQTT subscription or an EventFlow |

## Continuous-data rule

Do not repeatedly call `tier0 uns read`, `tier0 api /openapi/v1/uns/read`, or
`uns history` to simulate a subscription. The CLI currently has no
`uns subscribe` command and service info alone does not provide MQTT consumer
credentials.

For a continuous workflow:

1. Ask where the stream should run or be delivered (EventFlow, application,
   database, or another consumer).
2. For a Tier0 EventFlow, export the current canvas and reuse its
   backend-created Tier0 `mqtt-broker` config node. Add an `mqtt in` node for
   the required UNS topic/filter. Read `../../flow/references/protocols/mqtt-bridge.md`.
3. For an external MQTT client, proceed only when the deployment administrator
   supplies the broker TLS/auth settings and credentials. Do not infer them
   from service-info output and do not copy Node-RED credentials out of a Flow.
4. If neither option is available, state that continuous subscription is not
   exposed by the current CLI. Do not silently fall back to polling.

Use MQTT topic filters only when the broker and user permissions support them,
and quote `+` or `#` in shells.
