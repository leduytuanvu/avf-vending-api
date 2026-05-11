# FINAL BACKEND TEST REPORT

## Executive summary

- **Overall status:** BLOCKED: Production-readiness proof cannot be completed because no safe staging/production smoke URL is configured.
- **Commit SHA:** `2482821c7222bb008dd1ceb913b3c64c602cadfc`
- **UTC time:** `2026-05-11T17:55:00Z`
- **Tools used:** Go test suite, gofmt, go vet, Python coverage generators (`scripts/test/*`), OpenAPI (`docs/swagger/swagger.json`), proto corpus (`proto/`), MQTT contract doc.
- **REST operations (OpenAPI):** 365 total; probe HTTP 200-ish: **6**; blocked rows: **0**
- **gRPC methods enumerated:** 85
- **MQTT flows enumerated:** 5
- **Business flows mapped:** 10

## Commands run

| Label | Command | cwd | Exit | ms |
|---|---|---:|---:|---:|
| go_test_audit | `go test ./...` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 171 |
| gofmt_audit | `gofmt -l "." (must be empty)` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 1 |
| go_vet_audit | `go vet ./...` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 7 |
| rest_inventory | `python scripts/test/rest_openapi_coverage.py --base-url http://127.0.0.1:18080` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 0 |
| grpc_inventory | `python scripts/test/grpc_inventory_coverage.py` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 0 |
| mqtt_inventory | `python scripts/test/mqtt_inventory_coverage.py` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 0 |
| e2e_flow_inventory | `python scripts/test/e2e_flow_inventory.py` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 0 |
| chk_api | `python scripts/test/check-api-coverage.py` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 0 |
| chk_flow | `python scripts/test/check-flow-coverage.py` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 0 |
| report_gen | `python scripts/test/generate-test-report.py` | D:/admin/development/avf/avf-vending-system/avf-vending-api | 0 | 1 |

## Bugs fixed during audit

- `internal/grpcserver/machine_grpc_auth_test.go` — gofmt drift corrected (static gate parity with CI `fmt-check`).

## Remaining gaps (representative)

- **Local E2E executed:** `tests/e2e/run-all-local.sh --fresh-data` passed with 23 passed, 0 failed, 0 skipped. Evidence: `reports/test/e2e-evidence/run-all-local-last.log` and `.e2e-runs/run-20260511T143948Z-409-30449`.
- **Postgres correctness:** `go test ./... -count=1` passed after running against the local test setup; the final E2E database `avf_vending_test_final` was recreated and migrated from scratch before harness execution.
- **Makefile / buf / sqlc contract gate** passed in GitHub Actions Ubuntu production proof run `25683990468`.
- **Live payment PSP** signatures remain sandbox-only; mock paths documented under `internal/e2e/correctness/payment_webhook*_test.go` and `tests/e2e/scenarios/42_e2e_qr_payment_success_mock.sh`.

## Final full verification addendum

- **Branch:** `test/production-full-destructive-e2e`
- **Commit:** `28ca1200d4458da256c1d79569b86d6a492f3d22`
- **Clean DB:** `avf_vending_test_final`
- **API:** `http://127.0.0.1:18080`
- **gRPC:** `127.0.0.1:9090`
- **MQTT:** `127.0.0.1:1883`
- **Latest E2E run:** `.e2e-runs/run-20260511T143948Z-409-30449`
- **Full E2E:** `run-all-local.sh --fresh-data` exit 0, 23 passed, 0 failed, 0 skipped
- **Phase8-42:** pass, not skipped; signed local webhook and replay path succeeded
- **Static checks:** `gofmt`, `go vet`, shell syntax, Python compile, and `actionlint` passed
- **Go tests:** `go test ./... -count=1` passed
- **Race tests:** passed in GitHub Actions Ubuntu production proof run `25683990468`
- **Production proof workflow:** `https://github.com/leduytuanvu/avf-vending-api/actions/runs/25683990468`, conclusion success
- **Makefile gates:** `make test-short` and `make api-contract-check` passed in the production proof workflow
- **Go vulnerability scan:** passed in Security workflow run `25686906495` after moving toolchain references to Go `1.25.10` and `golang.org/x/net v0.53.0`
- **Local govulncheck:** passed with explicit `go1.25.10` and `govulncheck@v1.3.0`
- **REST OpenAPI coverage:** 365 operations; 99 scripted; 266 partial; 6 live probe OK responses
- **REST critical live coverage:** 30 critical checks; 27 live 2xx passes; 3 partial/non-2xx items; not full OpenAPI live coverage
- **gRPC coverage:** 85 methods enumerated; local gRPC suite passed
- **MQTT coverage:** 5 flows enumerated; local MQTT suite passed; EMQX local Docker health now reports healthy after listener-based healthcheck
- **Production smoke:** not run; no safe production/staging URL configured; read-only smoke script and NOT RUN artifacts added
- **Request/response evidence:** `reports/test/rest-api-requests-responses.jsonl`, `reports/test/api-request-response-report.jsonl`, `reports/test/e2e-evidence/run-all-local-last.log`
- **Production readiness:** see `reports/test/production-readiness.md`

REST per-operation live probing remains partial and should not be represented as 100% API live coverage.

## Production proof addendum

- **Proof workflow:** `.github/workflows/production-proof.yml` ran successfully on Ubuntu for `go test`, `CGO_ENABLED=1 go test -race`, `make test-short`, and `make api-contract-check` (`https://github.com/leduytuanvu/avf-vending-api/actions/runs/25683990468`).
- **EMQX:** local compose healthcheck changed from `emqx ctl status` to an in-container MQTT listener check because the broker served MQTT while the control CLI could not ping the node after long Windows Docker Desktop uptime. Docker health is now healthy and the MQTT suite passed.
- **Flaky test fix:** `TestMachineGRPC_Commerce_ExpiredCheckoutWindow_Blocked` now deterministically ages its order in Postgres instead of relying on an 80ms sleep.
- **Affected gates rerun:** `go vet ./...`, `go test ./internal/grpcserver -run TestMachineGRPC_Commerce_ExpiredCheckoutWindow_Blocked -count=1`, `go test ./internal/grpcserver -count=1`, `go test ./... -count=1`, script syntax/compile checks, compose config, MQTT suite, and REST critical coverage generation all passed.
- **Secret audit:** no unredacted local fixture values, JWT-shaped tokens, or Bearer-shaped secrets found in `reports`.
- **Security blocker:** resolved by Security workflow run `25686906495`; production readiness remains blocked only by missing read-only smoke URL.

## Final claim (exact)

> **BLOCKED: Production-readiness proof cannot be completed because no safe staging/production smoke URL is configured.**
