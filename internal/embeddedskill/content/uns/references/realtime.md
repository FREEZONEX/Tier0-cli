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
`uns history` to simulate a subscription. Use `tier0 mqtt subscribe` for an
external consumer, or an EventFlow when the stream should remain inside Tier0.

For a continuous workflow:

1. Ask where the stream should run or be delivered (EventFlow, application,
   database, or another consumer).
2. For a CLI or external consumer, read `../../mqtt/SKILL.md`. Create a
   revocable MQTT credential explicitly, save it under a named local profile,
   and subscribe with a bounded `--count` or `--timeout` when an agent runs the
   command. Example:

   ```bash
   tier0 mqtt subscribe --credential agent \
     --topic 'Plant/+/Metric/Temperature' --timeout 30s --json
   ```

3. For a Tier0 EventFlow, export the current canvas and reuse its
   backend-created Tier0 `mqtt-broker` config node. Add an `mqtt in` node for
   the required UNS topic/filter. Read `../../flow/references/protocols/mqtt-bridge.md`.
4. For a non-Tier0 MQTT client, use a dedicated credential and the broker/TLS
   settings returned or confirmed by the deployment. Never copy Node-RED
   credentials out of a Flow and never reuse the Tier0 API key as the MQTT
   password unless the user explicitly selects that platform capability.

Quote MQTT filters containing `+` or `#` in shells. Publishing does not accept
wildcards.
