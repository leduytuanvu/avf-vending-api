# Postman assets

## Canonical CI collections & environments (tracked)

| Path | Description |
|------|-------------|
| [`collections/avf-vending-api.postman_collection.json`](collections/avf-vending-api.postman_collection.json) | Primary REST collection (production inventory baseline) |
| [`collections/avf-vending-api-function-path.postman_collection.json`](collections/avf-vending-api-function-path.postman_collection.json) | Function-path variant for Newman/E2E |
| [`environments/avf-local.postman_environment.json`](environments/avf-local.postman_environment.json) | Local dev |
| [`environments/avf-staging.postman_environment.json`](environments/avf-staging.postman_environment.json) | Staging / pre-prod |
| [`environments/avf-production.postman_environment.json`](environments/avf-production.postman_environment.json) | Production (mutations locked) |

**Regenerate:** `make postman-generate` (runs after `make swagger`)  
**Drift gate:** `make postman-check` — diffs only `postman/collections/` and `postman/environments/`

## Scripts (tracked)

| Path | Role |
|------|------|
| [`scripts/collection_prerequest.js`](scripts/collection_prerequest.js) | Collection pre-request script |
| [`scripts/collection_test.js`](scripts/collection_test.js) | Collection test script |

Shell wrappers: `scripts/postman/generate_collection.sh`, `scripts/postman/check_artifacts.sh`

## Generated / optional inventories (not CI drift-gated)

`postman/generated/` — expanded REST/gRPC/MQTT inventories from `scripts/postman/generate_complete_api_suite.py`. Regenerate locally; directory is **gitignored** (see root `.gitignore`).

## Production E2E Postman (CI parity)

[`production/`](production/) — collection + environment generated from `tests/e2e/production/e2e-manifest.yaml`. Required by `scripts/ci/verify_production_postman_parity.sh`.

## Production full suite (tracked)

[`suites/production-full/`](suites/production-full/) — consolidated OpenAPI + proto + MQTT production verification collection and environment (`avf-vending-production.full.*`). Not the same as manifest E2E parity under `production/`; use for broad production API coverage and manual/Newman runs.

Regenerate with `python scripts/postman/generate_production_full_suite.py` (writes `postman/suites/production-full/avf-vending-production.full.*`). Legacy output path `postman/production-full-suite/` is gitignored.

## Reports

[`reports/`](reports/) — intentionally committed Newman/audit outputs only. Local Newman CLI output is gitignored.

## OpenAPI source

Postman collections are generated from route inventory + OpenAPI; live spec: [`../docs/swagger/swagger.json`](../docs/swagger/swagger.json).

Operator guide: [`docs/postman/README.md`](../docs/postman/README.md) and [`docs/runbooks/postman.md`](../docs/runbooks/postman.md).
