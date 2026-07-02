# Product Image Cloudinary Smoke Test

## Prerequisites

```env
MEDIA_PROVIDER=cloudinary
MEDIA_UPLOAD_ENABLED=true
CLOUDINARY_CLOUD_NAME=...
CLOUDINARY_API_KEY=...
CLOUDINARY_API_SECRET=...
```

## Postman

1. Import [`postman/suites/production-full/avf-vending-production.full.postman_collection.json`](../../postman/suites/production-full/avf-vending-production.full.postman_collection.json)
2. Import [`postman/suites/production-full/avf-vending-production.full.postman_environment.json`](../../postman/suites/production-full/avf-vending-production.full.postman_environment.json)
3. Set `adminPassword` locally
4. Run **Auth → POST /v1/auth/login**
5. Run **Product Media → POST /v1/admin/product-images**
   - Body: form-data
   - `file`: choose local `.png` / `.jpg`
   - `purpose`: `product_image`
6. Confirm response: `mediaId`, `displayUrl`, `thumbnailUrl`
7. Open `displayUrl` in browser
8. Create category/brand/tag (canary flow)
9. **Products → POST /v1/admin/products** with `"primaryMediaId": "{{mediaId}}"`
10. Assign product to machine (existing planogram/assortment flow)
11. **Machine Runtime → GET sale catalog / gRPC SyncSaleCatalog** — confirm image metadata

## Offline cache (app contract)

- Online: download `thumbnailUrl` or `displayUrl` using `cacheKey`
- Unchanged `mediaId` + `version` + `checksum`: keep local file
- Changed version/checksum: refresh download
- Offline with cache: show local image
- Offline without cache: placeholder

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| 503 `capability_not_configured` | Set Cloudinary env vars; redeploy |
| 400 `invalid_image_file` | Use png/jpg/webp/gif; check magic bytes |
| 413 `file_too_large` | Reduce file or raise `MEDIA_MAX_IMAGE_SIZE_MB` |
| 400 `missing_idempotency_key` | Add `Idempotency-Key: {{$guid}}` |
| 401 | Re-run login |
