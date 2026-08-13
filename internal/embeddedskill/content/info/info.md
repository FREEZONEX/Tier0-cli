---
name: tier0-info
description: "Get Tier0 gateway service information. Triggers: Tier0, service info, status, health check, connectivity."
---

# info - Service Information

Use this file when the user asks to verify Tier0 gateway connectivity or service status.

## Command

```bash
tier0 api /openapi/v1/info --body '{}' --json
```

No business parameters are required.

## Typical Use

```bash
# Verify BaseURL and API key connectivity while debugging
tier0 api /openapi/v1/info --body '{}' --debug --json
```

## PowerShell

```powershell
tier0 api /openapi/v1/info --body '{}' --json
```

## Interpretation

Use a successful response to confirm that the configured `base-url` is reachable. Authentication and workspace permissions should be diagnosed with `auth/whoami.md`.
