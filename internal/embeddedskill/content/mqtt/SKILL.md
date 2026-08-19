---
name: tier0-mqtt
description: "Tier0 MQTT operations: create and revoke MQTT credentials, publish messages, and subscribe to continuous UNS or broker topics."
---

# Tier0 MQTT

Use this skill for Tier0's real-time MQTT data plane. Snapshot UNS reads and
schema-validated UNS writes remain under `uns/SKILL.md`.

## Non-negotiable rules

1. Check `tier0 auth whoami --json` before creating or deleting a credential.
2. Preview `mqtt auth create`, `mqtt auth delete`, and `mqtt publish` with
   `--dry-run --json` before executing the same operation.
3. Credential deletion requires user confirmation and `--yes`.
4. A newly created password is returned only by that create call. Prefer
   `--save <profile>`; never copy it into logs, chat, command arguments, or
   source files.
5. Use a dedicated MQTT credential. Do not automatically reuse the Tier0 API
   key as an MQTT password.
6. Keep TLS verification enabled. Use `--ca-file` for a private CA. Only use
   `--insecure-skip-verify` when the user explicitly accepts that risk.
7. Agent subscriptions must be bounded with `--count` or `--timeout`. Do not
   leave a background subscription running and do not fall back to OpenAPI
   polling.
8. Quote subscription filters containing `+` or `#`. Publish topics cannot
   contain MQTT wildcards.

## Credential lifecycle

Preview and create a credential with random client-ID suffix support:

```bash
tier0 mqtt auth create --name agent --save agent \
  --random-suffix=true --dry-run --json
tier0 mqtt auth create --name agent --save agent \
  --random-suffix=true
```

`--save` writes a user-only local profile under Tier0's config directory. The
profile is tied to the configured Tier0 base URL. Private deployments can pass
`--broker`, and MQTT commands can pass `--ca-file`.

Preview deletion, show the credential ID and impact to the user, wait for
approval, then execute:

```bash
tier0 mqtt auth delete --credential agent --dry-run --json
tier0 mqtt auth delete --credential agent --yes
```

## Publish

Use `--file` for complex JSON. MQTT publish accepts JSON objects, arrays,
scalars, text, or arbitrary bytes; `--json-message` adds JSON validation.

```bash
tier0 mqtt publish --credential agent \
  --topic Vision/Task1/office/State/Result \
  --file payload.json --json-message --qos 1 --dry-run --json

tier0 mqtt publish --credential agent \
  --topic Vision/Task1/office/State/Result \
  --file payload.json --json-message --qos 1
```

Publish dry-run output uses `data.mqtt`; inspect `broker`, `topic`, `qos`,
`retain`, and `payloadBytes`. It deliberately omits username and password.

## Subscribe

`--json` emits one NDJSON record per message. JSON payloads remain structured;
non-UTF-8 payloads are emitted as base64.

```bash
tier0 mqtt subscribe --credential agent \
  --topic 'Vision/+/office/State/Result' \
  --qos 1 --count 10 --timeout 60s --json
```

Use `--format raw` only when the caller wants payload bytes without topic and
delivery metadata. Connection status and debug details belong on stderr; stdout
is the message stream.

## External environment credentials

When a saved profile is inappropriate, set all four variables and omit
`--credential`:

```text
TIER0_MQTT_BROKER
TIER0_MQTT_CLIENT_ID
TIER0_MQTT_USERNAME
TIER0_MQTT_PASSWORD
```

`TIER0_MQTT_RANDOM_SUFFIX=true` enables a per-process client-ID suffix when the
server credential permits it.
