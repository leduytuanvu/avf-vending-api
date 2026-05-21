# AVF Production Full API Testing Guide

Generated: 2026-05-21T07:19:01Z

## 1. Import

1. Import `avf-production-full.postman_collection.json`
2. Import `avf-production.postman_environment.json`
3. Select **AVF Production** environment
4. Set `adminPassword` locally (never commit)

## 2. Default environment flags (ready to run)

| Variable | Default |
| --- | --- |
| allowGatedWrites | true |
| confirmProductionWrites | I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION |
| allow_destructive | true |
| canaryMode | true |
| readiness | true |

## 3. Recommended run order

1. **System Health** — `Health System` → REST → `/health/live`, `/health/ready`, `/version`
2. **Auth** → `POST /v1/auth/login` then `GET /v1/auth/me`
3. Domain REST folders (Catalog, Product Media, Products, Machines, …)
4. **Product Media** — `POST /v1/admin/media/uploads/init` (200 or 503, not raw 404)
5. **Product Media** — `POST /v1/admin/media/external-images` (201 or 503 if disabled)
6. Product create with `primaryMediaId`
7. Machine planogram / catalog assignment (canary)
8. gRPC / MQTT manual doc folders

## 4. Idempotency-Key

Write requests use `Idempotency-Key: {{$guid}}` directly.

## 5. Gated writes

`[GATED-WRITE]` requests require `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`.

## 6. Object storage vs external image URL

- **Upload init** requires object storage (`API_ARTIFACTS_ENABLED`) — else **503** `capability_not_configured`
- **External image URL** requires `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED` — else **503**

## 7. Troubleshooting

| Issue | Fix |
| --- | --- |
| invalid Authorization header | Re-run login; use `Bearer {{accessToken}}` |
| missing_idempotency_key | Ensure `Idempotency-Key: {{$guid}}` on writes |
| 401/403 | Check roles / token expiry |
| 404 route | Verify deploy version; media routes should return 401/503 not raw 404 |
| 503 capability_not_configured | Enable feature in production env or accept disabled capability |
| duplicate slug/SKU | Use `{{$timestamp}}` in slug/sku fields |

## 8. Coverage

- REST operations: 328
- gRPC methods (doc): 86
- MQTT topics (doc): 28
