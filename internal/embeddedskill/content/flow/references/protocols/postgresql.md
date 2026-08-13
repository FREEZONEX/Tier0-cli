---
name: tier0-flow-protocol-postgresql
description: "PostgreSQL Flow guide: subscribe to UNS/MQTT messages, transform VQT payloads, and archive records into PostgreSQL."
---

# PostgreSQL - UNS Archive

Use this guide when the user wants to archive UNS values into PostgreSQL from a Node-RED Flow.

## Required Rules

1. Query available PostgreSQL nodes with `tier0 flow nodes --event --json`.
2. Confirm database host, port, database, schema, table, and credentials.
3. Use parameterized SQL where the node supports it.
4. Back up before deploy.
5. Preserve backend-created Tier0 MQTT broker config nodes when subscribing to Tier0 MQTT.

## Architecture

```text
[mqtt in] -> [function: normalize VQT] -> [postgresql]
```

## Suggested Table

```sql
CREATE TABLE IF NOT EXISTS uns_values (
  id BIGSERIAL PRIMARY KEY,
  topic TEXT NOT NULL,
  value JSONB NOT NULL,
  quality TEXT,
  ts TIMESTAMPTZ NOT NULL,
  inserted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## Normalize Function

```js
const payload = typeof msg.payload === "string" ? JSON.parse(msg.payload) : msg.payload;

const value = payload.value ?? payload;
const quality = payload.quality ?? "Good";
const ts = payload.timeStamp ? new Date(payload.timeStamp) : new Date();

msg.params = [
  msg.topic,
  JSON.stringify(value),
  quality,
  ts.toISOString()
];

msg.query = `
  INSERT INTO uns_values(topic, value, quality, ts)
  VALUES ($1, $2::jsonb, $3, $4::timestamptz)
`;

return msg;
```

## Deployment Flow

```bash
tier0 flow nodes --event --json
tier0 flow data --id <id> --out backup.json
# generate flows.json
tier0 flow deploy --id <id> -f flows.json --dry-run --json
# wait for user confirmation
tier0 flow deploy --id <id> -f flows.json --yes
```

## Template

`templates/postgresql-uns-archive.json` is a structural example. Replace database config, table name, SQL, topic filters, and preserve existing config nodes from the exported Flow.
