---
name: tier0-files-upload
description: "Upload a file to Tier0 object storage."
---

# upload — Upload a file

## API

```
POST /openapi/v1/assets/files
```

## Request

| Field | Type | Required | Description |
|------|------|------|------|
| `fileName` | string | **Yes** | Original file name |
| `contentType` | string | **Yes** | MIME type |
| `size` | number | **Yes** | File size in bytes（单文件上限见套餐/服务端配置，TASK-025 后统一 100MB） |
| `business` | string | No | Business scene, e.g. `attachment` |
| `useBy` | string | No | `user` / `workspace` / `platform`, default `user` |
| `visibility` | string | No | `public` / `private`, default `private` |
| `appInstanceId` | string | No | AI app instance ID |
| `sessionId` | string | No | AI session ID |

## Response（TASK-025 起为预签名 POST，PUT 已下线）

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "fileId": 12345,
    "filePath": "w335601780494560/common/20260804/fr_1cbdb330d6a1436c97039d375d1fc474/report.csv",
    "fileUrl": "",
    "expiresAt": 1784542276678,
    "postUrl": "https://tier0-upload-temp-pre.s3.ap-southeast-1.amazonaws.com/",
    "postFields": {
      "key": "w335601780494560/common/20260804/fr_1cbdb330d6a1436c97039d375d1fc474/report.csv",
      "policy": "...",
      "x-amz-algorithm": "AWS4-HMAC-SHA256",
      "x-amz-credential": "...",
      "x-amz-date": "...",
      "x-amz-signature": "..."
    }
  }
}
```

## CLI

```bash
tier0 assets upload ./report.csv --use-by workspace --visibility private
```

## Flow

1. CLI requests `POST /openapi/v1/assets/files` with file metadata（`size` 必填，签入 policy 的 `content-length-range [1, size]`）。
2. Backend returns `postUrl` + `postFields`（预签名 POST policy）和 `filePath`。
3. 以 `multipart/form-data` POST `postUrl` 直传：**`postFields` 全部先入表单，`file` 字段必须最后追加**，不额外设置 `Content-Type` 表单字段（浏览器/客户端自动生成 multipart boundary）。
4. 上传成功返回 204；低报 `size` 上传更大文件会被对象存储按 policy 直接拒绝（Cloud AWS S3 返回 403，RustFS 返回 400 EntityTooLarge）。
5. Save `filePath` for download/url/delete operations.

## Notes

- `postUrl`/`postFields` 默认有效期 3600 秒（`expiresAt` 为准）。
- `useBy=workspace` is recommended for API Key authentication on Cloud.
- `visibility=public` returns a long-lived `fileUrl`.
- Cloud 侧配额（套餐存储上限）在申请阶段预占，超限返回错误码 `3004001`（Storage quota exceeded）；单文件超过上限返回 `3004002`。
