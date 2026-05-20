# Postman assets

## Canonical CI collections & environments

| Path | Description |
|------|-------------|
| [`collections/avf-vending-api.postman_collection.json`](collections/avf-vending-api.postman_collection.json) | Primary REST collection (production inventory baseline) |
| [`collections/avf-vending-api-function-path.postman_collection.json`](collections/avf-vending-api-function-path.postman_collection.json) | Function-path variant for Newman/E2E |
| [`environments/avf-local.postman_environment.json`](environments/avf-local.postman_environment.json) | Local dev |
| [`environments/avf-staging.postman_environment.json`](environments/avf-staging.postman_environment.json) | Staging / pre-prod |
| [`environments/avf-production.postman_environment.json`](environments/avf-production.postman_environment.json) | Production (mutations locked) |

Regenerate: `make postman-generate` or `scripts/postman/generate_collection.sh`  
Validate: `make postman-check` or `python tools/check_postman_artifacts.py`

## Scripts

| Path | Role |
|------|------|
| [`scripts/collection_prerequest.js`](scripts/collection_prerequest.js) | Collection pre-request script |
| [`scripts/collection_test.js`](scripts/collection_test.js) | Collection test script |

Shell wrappers: `scripts/postman/generate_collection.sh`, `scripts/postman/check_artifacts.sh`

## Production verification suite

[`suites/full-production-suite/`](suites/full-production-suite/) — full REST + gRPC + MQTT production inventory (325/325/85/28 counts). Separate from CI-native collections above.

## Reports

[`reports/`](reports/) — intentionally committed Newman/audit outputs only. Local Newman CLI output is gitignored (see root `.gitignore`).

## OpenAPI source

Postman collections are generated from route inventory + OpenAPI; live spec: [`../docs/swagger/swagger.json`](../docs/swagger/swagger.json) (also served at `/swagger/doc.json` when enabled).

**Moved:** former `docs/postman/*.json` → `postman/collections/` + `postman/environments/`. See [`docs/postman/README.md`](../docs/postman/README.md) for redirect.
