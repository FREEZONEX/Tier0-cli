---
name: tier0-platform-members
description: "Query Tier0 platform members, Workspace roles, and user statuses."
---

# platform members - Query Platform Members

Use this reference to list or filter users in the API key's Workspace. This is a read-only operation and has no project path parameter.

## Availability

The public SaaS gateway currently returns HTTP 404 for this path. Do not call
it on `https://tier0.dev` or present it as generally available until the
service deploys the endpoint. On a private deployment, call it only after the
deployment owner confirms support.

Cloud API keys need the `uns:read` resource key. Enterprise API keys need the `launchpad.view` resource; all standard read-only, writer, and full-access keys include the required read resource.

## API

```text
POST /openapi/v1/platform/getMembers
```

## Request Body

All fields are optional.

| Field | Type | Meaning |
|---|---|---|
| `keyword` | string | Exact numeric user ID, or a case-insensitive substring of username, nickname, or email |
| `roleKey` | string | Match one role key |
| `roles` | string[] | Match any listed role key |
| `statuses` | string[] | Match `active` or `disabled` users |
| `updatedAtStart` | string | Include members updated at or after this RFC3339 timestamp |
| `updatedAtEnd` | string | Include members updated at or before this RFC3339 timestamp |
| `page` | integer | Page number, default `1` |
| `size` | integer | Page size, default `20`, maximum `100` |

`roleKey` and `roles` form one case-insensitive OR filter. Matching users still return all assigned roles. Cloud exposes built-in role keys as `owner`, `builder`, and `operator`; legacy `admin` and `member` values are also accepted as filters. Enterprise preserves its stored role keys and role names.

The effective update time is the latest update to the user, Workspace membership, or assigned roles. Whole-second `updatedAtEnd` values include the full named second.

## Command

```bash
tier0 api /openapi/v1/platform/getMembers \
  --body '{"keyword":"alice","roles":["builder"],"statuses":["active"],"updatedAtStart":"2026-07-01T00:00:00Z","updatedAtEnd":"2026-07-31T23:59:59Z","page":1,"size":20}' \
  --json
```

List the first page without filters:

```bash
tier0 api /openapi/v1/platform/getMembers --body '{}' --json
```

## Success Response

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "list": [
      {
        "userId": "2001",
        "userName": "alice",
        "nickName": "Alice",
        "email": "alice@example.com",
        "status": "active",
        "roles": [
          {
            "roleId": "301",
            "roleKey": "builder",
            "roleName": "Builder",
            "description": "Can build and configure resources"
          }
        ],
        "updatedAt": "2026-07-17T08:30:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "size": 20
  }
}
```

`userId` and `roleId` are strings. The response does not include project member IDs, role-bound applications, member counts, or `accessLevel`.

## Troubleshooting

1. Inspect the response `code`; Cloud can return a business error envelope with HTTP 200.
2. For authentication or permission failures, run `tier0 auth whoami --json` and confirm the key belongs to the intended Workspace.
3. Use only RFC3339 strings for update-time filters and only `active` or `disabled` for statuses.
