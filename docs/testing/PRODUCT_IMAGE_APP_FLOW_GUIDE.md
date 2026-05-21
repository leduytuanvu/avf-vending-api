# Product Image App Flow Guide

## Overview

Product images upload via Cloudinary on the server. Clients send multipart `file` only — **no `company_id`**. The API resolves the stable server-side `MEDIA_COMPANY_ID`.

## End-to-end flow

1. **Login** — `POST /v1/auth/login` → Bearer access token
2. **Upload product image** — `POST /v1/admin/product-images`
   - multipart: `file` (required), `purpose=product_image` (optional), `altText` (optional)
   - headers: `Authorization`, `Idempotency-Key`, `X-Request-ID`, `X-Correlation-ID`
   - response: `mediaId`, `displayUrl`, `thumbnailUrl`, `provider`, `checksum`, `version`
3. **Create category** — admin catalog route
4. **Create brand** — admin catalog route
5. **Create product** — include `primaryMediaId` from upload response
6. **Assign product to machine** — planogram / assortment
7. **Verify machine catalog** — sale catalog snapshot includes image metadata and `cacheKey`

## Server configuration

| Variable | Purpose |
|----------|---------|
| `MEDIA_PROVIDER=cloudinary` | Enable Cloudinary provider |
| `MEDIA_UPLOAD_ENABLED=true` | Enable multipart upload route |
| `MEDIA_COMPANY_ID` | Stable non-nil UUID v7; mapped internally to media `company_id` |
| `CLOUDINARY_*` | Cloudinary credentials (secret server-side only) |

See [CLOUDINARY_MEDIA_SETUP.md](../deployment/CLOUDINARY_MEDIA_SETUP.md).

## Vending app offline image behavior

- **Online:** download `displayUrl` / `thumbnailUrl` from catalog snapshot
- **Offline:** use local cache keyed by `mediaId`, `version`, and `checksum`
- **Image changed:** refresh cache when `version`, `checksum`, or `mediaId` changes

## Postman production suite

Request: `[GATED-WRITE] POST /v1/admin/product-images (Cloudinary multipart)`

- Form fields: `file`, `purpose`, `altText` only (enabled)
- `company_id` field exists but is **disabled** — legacy override documentation only

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `400 invalid_argument: company_id` | Missing `MEDIA_COMPANY_ID` at runtime (pre-fix) or invalid override | Set `MEDIA_COMPANY_ID` on app node; redeploy |
| `503 capability_not_configured` | Cloudinary credentials or `MEDIA_COMPANY_ID` missing | Configure GitHub vars/secrets; run deploy env sync |
| Startup fails in production | `MEDIA_COMPANY_ID` missing with Cloudinary enabled | Set GitHub variable and redeploy |
