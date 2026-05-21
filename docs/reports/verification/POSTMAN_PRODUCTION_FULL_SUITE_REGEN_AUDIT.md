# Postman Production Full Suite Regeneration Audit

Generated: 2026-05-21

## Source of truth

- `docs/swagger/swagger.json` (regenerated via `python tools/build_openapi.py`)
- `postman/suites/full-production-suite/generate_full_postman_suite.py` (gfs)
- `scripts/postman/folder_business.py` (domain routing)
- `scripts/postman/generate_production_full_suite.py` (production packager)

## Counts

| Layer | Count |
| --- | --- |
| OpenAPI REST operations | 328 |
| Generated Postman REST requests | 328 |
| gRPC methods (doc folders) | 86 |
| MQTT topics (doc folders) | 28 |

## Changes in this regen

- Added missing `POST /v1/admin/media/external-images` (external product image URL flow)
- Environment defaults set for immediate canary use:
  - `allowGatedWrites=true`
  - `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`
  - `allow_destructive=true`
  - `canaryMode=true`
  - `readiness=true`
- All REST requests use Postman URL objects and `Idempotency-Key: {{$guid}}`
- `[GATED-WRITE]` requests include `allowGatedWrites` + `confirmProductionWrites` guard scripts
- Media upload init documents **200** or **503 capability_not_configured** (not raw 404)
- Replaced `AVF_PRODUCTION_FULL_TESTING_GUIDE.md` with `TESTING_GUIDE.md`

## Missing / stale routes

- **Before:** 327 requests; missing `POST /v1/admin/media/external-images`
- **After:** 328 requests; full OpenAPI parity

## Feature-gated routes

- `POST /v1/admin/media/uploads/init` — 503 when object storage disabled
- `POST /v1/admin/media/external-images` — 503 when `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED` false

## Destructive / gated routes

All non-read-only writes prefixed `[GATED-WRITE]` with pre-request guards.

## Validator

`.tmp-validate-production-postman-suite.py` — **VALIDATION_PASS** (328 requests)
