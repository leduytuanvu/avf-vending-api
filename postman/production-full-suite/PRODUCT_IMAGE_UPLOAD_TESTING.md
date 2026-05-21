# Product Image Upload — Production Postman Testing

Guide for `POST /v1/admin/product-images` in the **AVF Production Full API Suite** collection.

## 1. Import collection and environment

1. Open Postman.
2. **Import** → select:
   - `postman/production-full-suite/avf-production-full.postman_collection.json`
   - `postman/production-full-suite/avf-production.postman_environment.json`
3. In the environment dropdown (top-right), select **AVF Production**.

## 2. Required environment variables

| Variable | Example / placeholder | Notes |
|----------|----------------------|-------|
| `baseUrl` | `https://api.ldtv.dev` | Production API base URL |
| `adminEmail` | `admin@ldtv.dev` | Your platform admin email |
| `adminPassword` | `<set-in-postman>` | **Set locally** — never commit real passwords |
| `allowGatedWrites` | `true` | Required for write requests |
| `confirmProductionWrites` | `I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` | Required for write requests |
| `accessToken` | `<auto-populated-after-login>` | Set automatically after login |
| `refreshToken` | `<auto-populated-after-login>` | Set automatically after login |

Optional (already defaulted in environment):

- `allow_destructive` = `true`
- `canaryMode` = `true`
- `readiness` = `true`

## 3. Login

1. Open **Auth** → `POST /v1/auth/login`.
2. Set `adminPassword` in the environment (Current value).
3. Send the request.
4. On **200**, collection tests save `accessToken` and `refreshToken` to the environment.
5. Optional: run `GET /v1/auth/me` to confirm roles include platform admin.

## 4. Product image upload

1. Open **`[GATED-WRITE] POST /v1/admin/product-images - Upload Product Image`** (under Product Media / Media Admin).
2. **Headers** — confirm there is **no** manual `Content-Type` header. Postman must auto-generate `multipart/form-data; boundary=...`.
3. **Body** → **form-data**:

   | Key | Type | Value |
   |-----|------|-------|
   | `file` | File | Select a local png/jpg/jpeg/webp/gif ≤ 5MB |
   | `purpose` | Text | `product_image` |
   | `altText` | Text | `Coca Cola 330ml product image` |

4. Do **not** add `company_id` — backend uses server-side `MEDIA_COMPANY_ID`.
5. Send.

### Very important

**Do not manually set `Content-Type`** for this request. If you add `application/json` or a bare `multipart/form-data` without boundary, the server may reject the file with `invalid_image_file` / `unsupported content type`.

The pre-request script removes any stray `Content-Type` header before send.

## 5. Supported files

- `image/jpeg`
- `image/png`
- `image/webp`
- `image/gif`

Maximum size: **5MB**.

## 6. Expected success response (200/201)

```json
{
  "mediaId": "...",
  "purpose": "product_image",
  "filename": "...",
  "contentType": "image/png",
  "status": "ready",
  "url": "...",
  "secureUrl": "...",
  "thumbUrl": "...",
  "displayUrl": "...",
  "width": 800,
  "height": 800,
  "sizeBytes": 481360,
  "checksumSha256": "..."
}
```

Tests auto-save `mediaId` and `productImageDisplayUrl` to the environment.

## 7. Troubleshooting

| Symptom | Fix |
|---------|-----|
| `GATED-WRITE blocked` | Set `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` |
| Missing idempotency | Pre-request sets `_runtimeIdempotencyKey`; re-send or re-import collection |
| `invalid_image_file` / `unsupported content type` | Remove manual `Content-Type` header; re-select file; try png; use curl fallback below |
| `invalid_argument` / `company_id` | Remove `company_id` from body — not required |
| **401** unauthorized | Login first; check `accessToken` is set |

## 8. Curl fallback

Replace `YOUR_ACCESS_TOKEN` and file path:

```bash
curl -i -X POST "https://api.ldtv.dev/v1/admin/product-images" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "X-Request-ID: $(uuidgen)" \
  -H "X-Correlation-ID: $(uuidgen)" \
  -H "Idempotency-Key: $(uuidgen)" \
  -F "file=@/path/to/image.png;type=image/png" \
  -F "purpose=product_image" \
  -F "altText=Coca Cola 330ml product image"
```

## 9. Re-import after updates

If upload still fails after a repo update, **re-import** both the collection and environment files so Postman picks up header/body fixes.
