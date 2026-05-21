# Product Flow Test Guide And Result

Production canary for admin catalog → price → planogram → machine sale catalog at `https://api.ldtv.dev`.

**Important:** A product is catalog-level first. It is **not** sold on every machine until price book + planogram slot assignment + publish complete. The vending app should fetch the sale catalog online and cache `displayUrl` / `thumbnailUrl` for offline tiles.

## Variables

| Variable | Committed placeholder | Notes |
|----------|----------------------|--------|
| `baseUrl` | `https://api.ldtv.dev` | |
| `adminEmail` | `admin@ldtv.dev` | |
| `adminPassword` | `<set-in-postman>` | Never commit real password |
| `allowGatedWrites` | `true` | |
| `confirmProductionWrites` | `I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` | |
| `machineId` | `<set-existing-machine-id>` | Or run GET `/v1/admin/machines` (saves `machineId` from list) |
| `accessToken` | `<auto-populated-after-login>` | From `tokens.accessToken` |
| `categoryId`, `brandId`, `tagId` | set after create | |
| `primaryMediaId`, `primaryMediaUrl`, `primaryMediaThumbnailUrl` | set after image upload | |
| `productId`, `canaryProductSku` | set after product create | |
| `priceBookId`, `unitPriceMinor` | `15000` (= $150.00 USD) | |
| Local image file | Postman form-data `file` | Real png/jpg/webp/gif ≤ 5MB; **do not** set multipart Content-Type manually |

Import: `avf-product-admin-to-app-flow.postman_collection.json` + `avf-production.postman_environment.json`

## Flow

1. **Login** — `POST /v1/auth/login`
2. **Create category** — `POST /v1/admin/categories`
3. **Create brand** — `POST /v1/admin/brands`
4. **Create tag** — `POST /v1/admin/tags`
5. **Upload image** — `POST /v1/admin/product-images` (multipart)
6. **Create product** — `POST /v1/admin/products` (pre-request builds `_runtimeProductCreateBody`)
7. **Configure price** — price book create → PUT items → assign-target to machine
8. **Operator session (production)** — `POST /v1/admin/machines/{machineId}/operator-sessions/start` with admin JWT (`force_admin_takeover: true` if needed). Legacy `POST /v1/machines/{machineId}/operator-sessions/login` is **disabled** when `MachineRESTLegacyEnabled=false`.
9. **Planogram** — PUT draft → POST publish (body includes `operator_session_id` from step 8)
10. **Sale catalog (app)** — gRPC `MachineCatalogService.SyncSaleCatalog` / `GetSaleCatalog` with **Machine JWT** (not admin JWT). REST `GET /v1/machines/{machineId}/sale-catalog` is disabled in production by default.

## Key Requests

### Login

```http
POST /v1/auth/login
Content-Type: application/json

{"email":"{{adminEmail}}","password":"{{adminPassword}}"}
```

Success (sanitized):

```json
{
  "accountId": "d80fa04f-baec-4852-b55b-4017641e47b9",
  "email": "admin@ldtv.dev",
  "roles": ["platform_admin", "admin"],
  "tokens": { "accessToken": "[REDACTED]", "refreshToken": "[REDACTED]" }
}
```

### Upload product image

```http
POST /v1/admin/product-images
Authorization: Bearer {{accessToken}}
Idempotency-Key: {{_runtimeIdempotencyKey}}

form-data: file=<local image>, purpose=product_image, altText=Coca Cola 330ml product image
```

Success **201**:

```json
{
  "mediaId": "019e4ba2-8428-7e24-abcc-71c4dca83a9b",
  "displayUrl": "https://res.cloudinary.com/.../products/....png",
  "thumbnailUrl": "https://res.cloudinary.com/.../w_300,h_300,...",
  "contentType": "image/png",
  "status": "ready"
}
```

### Create product

Pre-request sets `{{_runtimeProductCreateBody}}` (camelCase `V1AdminProductMutationRequest`):

```json
{
  "sku": "COCA-330ML-1779385403057",
  "name": "Coca Cola Can 330ml",
  "description": "Production canary test product",
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "categoryId": "<uuid>",
  "brandId": "<uuid>",
  "primaryMediaId": "<uuid>"
}
```

```http
POST /v1/admin/products
Content-Type: application/json
Body: {{_runtimeProductCreateBody}}
```

### Start operator session (production admin path)

```http
POST /v1/admin/machines/{{machineId}}/operator-sessions/start
Authorization: Bearer {{accessToken}}
Content-Type: application/json

{"force_admin_takeover": true, "auth_method": "oidc"}
```

Success **200** — save `session.id` as `operator_session_id` for planogram draft/publish.

### Machine topology (required before planogram)

```http
PUT /v1/admin/machines/{{machineId}}/topology

{
  "operator_session_id": "<session uuid>",
  "cabinets": [{"code": "A", "title": "Cabinet A", "sortOrder": 1}],
  "layouts": [{
    "cabinetCode": "A",
    "layoutKey": "grid-4x6",
    "revision": 1,
    "layoutSpec": {"rows": 4, "cols": 6},
    "status": "published"
  }]
}
```

### Planogram publish

```http
PUT /v1/admin/machines/{{machineId}}/planograms/draft
POST /v1/admin/machines/{{machineId}}/planograms/publish

{
  "operator_session_id": "<session uuid from admin start>",
  "planogramId": "<new uuid v7>",
  "planogramRevision": 1,
  "syncLegacyReadModel": false,
  "items": [{
    "cabinetCode": "A",
    "slotCode": "A1",
    "legacySlotIndex": 1,
    "productId": "{{productId}}",
    "maxQuantity": 12,
    "priceMinor": 15000
  }]
}
```

Requires **ACTIVE** operator session on the machine. In production use **admin** `POST /v1/admin/machines/{machineId}/operator-sessions/start` (not legacy machine REST login).

### Sale catalog (gRPC — production)

REST is disabled when `MachineRESTLegacyEnabled=false`. Use **Machine JWT** metadata on the **machine gRPC public host** (`GRPC_PUBLIC_BASE_URL`, e.g. `grpcs://machine-api.<domain>:443`). The HTTP edge at `api.ldtv.dev` does **not** proxy gRPC — Caddy must expose a separate vhost (see `deployments/prod/examples/caddy-machine-grpc.Caddyfile.sample`).

**Obtain `MACHINE_ACCESS_TOKEN` (runtime only — never commit):**

1. Admin login — `POST /v1/auth/login`
2. Create code — `POST /v1/admin/machines/{machineId}/activation-codes` (admin JWT)
3. Claim — `POST /v1/setup/activation-codes/claim` (public; returns `machineToken`)
   gRPC equivalent: `MachineAuthService/ClaimActivation` or `MachineActivationService/ClaimActivation`
4. Optional refresh — gRPC `MachineTokenService/RefreshMachineToken` with `refresh_token`

```powershell
$env:MACHINE_ACCESS_TOKEN = "<from claim response machineToken>"
$env:GRPC_TARGET = "machine-api.ldtv.dev:443"   # must resolve in DNS + Caddy h2c → api:9090
```

```bash
grpcurl \
  -import-path proto \
  -proto proto/avf/machine/v1/catalog.proto \
  -proto proto/avf/machine/v1/common.proto \
  -H "authorization: Bearer ${MACHINE_ACCESS_TOKEN}" \
  -d '{"machine_id":"<machineId>","include_images":true}' \
  "${GRPC_TARGET}" \
  avf.machine.v1.MachineCatalogService/SyncSaleCatalog
```

`GetSaleCatalog` / `GetCatalogSnapshot` are aliases (same request/response). Expected shape:

```json
{
  "snapshot": {
    "machineId": "<uuid>",
    "catalogVersion": "runtime_sale_catalog_v4:...",
    "configVersion": 1,
    "currency": "USD",
    "items": [{
      "slotCode": "A1",
      "cabinetCode": "A",
      "productId": "<uuid>",
      "sku": "COCA-ACTIVE-...",
      "priceMinor": 15000,
      "primaryMedia": { "displayUrl": "https://res.cloudinary.com/...", "thumbUrl": "..." }
    }]
  }
}
```

## Common Errors

| Error | Cause | Fix |
|-------|--------|-----|
| `invalid_json` on product create | Bad Postman templating / unknown fields | Use `{{_runtimeProductCreateBody}}` from pre-request |
| `primaryMediaId requires company context` | Fixed in deploy `8d41859` — scope resolves `MEDIA_COMPANY_ID` | Ensure `MEDIA_COMPANY_ID` is set in production API env |
| `invalid_image_file` on upload | Manual Content-Type on multipart | Remove Content-Type header; pick real image file |
| `operator: session not found` on planogram | Missing/invalid `operator_session_id` | Call `POST /v1/admin/machines/{machineId}/operator-sessions/start` first |
| `404 page not found` on legacy operator login / sale-catalog | `MachineRESTLegacyEnabled=false` | Admin operator start + gRPC `MachineCatalogService.SyncSaleCatalog` |
| GATED-WRITE blocked | Safety flags | Set `allowGatedWrites` + `confirmProductionWrites` |

## Latest Production Result

**Run:** 2026-05-21 UTC · deploy main `be0f43b44db96a9af8077447e57356797316b191` · deploy run [26249178789](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26249178789) · API `https://api.ldtv.dev`

| Step | Status |
|------|--------|
| Health live/ready/version | **PASS** |
| Login | **PASS** |
| Image upload (Cloudinary + `MEDIA_COMPANY_ID`) | **PASS** (201) |
| Product create (active + primaryMediaId) | **PASS** (200) |
| Price book + items + assign-target | **PASS** |
| Admin operator session start | **PASS** (200) |
| Machine topology PUT | **PASS** (204) |
| Planogram draft/publish | **PASS** (204 / 200) — `syncLegacyReadModel=false`; MQTT `machine_planogram_publish` dispatched |
| Sale catalog REST | **BLOCKED** — `404` (`MachineRESTLegacyEnabled=false`) |
| gRPC SyncSaleCatalog | **BLOCKED** — machine gRPC not reachable from public internet (see below) |
| gRPC GetSaleCatalog | **BLOCKED** — same endpoint blocker |

### IDs from last run (sanitized)

| ID | Value |
|----|--------|
| primaryMediaId | `019e4c17-d4aa-7c3e-b047-855575ae63c4` |
| productId (active) | `019e4c17-de65-71c4-b0ab-3013619b2e8c` |
| sku | `COCA-ACTIVE-1779393091510` |
| machineId | `55555555-5555-5555-5555-555555555555` |
| operatorSessionId | `019e4c16-12b0-7a0f-8f9e-70b9b3ad4f9f` |
| planogramId | `a13beea1-c3d3-4d81-a589-ddb5b484d3b8` |
| commandId (planogram publish) | `019e4c17-ff9c-70a6-bff4-9e8163928cc7` |
| priceBookId | `019e4c17-e6e3-7a2c-9f55-0a84fe3ff866` |

### MQTT command evidence

Planogram publish returned `command.commandId` with `dispatchState=published` and `command_type=machine_planogram_publish` (indirect — broker ACL not verified in this run).

## Machine gRPC Catalog Verification

**Run:** 2026-05-21 UTC · same deploy as above · branch `test/production-machine-catalog-grpc`

| Check | Result |
|-------|--------|
| `MACHINE_ACCESS_TOKEN` obtained | **yes** — admin activation code + `POST /v1/setup/activation-codes/claim` |
| Token source | `admin_activation_code_claim` (REST; not legacy machine REST) |
| Token expiry | ~24h from claim (sanitized) |
| gRPC target tried | `machine-api.ldtv.dev:443`, `api.ldtv.dev:443`, `api.ldtv.dev:9090` |
| SyncSaleCatalog | **BLOCKED** — endpoint unreachable |
| GetSaleCatalog | **BLOCKED** — endpoint unreachable |
| Catalog contains productId | **not verified** (RPC blocked) |
| Catalog contains price | **not verified** |
| Catalog contains slot | **not verified** |
| Catalog contains image URL / mediaId | **not verified** |

### Blocker detail

Production API reports `MACHINE_GRPC_ENABLED` wiring (`GRPC_ADDR=:9090`, `GRPC_BEHIND_TLS_PROXY=true`) but **no public machine gRPC edge** is reachable from this runner:

- `machine-api.ldtv.dev` — **DNS NXDOMAIN** (no A/AAAA record)
- `api.ldtv.dev:443` + gRPC — **404 / Unimplemented** (Caddy `deployments/prod/shared/Caddyfile` proxies HTTP only to `:8080`, not h2c to `:9090`)
- `api.ldtv.dev:9090` — **connection timeout** (port not exposed)

**Fix (ops, not app code):** Add DNS + Caddy vhost per `deployments/prod/examples/caddy-machine-grpc.Caddyfile.sample`, set `GRPC_PUBLIC_BASE_URL=grpcs://machine-api.ldtv.dev:443`, re-run `SyncSaleCatalog`.

### Code sources inspected

| Area | Files |
|------|--------|
| Catalog RPC | `proto/avf/machine/v1/catalog.proto`, `internal/grpcserver/machine_catalog_grpc.go` |
| Machine auth | `proto/avf/machine/v1/auth.proto`, `proto/avf/machine/v1/machine_activation.proto`, `internal/httpserver/activation_http.go` |
| Token claim | `POST /v1/setup/activation-codes/claim`, `POST /v1/admin/machines/{id}/activation-codes` |
| Catalog build | `internal/app/salecatalog/service.go` — reads published planogram slots (no machine ACK required for snapshot) |

### MQTT / command apply (prior run)

Planogram publish returned `commandId=019e4c17-ff9c-70a6-bff4-9e8163928cc7`, `dispatchState=published`, `machine_planogram_publish`. Server-side sale catalog projection uses **published planogram** (`bootstrap.CurrentCabinetSlots`), not machine ACK — once gRPC is reachable, product should appear without device ACK.

### Canary IDs checked (when gRPC unblocked)

| Field | Value |
|-------|--------|
| productId | `019e4c17-de65-71c4-b0ab-3013619b2e8c` |
| primaryMediaId | `019e4c17-d4aa-7c3e-b047-855575ae63c4` |
| sku prefix | `COCA-ACTIVE` |
| machineId | `55555555-5555-5555-5555-555555555555` |
| expected priceMinor | `15000` |
| expected slot | `A1` / cabinet `A` |

## Postman fixes applied

- Product create body via `_runtimeProductCreateBody` + strict field validation
- Gated-write runtime idempotency headers on all `[GATED-WRITE]` requests
- Image upload multipart without manual Content-Type
- `unitPriceMinor=15000`; machine list saves `machineId` field
- Environment placeholders (no secrets committed)

Validate: `python postman/production-full-suite/validate_product_flow_suite.py`
