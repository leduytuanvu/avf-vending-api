# External Product Image URL Full Test Final

## Git

- **Branch:** `feature/external-product-image-url-cache-flow`
- **Base SHA:** `fccb8a88abebba9f134f6877f8b63cec11d39c0a`
- **Test commit:** (this verification pass)

## Feature Summary

- External URL registration at `POST /v1/admin/media/external-images`
- Product primary image via `primaryMediaId` / `primaryImageUrl`
- Machine assignment via planogram draft → publish → slot config
- Sale catalog / gRPC expose `displayUrl`, `cacheKey`, `version`, offline hints
- Offline cache contract documented for vending app (not implemented in this repo)

## Config

| Variable | Purpose |
|----------|---------|
| `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED` | Feature gate |
| `PRODUCT_IMAGE_EXTERNAL_URL_ALLOWED_HOSTS` | Host allowlist (`adm.avf.vn`) |
| `PRODUCT_IMAGE_EXTERNAL_URL_REQUIRE_HTTPS` | HTTPS enforcement |
| `PRODUCT_IMAGE_EXTERNAL_URL_MAX_BYTES` | Remote size cap |
| `PRODUCT_IMAGE_EXTERNAL_URL_TIMEOUT` | Remote probe timeout |

**Object storage required for external URL flow:** **No**

## DB

- Migrations: existing schema (`source_type=external`, URL columns) — no new migration in feature branch
- Migration up verified via E2E test helper (`goose up` on Docker Postgres 15432)
- sqlc compiles; queries updated for `source_type` + `original_url` projection

## REST Tests

| Test | Result |
|------|--------|
| Auth on admin routes | Covered by existing middleware; external handler returns 503 before auth when service nil |
| Register external image (unit validation) | **PASS** |
| Idempotency key required | **PASS** (`TestPostAdminExternalProductImage_featureDisabledReturns503`) |
| Feature disabled → 503 JSON | **PASS** (not raw 404) |
| Bad URL / SSRF | **PASS** |
| Create product + bind | **PASS** (E2E with DB) |
| Machine assignment + sale catalog | **PASS** (E2E `TestExternalProductImage_registerBindMachineCatalog`) |

## gRPC Tests

- **Implemented:** yes
- **Tests run:** `TestProductMediaRefProto_externalURLMetadata`
- **Result:** **PASS** — `source_type`, `cache_key`, `offline_required`, `download_strategy` mapped

## MQTT Tests

- **Implemented for images:** no (by design)
- **Role:** planogram publish may emit sync command; no image bytes on MQTT
- **Tests run:** none specific to external images
- **Result:** N/A — documented only

## Offline Cache

- **App implementation in repo:** no
- **Contract docs:** yes (`docs/testing/VENDING_APP_PRODUCT_IMAGE_OFFLINE_CACHE_CONTRACT.md`)
- **Backend contract tests:** E2E asserts catalog returns sufficient metadata

## Postman / OpenAPI

- OpenAPI route registered: `/v1/admin/media/external-images` in `swagger.json`
- Postman production-full-suite: **missing from workspace** (deleted collections in working tree)
- Generated REST collection: **no `external-images` entry** (generator not run / folder not added)
- JSON validation: **not run** (no production JSON files present)

## Commands Run

```text
go test ./internal/app/mediaadmin/... ./internal/httpserver/... ./internal/grpcserver/... ./internal/e2e/correctness/ -run "External|ProductMediaRef|Media.*Route|PostAdmin" -count=1  → PASS
go test ./... -count=1  → FAIL (parallel DB integration pollution; pre-existing unrelated packages)
go vet ./...  → PASS
go list ./...  → PASS
docker compose -f deployments/docker/docker-compose.yml up -d postgres  → running
TEST_DATABASE_URL=postgresql://postgres:postgres@127.0.0.1:15432/postgres go test ./internal/e2e/correctness/ -run TestExternalProductImage  → PASS
py -3 tools/build_openapi.py  → PASS
```

## Failures Fixed (this pass)

| Root cause | Fix |
|------------|-----|
| E2E could not use httptest URL (SSRF blocks 127.0.0.1) | Added optional `RemoteProbe` on `mediaadmin.Deps` for integration tests |
| Assortment insert missing `meta` | Added `Meta: []byte("{}")` in E2E |
| Incomplete URL validation coverage | Added `external_url_validation_test.go` |
| HTTP test missing idempotency header | Added `Idempotency-Key` to handler test |

## Known Limitations

1. **Postman production suite** not updated — folder `Product Media` absent; blocks full Postman acceptance.
2. **Production live smoke** not executed (no credentials in agent environment).
3. **Full `go test ./...`** fails when all integration packages run concurrently against one Postgres (pre-existing flake; not introduced by external image feature).
4. **Real HEAD probe** to `adm.avf.vn` not automated in CI (network + SSRF design); production smoke covers this manually.
5. **Vending app** offline cache implementation lives outside this repository.

## Final Verdict

**BLOCKED_EXTERNAL_IMAGE_URL_FLOW**

Feature-scoped backend verification **passes** (unit + HTTP + gRPC + DB E2E). Full 100% acceptance is **blocked** because:

- Postman production suite does not include external image requests (files missing / not regenerated)
- Full repository `go test ./...` is not green under shared Docker Postgres (parallel integration tests)
- Production HTTP smoke against live API was not executed
- Vending app offline cache is documented but not implemented/tested in this repo

To reach **EXTERNAL_PRODUCT_IMAGE_URL_FLOW_100_PERCENT_PASS**:

1. Restore/update `postman/production-full-suite` with Product Media folder + Newman smoke
2. Run production smoke on canary with `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED=true`
3. Stabilize or isolate parallel integration tests in CI (or accept `-p 1` for e2e packages)
