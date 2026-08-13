---
name: tier0-auth-whoami
description: "Inspect the user, Workspace, roles, and permissions bound to the current API key. Use for API key diagnostics and permission checks."
---

# auth whoami - API Key Identity

Use this file to diagnose which user and Workspace the current CLI API key is bound to, and which roles or permissions it has.

`auth whoami` is diagnostic only. It is not required before every API call.

## Commands

```bash
# Preferred CLI command
tier0 auth whoami

# JSON output for agents
tier0 auth whoami --json

# Direct API call
tier0 api /openapi/v1/auth/whoami --body '{}'
```

## Response Fields

```json
{
  "userID": 1001,
  "userName": "Alice",
  "email": "alice@example.com",
  "workspaceID": 2001,
  "workspaceName": "Factory A",
  "keyPrefix": "sk-per-abc123",
  "keyType": "personal",
  "roles": ["admin"],
  "permissions": ["full_access"]
}
```

## Troubleshooting Rules

1. Use `workspaceID` and `workspaceName` to confirm the key targets the expected Workspace.
2. Use `permissions` to confirm that the key can call the target API. `full_access` can access all OpenAPI resources.
3. Use `keyType` to distinguish personal keys from service keys.
4. For HTTP 401, check that `tier0 config` has an API key, the key has not expired, and the BaseURL is correct.
