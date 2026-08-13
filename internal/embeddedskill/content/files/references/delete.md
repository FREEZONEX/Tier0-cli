---
name: tier0-files-delete
description: "Delete a file from Tier0 object storage."
---

# delete — Delete a file

## API

```
POST /openapi/v1/assets/files/delete
```

## Request

| Field | Type | Required | Description |
|------|------|------|------|
| `filePath` | string | **Yes** | Object key returned by upload |

## Response

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "deleted": true
  }
}
```

## CLI

```bash
tier0 assets delete --file-path workspace/.../report.csv
```

## Notes

- Deletion is irreversible.
- Only files belonging to the current workspace can be deleted.
