---
name: tier0-files
description: "Tier0 file operations: upload, download, get URL, and delete files in object storage (S3/RustFS)."
---

# tier0-files

Use this skill for Tier0 object storage file operations.

## Use When

- The user asks to upload a file to Tier0.
- The user asks to download a file from Tier0.
- The user asks to get a public or presigned URL for a file.
- The user asks to delete a file from Tier0.

## Do Not Use When

- The user wants UNS data operations. Use `uns/SKILL.md`.
- The user wants Flow operations. Use `flow/SKILL.md`.

## Non-Negotiable Rules

1. `upload` requires a local file path and returns `filePath` for subsequent operations.
2. `download`, `url`, and `delete` require the `filePath` returned by upload.
3. `useBy` defaults to `workspace` in CLI to avoid Cloud API Key `IsMember()` permission issue.
4. File size must be 0 < size ≤ 10MB.
5. Preview agent-generated `delete` requests with `--dry-run --json`.

## Routing

| Intent | Read |
| --- | --- |
| Upload a file | `references/upload.md` |
| Download a file | `references/download.md` |
| Get file access URL | `references/url.md` |
| Delete a file | `references/delete.md` |

## Common Commands

```bash
# Upload
tier0 assets upload ./report.csv --use-by workspace --visibility private

# Download
tier0 assets download --file-path workspace/.../report.csv -o ./report.csv

# Get URL
tier0 assets url --file-path workspace/.../report.csv --expired-sec 300

# Delete
tier0 assets delete --file-path workspace/.../report.csv --dry-run --json
# after user confirmation
tier0 assets delete --file-path workspace/.../report.csv --yes
```

## Mutation Preflight

Append `--dry-run --json` to the final delete command and inspect
`data.api[0].method`, `url`, and `body`. A preview does not authenticate, send a
request, or prove business success. Remove `--dry-run` only after the preview
matches the user's intent.

For delete, show the preview and destructive impact to the user, wait for
confirmation, then execute with `--yes`.
