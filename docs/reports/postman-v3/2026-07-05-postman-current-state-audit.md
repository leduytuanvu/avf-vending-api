# Postman current-state audit (2026-07-05)

## Root cause (Local Mode error)

Postman v12 Local Mode / Native Git requires **collection schema 3.0.0** (folder of `*.request.yaml` files). The repo previously tracked only **v2.1 JSON**, which triggers:

> Postman no longer supports JSON for collections and environments in Local Mode. Upgrade these files to v3 YAML format.

## Tracked JSON artifacts (canonical for CI / Newman)

| Path | Role | Requests |
|------|------|----------|
| `postman/collections/avf-vending-api.postman_collection.json` | Primary CI / smoke | 33 |
| `postman/collections/avf-vending-api-function-path.postman_collection.json` | E2E default | 33 |
| `postman/environments/avf-{local,staging,production}.postman_environment.json` | CI drift-gated envs | ~55 vars each |
| `postman/suites/production-full/avf-vending-production.full.*` | Full OpenAPI + gRPC/MQTT docs | 461 (460 HTTP + 1 doc README item) |
| `postman/production/avf-production-e2e.*` | Manifest E2E parity | 48 |

## Generators

| Script | Output |
|--------|--------|
| `tools/build_postman_collection.py` | `postman/collections/`, `postman/environments/` |
| `scripts/postman/generate_production_full_suite.py` | `postman/suites/production-full/` |
| `postman/production/generate_postman_from_manifest.py` | `postman/production/` |
| `scripts/postman/postman_openapi_lib.py` | Shared OpenAPI builder (restored from git history) |

## Validators / CI gates

| Gate | Scope |
|------|-------|
| `make postman-check-json` | JSON regen + `tools/check_postman_artifacts.py` + git diff `postman/collections/` `postman/environments/` |
| `make postman-check-v3` | `tools/check_postman_v3_artifacts.py` + `scripts/postman/validate_v3_yaml.py` + git diff `postman/v3/` |
| `make api-contract-check` | Includes `postman-check` (JSON + v3) |
| `scripts/ci/verify_production_postman_parity.sh` | `postman/production/*` only |

## Path drift (resolved)

| Canonical | Legacy (do not use) |
|-----------|---------------------|
| `postman/suites/production-full/` | `postman/suites/full-production-suite/` |
| `postman/v3/suites/production-full/` | `postman/production-full-suite/` (gitignored) |

## New v3 outputs (Postman Local Mode)

| Path | Source JSON |
|------|-------------|
| `postman/v3/collections/avf-vending-api/` | Primary collection |
| `postman/v3/collections/avf-vending-api-function-path/` | Function-path collection |
| `postman/v3/collections/avf-production-e2e/` | Production E2E |
| `postman/v3/suites/production-full/avf-vending-production-full/` | Production full suite |
| `postman/v3/environments/*.environment.yaml` | Matching JSON environments |
| `postman/v3/manifest.json` | Parity manifest |

## Files that must remain for CI / Newman

All existing JSON paths above — unchanged.
