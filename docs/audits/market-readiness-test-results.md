# Market readiness — test results

**Branch:** `qa/market-readiness-full-flow-validation`  
**Date:** 2026-05-27  
**Environment:** Windows dev host (no `make`/`bash` in PATH; Go/Python available)

## Commands and outcomes

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `gofmt -l .` (empty) | 0 | **PASS** |
| 2 | `go vet ./...` | 0 | **PASS** |
| 3 | `go test ./...` | 0 | **PASS** |
| 4 | `python tools/openapi_verify_release.py` | 0 | **PASS** |
| 5 | `python tools/generate_market_readiness_inventory.py` | 0 | **PASS** (REST=329, gRPC=80, MQTT=16) |
| 6 | `python postman/suites/full-production-suite/generate_full_postman_suite.py` | 0 | **PASS** (after REST_EXPECTED→329) |
| 7 | `python postman/suites/full-production-suite/validate_generated_assets.py` | 0 | **PASS** (initially **FAIL** at 327 expected) |
| 8 | `python tools/check_postman_artifacts.py` | 0 | **PASS** |
| 9 | `curl.exe https://api.ldtv.dev/health/live` | 0 | **PASS** 200 |
| 10 | `curl.exe https://api.ldtv.dev/health/ready` | 0 | **PASS** 200 |
| 11 | `bash scripts/check_migrations.sh` | — | **SKIP** (bash/WSL unavailable) |
| 12 | `make test-e2e-local` | — | **SKIP** (requires TEST_DATABASE_URL + make) |
| 13 | `scripts/production/smoke-market-readiness.sh` | — | **SKIP** locally (bash); health verified via curl |

## Fix applied during gate

- Updated `REST_EXPECTED` from **327** → **329** in `generate_full_postman_suite.py` and `validate_generated_assets.py` to match current `docs/swagger/swagger.json` (no coverage removed).

## Not run (blocked / needs credentials)

- Production admin smoke with `ACCESS_TOKEN`
- Machine gRPC/MQTT canary
- Destructive production E2E
- CI (pending push)
- Deploy post-verify

## Stop rule

Validator failure at step 7 halted forward progress until REST count fixed; re-run **PASS**.
