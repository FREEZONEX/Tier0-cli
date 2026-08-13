---
name: tier0-flow-http-endpoints
description: "Expose and call user-defined SourceFlow or EventFlow HTTP endpoints built with Node-RED http in/http response nodes through the Tier0 CLI."
---

# Flow HTTP endpoints

Use this reference for application APIs and webhooks implemented in a Flow. These are user-defined `http in` routes, not Flow management or Node-RED Admin APIs.

## Route mapping

| Flow type | `http in.url` | CLI endpoint |
| --- | --- | --- |
| SourceFlow | `/api/device/webhook` | `/flow/source/api/device/webhook` |
| EventFlow | `/api/material/track` | `/flow/event/api/material/track` |

Use SourceFlow for device or source-system ingress. Use EventFlow for business events and application-facing processing.

## Required canvas chain

```text
http in -> processing nodes -> http response
```

- Match the `http in` method and URL exactly.
- Route every success and error branch to `http response`; otherwise the request waits until timeout.
- Read `nodes.md` before generating the canvas and confirm `http in` and `http response` are available.
- Export the existing canvas before editing, preserve system config nodes, then deploy through `tier0 flow deploy`.

Append nodes like these to the exported full canvas, using unique IDs and an existing tab ID for `z`:

```json
[
  {
    "id": "http-material-track",
    "type": "http in",
    "z": "<existing-tab-id>",
    "name": "Receive material event",
    "url": "/api/material/track",
    "method": "post",
    "upload": false,
    "swaggerDoc": "",
    "wires": [["handle-material-track"]]
  },
  {
    "id": "handle-material-track",
    "type": "function",
    "z": "<existing-tab-id>",
    "name": "Handle material event",
    "func": "msg.statusCode = 200;\nmsg.headers = { 'content-type': 'application/json' };\nmsg.payload = { ok: true, received: msg.payload };\nreturn msg;",
    "outputs": 1,
    "wires": [["reply-material-track"]]
  },
  {
    "id": "reply-material-track",
    "type": "http response",
    "z": "<existing-tab-id>",
    "name": "Return response",
    "statusCode": "",
    "headers": {},
    "wires": []
  }
]
```

| HTTP value | Node-RED value |
| --- | --- |
| JSON or form request body | `msg.payload` |
| Query parameters | `msg.req.query` |
| Request headers | `msg.req.headers` |
| Path parameters such as `/item/:id` | `msg.req.params` |
| Response body | `msg.payload` entering `http response` |
| Response status | `msg.statusCode` |
| Response headers | `msg.headers` |

## Call with the CLI

Use the generic `tier0 api` command. The CLI sends the configured Tier0 API key and treats the response as application-defined JSON rather than a Tier0 OpenAPI envelope.

Use `--json` when the endpoint returns a JSON object or array. Omit it for plain-text responses.

GET with query parameters:

```bash
tier0 api '/flow/source/api/device/status?id=device-1' \
  --method GET \
  --json
```

POST with an inline JSON body:

```bash
tier0 api /flow/event/api/material/track \
  --method POST \
  --body '{"materialId":"MAT-001","state":"arrived"}' \
  --json
```

For complex input, use a file:

```bash
tier0 api /flow/event/api/material/track \
  --method POST \
  --body-file request.json \
  --dry-run \
  --json
```

Inspect the dry-run URL, method, and body before invoking an endpoint that changes business state. Dry-run does not execute the Flow.

## Boundaries

- `/flow/source/**` and `/flow/event/**` are only for user-defined `http in` business routes.
- Manage Flow resources and canvases with `tier0 flow list/get/create/update/data/nodes/deploy/delete`.
- Do not call Node-RED Admin paths through `tier0 api`; the gateway blocks them.
- Tier0 API keys are credentials. Do not expose them in browser code, request bodies, logs, or examples.
- `tier0 api --body` accepts JSON only. It does not send arbitrary binary or form-data bodies.
- Interpret response fields according to the user-defined endpoint contract. A JSON field such as `code` or `data.success` is application data; HTTP status determines transport success.
- Design synchronous work to complete before the gateway timeout. Return early and continue asynchronously for long-running work.
