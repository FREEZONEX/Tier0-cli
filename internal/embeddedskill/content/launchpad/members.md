---
name: tier0-launchpad-members
description: "Query members of a Tier0 project, including roles and role-bound applications."
---

# launchpad members - Query Project Members

Use this reference when the user wants to list or filter members of a project in the API key's Workspace.

## Availability

The public SaaS gateway currently returns HTTP 404 for this path. Do not call
it on `https://tier0.dev` or present it as generally available until the
service deploys the endpoint. On a private deployment, call it only after the
deployment owner confirms support.

This is a read-only operation. The configured API key must grant the `uns:read` resource key. The built-in `read_only`, `data_writer`, and `full_access` permission levels all grant this resource key.

## API

```text
POST /openapi/v1/launchpad/:projectName/getMembers
```

`projectName` is a single URL path segment and must be URL-encoded. For example, use `Factory%20Analytics` for `Factory Analytics`. The API key determines the Workspace in which the project name is resolved. If multiple projects in that Workspace have the same name, use the project's ID in this path to disambiguate it.

## Request Body

All body fields are optional.

| Field | Type | Meaning |
| --- | --- | --- |
| `roleKey` | string | Match members assigned to this role key |
| `roles` | string[] | Match members assigned to any of these role keys |
| `updatedAtStart` | string | Include members updated at or after this RFC3339 timestamp |
| `updatedAtEnd` | string | Include members updated at or before this RFC3339 timestamp |
| `page` | integer | Page number, default `1` |
| `size` | integer | Page size, default `20`, maximum `100` |

`roleKey` and `roles` are combined into one case-insensitive role-key filter. A member matches when any supplied role key matches. Duplicate or blank role keys are ignored. Matching members still return all of their assigned roles, not only the roles used for filtering.

Both update-time filters must use RFC3339, for example `2026-07-01T00:00:00Z`. The end timestamp is inclusive; a timestamp expressed to whole-second precision includes that entire second.

## Command

```bash
tier0 api /openapi/v1/launchpad/Factory%20Analytics/getMembers \
  --body '{"roleKey":"owner","roles":["builder","operator"],"updatedAtStart":"2026-07-01T00:00:00Z","updatedAtEnd":"2026-07-31T23:59:59Z","page":1,"size":20}' \
  --json
```

Use an empty object to list the first page without filters:

```bash
tier0 api /openapi/v1/launchpad/Factory%20Analytics/getMembers --body '{}' --json
```

## Success Response

The gateway returns the standard OpenAPI envelope:

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "list": [
      {
        "memberId": "101",
        "userId": "2001",
        "userName": "Alice",
        "email": "alice@example.com",
        "roles": [
          {
            "roleId": "301",
            "roleKey": "builder",
            "roleName": "Builder",
            "description": "Can build and configure project resources",
            "memberCount": 4,
            "apps": [
              {
                "appId": "401",
                "appName": "Operations Console"
              }
            ]
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

## Response Fields

| Field | Type | Meaning |
| --- | --- | --- |
| `data.list` | member[] | Members on the requested page |
| `data.total` | integer | Total members matching the filters |
| `data.page` | integer | Normalized page number |
| `data.size` | integer | Normalized page size |
| `memberId` | string | Project member identifier |
| `userId` | string | User identifier |
| `userName` | string, optional | User display name |
| `email` | string, optional | User email address |
| `roles` | role[] | All roles assigned to the member |
| `updatedAt` | string, optional | Member update time in RFC3339 |
| `roleId` | string | Project role identifier |
| `roleKey` | string | Role key used for filtering and authorization |
| `roleName` | string, optional | Localized role display name |
| `description` | string, optional | Role description |
| `memberCount` | integer | Number of project members assigned to the role |
| `apps` | app[], optional | Applications bound to the role |
| `appId` | string | Application identifier |
| `appName` | string | Application name |

`data.list` and each member's `roles` are arrays even when empty. Optional profile, localization, timestamp, and application fields may be omitted.

## Troubleshooting

1. Always inspect the response `code`. The cloud gateway can return a business error envelope with HTTP 200.
2. For authentication or permission errors, verify the configured API key and BaseURL, then run `tier0 auth whoami --json` and inspect `permissions`. The current response does not expose a `resourceKeys` field.
3. For a not-found error, verify the URL-encoded project name and that the API key belongs to the expected Workspace.
4. For an ambiguous-name error, retry with the project's ID instead of its name.
