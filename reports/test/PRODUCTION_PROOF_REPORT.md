# Production Proof Report

Generated: `2026-05-11T16:42:00Z`

## Final Claim

> **BLOCKED: Production-readiness proof cannot be completed because no safe staging/production smoke URL is configured.**

All executable local gates run during this proof pass completed successfully after one flaky timing test was made deterministic. Ubuntu CI proof now also passed for `go test`, `go test -race`, `make test-short`, and `make api-contract-check`. Full production-readiness proof remains blocked only because no safe staging/production URL is configured for read-only smoke.

## Proven Locally

- Full local E2E baseline remains the green run: `.e2e-runs/run-20260511T143948Z-409-30449` with `23` passed, `0` failed, `0` skipped.
- EMQX local Docker health is resolved by checking the in-container MQTT listener; Docker reports `avf-emqx` healthy and `tests/e2e/run-mqtt-local.sh` passed.
- `go vet ./...` passed.
- `go test ./... -count=1` passed.
- `internal/grpcserver` targeted checkout-window test and package test passed after removing sub-second sleep flakiness.
- `scripts/test/run-readonly-smoke.sh` passed `bash -n`.
- `scripts/test/rest_critical_live_coverage.py` passed Python compile and generated critical REST reports.
- `docker compose -f deployments/docker/docker-compose.yml --profile broker config` passed.
- Critical REST live coverage generated: `30` scoped critical checks, `27` live 2xx passes, `3` partial/non-2xx items. This is not 100% OpenAPI live coverage.
- Final secret audit found no unredacted local fixture values, JWT-shaped tokens, or Bearer-shaped secrets in `reports`.

## Proven In GitHub Actions

- PR: `https://github.com/leduytuanvu/avf-vending-api/pull/202`
- Workflow run: `https://github.com/leduytuanvu/avf-vending-api/actions/runs/25683385601`
- Run ID: `25683385601`
- Job: `Linux race and contract gates`
- Head SHA: `271019198c5bfefd57530d8437c16f8320d4f067`
- `go test ./... -count=1`: passed.
- `CGO_ENABLED=1 go test -race ./... -count=1`: passed.
- `make test-short`: passed.
- `make api-contract-check`: passed.

## Still Blocked

- Read-only staging/production smoke was not run because none of `STAGING_BASE_URL`, `PRODUCTION_BASE_URL`, `PROD_BASE_URL`, or `BASE_URL_PROD` is configured.

## Changed Proof Artifacts

- `.github/workflows/production-proof.yml`
- `deployments/docker/docker-compose.yml`
- `internal/grpcserver/machine_commerce_grpc_integration_test.go`
- `scripts/test/run-readonly-smoke.sh`
- `scripts/test/rest_critical_live_coverage.py`
- `reports/test/NEXT_PRODUCTION_PROOF_PLAN.md`
- `reports/test/readonly-smoke.json`
- `reports/test/readonly-smoke.md`
- `reports/test/rest-critical-live-coverage.json`
- `reports/test/rest-critical-live-coverage.md`

## Rerun Commands

```bash
go vet ./...
go test ./internal/grpcserver -run TestMachineGRPC_Commerce_ExpiredCheckoutWindow_Blocked -count=1
go test ./internal/grpcserver -count=1
go test ./... -count=1
bash -n scripts/test/run-readonly-smoke.sh
python -m py_compile scripts/test/rest_critical_live_coverage.py
docker compose -f deployments/docker/docker-compose.yml --profile broker config
bash tests/e2e/run-mqtt-local.sh
python scripts/test/rest_critical_live_coverage.py
```

Read-only smoke proof still required when a safe URL is available:

```bash
BASE_URL="$STAGING_BASE_URL" bash scripts/test/run-readonly-smoke.sh
BASE_URL="$PRODUCTION_BASE_URL" bash scripts/test/run-readonly-smoke.sh
```
