# Product Image Upload — Production Postman Testing

See **Step 5** in `PRODUCT_CATALOG_FLOW_TESTING.md` for the full catalog flow.

## Request

**POST** `/v1/admin/product-images`

### Headers

- `Accept: application/json`
- `Authorization: Bearer {{accessToken}}`
- `X-Request-ID: {{_runtimeRequestId}}`
- `X-Correlation-ID: {{_runtimeCorrelationId}}`
- `Idempotency-Key: {{_runtimeIdempotencyKey}}`
- **Do not** set `Content-Type` manually

### Body (form-data)

| Key | Type | Value |
|-----|------|-------|
| file | File | Select png/jpg/jpeg/webp/gif ≤ 5MB |
| purpose | Text | `product_image` |
| altText | Text | `Coca Cola 330ml product image` |

Pre-request removes any stray `Content-Type` header so Postman auto-generates multipart boundary.

## Expected success (200/201)

Response includes `mediaId`, `displayUrl`, `secureUrl`, `contentType`, `status: ready`.

Tests save `mediaId` and `primaryMediaId` for product create.

## Curl fallback

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

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `invalid_image_file` / unsupported content type | Remove manual Content-Type; reselect file; use curl `;type=image/png` |
| `GATED-WRITE blocked` | Set gated-write environment flags |
| `401` | Login first |
