# Postman Full Flow Manual Testing Guide

For new testers running every major AVF Vending API flow locally via Postman.

---

## 1. Prerequisites

### 1.1 Start local Docker services

```bash
docker compose -f deployments/docker/docker-compose.yml up -d
docker compose -f deployments/docker/docker-compose.yml --profile broker up -d   # MQTT + MinIO
```

| Service | Host:Port |
|---------|-----------|
| PostgreSQL | `127.0.0.1:15432` |
| Redis | `127.0.0.1:6379` |
| NATS | `127.0.0.1:4222` |
| EMQX MQTT | `127.0.0.1:1883` |

### 1.2 Migrate and seed

```bash
export DATABASE_URL="postgres://postgres:postgres@127.0.0.1:15432/avf_vending?sslmode=disable"
MIGRATIONS_DIR=migrations go run ./cmd/migrate up
```

Seed is applied by migration `00003_seed_dev.sql` (bootstrap admin — see `.env.example` for default credentials).

### 1.3 Start API

```bash
export DATABASE_URL=...   # same as above
export REDIS_URL=redis://127.0.0.1:6379
export NATS_URL=nats://127.0.0.1:4222
go run ./cmd/api
# Default REST: http://127.0.0.1:8080
```

Optional: `go run ./cmd/worker`, `go run ./cmd/mqtt-ingest` for outbox/MQTT paths.

### 1.4 Import Postman assets

1. Postman → **Import**
2. Collection: `postman/collections/avf-vending-api.postman_collection.json`
3. Environment: `postman/environments/avf-local.postman_environment.json`
4. Select **avf-local** environment in top-right dropdown

---

## 2. Environment variables

Set in **avf-local** before starting:

| Variable | Example | Notes |
|----------|---------|-------|
| `baseUrl` | `http://127.0.0.1:8080` | Required |
| `adminEmail` | from seed / `.env.example` | Bootstrap admin |
| `adminPassword` | from seed | Never commit real passwords |
| `accessToken` | *(empty)* | Auto-filled after login |
| `refreshToken` | *(empty)* | Auto-filled after login |
| `machineToken` | *(empty)* | After activation claim |
| `machineId` | *(empty)* | After machine create |
| `idempotencyKey` | auto | Collection prerequest |
| `resource_uuid` | auto | UUID v7 per request |

---

## 3. Authentication flow

### 3.1 Login

| Field | Value |
|-------|-------|
| **Purpose** | Obtain admin JWT |
| **Method/Path** | `POST {{baseUrl}}/v1/auth/login` |
| **Headers** | `Content-Type: application/json` |
| **Body** | `{"email":"{{adminEmail}}","password":"{{adminPassword}}"}` |
| **Expected 200** | `{ "access_token", "refresh_token", "expires_in" }` |
| **Save** | `accessToken`, `refreshToken` (collection test script) |

**Errors:** wrong password → **401**; locked account → **403**

### 3.2 Auth me

| Field | Value |
|-------|-------|
| **Method/Path** | `GET {{baseUrl}}/v1/auth/me` |
| **Headers** | `Authorization: Bearer {{accessToken}}` |
| **Expected 200** | User profile with roles |

### 3.3 Refresh

| Field | Value |
|-------|-------|
| **Method/Path** | `POST {{baseUrl}}/v1/auth/refresh` |
| **Body** | `{"refresh_token":"{{refreshToken}}"}` |
| **Expected 200** | New token pair |

### 3.4 Logout

| Field | Value |
|-------|-------|
| **Method/Path** | `POST {{baseUrl}}/v1/auth/logout` |
| **Headers** | Bearer + optional idempotency |
| **Expected 204** | Session invalidated |

---

## 4. Folder-by-folder sequence (happy path)

Run folders **top to bottom** in collection **Integrated — product media offline cache** (or main module folders).

### 4.1 Catalog — Category

| Field | Value |
|-------|-------|
| **POST** | `/v1/admin/categories` |
| **Headers** | Bearer, `Idempotency-Key: {{idempotencyKey}}`, `Content-Type: application/json` |
| **Body** | `{"name":"Test Category","code":"test-cat-{{resource_uuid}}"}` |
| **Expected 201** | `{ "id", "name", "code" }` |
| **Save** | `categoryId` ← `id` |

### 4.2 Catalog — Brand

| **POST** | `/v1/admin/brands` |
| **Body** | `{"name":"Test Brand","code":"test-brand-{{resource_uuid}}"}` |
| **Save** | `brandId` |

### 4.3 Catalog — Tag

| **POST** | `/v1/admin/tags` |
| **Body** | `{"name":"Test Tag","slug":"test-tag-{{resource_uuid}}"}` |
| **Save** | `tagId` |

### 4.4 Media — Init upload

| **POST** | `/v1/admin/media/init-upload` |
| **Body** | `{ "filename": "product.jpg", "content_type": "image/jpeg", "purpose": "product_image" }` |
| **Save** | `mediaId`, upload URL fields |

### 4.5 Media — Complete upload

| **POST** | `/v1/admin/media/{{mediaId}}/complete` |
| **Expected 200** | Asset ready |

### 4.6 Product create

| **POST** | `/v1/admin/products` |
| **Body** | Include `category_id`, `brand_id`, `sku`, `name`, link `media_id` if applicable |
| **Save** | `productId` |

### 4.7 Site / Region

| **POST** | `/v1/admin/regions` then `/v1/admin/sites` |
| **Save** | `regionId`, `siteId` |

### 4.8 Machine provision

| **POST** | `/v1/admin/machines` |
| **Body** | `site_id`, `serial_number`, hardware profile |
| **Save** | `machineId` |

### 4.9 Activation

1. **POST** `/v1/admin/machines/{{machineId}}/activation-codes` → save `activationCode`
2. **POST** `/v1/machines/claim` with code → save `machineToken`

### 4.10 Planogram / publish

1. Create planogram + slots assigning `productId`
2. Publish to machine
3. **GET** machine slots — verify assignment

### 4.11 Machine sync (REST or gRPC)

| **GET** | `/v1/machines/{{machineId}}/sale-catalog?include_images=true` |
| **Headers** | `Authorization: Bearer {{machineToken}}` |

gRPC alternative: `MachineBootstrapService/GetBootstrap` via grpcurl (see GRPC report).

### 4.12 Commerce — Order + vend

Use machine JWT folders:

1. Create order / vend session
2. Payment attempt (test provider)
3. Vend success callback
4. **GET** inventory — quantity decremented

### 4.13 Audit verification

| **GET** | `/v1/admin/audit-events?limit=20` |
| **Expected** | Events for login, catalog changes, vend |

---

## 5. Full happy-path checklist

- [ ] Admin login → tokens stored
- [ ] Category, brand, tag created
- [ ] Media init + complete
- [ ] Product with image metadata
- [ ] Site + machine provisioned
- [ ] Machine activated (`machineToken`)
- [ ] Planogram published to slots
- [ ] Sale catalog sync returns product
- [ ] Order + payment + vend success
- [ ] Inventory decremented
- [ ] Audit events present

---

## 6. Negative-path tests

| Test | How | Expected |
|------|-----|----------|
| Bad auth | Omit `Authorization` on admin route | **401** |
| Forbidden role | Use machine token on admin route | **403** |
| Invalid UUID | Path `.../not-a-uuid` | **400** or **404** |
| Duplicate idempotency | Repeat same POST with same `Idempotency-Key` | **200/201** replay, same resource id |
| Insufficient stock | Vend with qty > slot quantity | **409** or domain error |
| Payment failure | Webhook with failed status | Order stays unpaid; see reconciliation |
| Vend timeout | No machine ACK | Command timeout; audit/conflict events |

---

## 7. Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| **401** | Expired/missing token | Re-login; check `accessToken` env |
| **403** | RBAC or wrong token type | Use admin JWT for admin routes; machine JWT for machine routes |
| **404** | Wrong id or route | Verify saved `productId`/`machineId`; check `baseUrl` |
| **409** | Duplicate code/SKU/idempotency conflict | Use unique `code`/`sku`; new idempotency key |
| **422** | Validation | Read response body; fix required fields |
| **500** | Server/DB error | Check API logs; verify migrations at version 5 |
| Migration mismatch | Old schema | `go run ./cmd/migrate up` |
| Postman URL import error | String URL instead of object | Re-import regenerated collection |
| MQTT unavailable | Broker down | `docker compose --profile broker up -d` |
| gRPC connection refused | gRPC not listening | Start API with gRPC enabled; check `:9090` |

---

## 8. Related docs

- `docs/reports/verification/POSTMAN_COLLECTION_ENVIRONMENT_AUDIT.md`
- `docs/testing/e2e-local-test-guide.md`
- `docs/api/mqtt-contract.md`
- `docs/swagger/swagger.json` (OpenAPI source of truth)
