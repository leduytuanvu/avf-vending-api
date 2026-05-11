# Next Production Proof Plan

Generated: 2026-05-11T15:08:00Z

## Current Evidence Baseline

- Branch: `test/production-full-destructive-e2e`
- Commit: `d9c60e3c8c29a3032c4cf2c133a0e7c9bae250f9`
- Local full E2E: passed, `23` passed, `0` failed, `0` skipped.
- Latest E2E evidence: `.e2e-runs/run-20260511T143948Z-409-30449`
- Phase8-42: passed, not skipped.
- `go vet ./...`: passed.
- `go test ./... -count=1`: passed.
- REST OpenAPI inventory: `365` operations, `99` scripted, `266` partial, `6` live probe OK.
- gRPC suite: passed.
- MQTT suite: passed.

## Remaining Blockers

### 1. Race Test Proof

Current blocker: Windows host cannot run `go test -race` because race builds require cgo and `gcc` is missing.

Local proof command when Linux/WSL/dev-container with gcc is available:

```bash
go version
go env GOOS GOARCH CGO_ENABLED
gcc --version
CGO_ENABLED=1 go test -race ./... -count=1
```

Fallback proof path: add/use an Ubuntu CI workflow that installs `gcc` and runs:

```bash
go test ./... -count=1
CGO_ENABLED=1 go test -race ./... -count=1
```

Do not mark race testing as passed until an actual local Linux/WSL/dev-container or CI run passes.

### 2. Contract and Makefile Gates

Current blocker: `make` was not available in the previous Windows shell.

Local proof commands when `make` is available:

```bash
make -n test-short
make -n api-contract-check
make test-short
make api-contract-check
```

Fallback proof path: wire these targets into the Ubuntu production-proof workflow and require a passing CI run.

### 3. EMQX Docker Healthcheck

Current blocker: MQTT tests passed, but Docker reports `avf-emqx` unhealthy.

Investigation commands:

```bash
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
docker inspect avf-emqx --format '{{json .State.Health}}'
docker logs --tail=200 avf-emqx
docker exec avf-emqx sh -lc 'emqx_ctl status || true'
docker exec avf-emqx sh -lc 'wget -qO- http://127.0.0.1:18083/status || true'
docker exec avf-emqx sh -lc 'nc -z 127.0.0.1 1883; echo mqtt_port=$?'
```

Acceptance: healthcheck becomes healthy, or reports state the exact non-blocking reason with MQTT proof evidence.

### 4. Read-only Production/Staging Smoke

Current blocker: no safe production/staging URL was configured.

Proof command when a safe URL is provided:

```bash
BASE_URL="$STAGING_BASE_URL" bash scripts/test/run-readonly-smoke.sh
BASE_URL="$PRODUCTION_BASE_URL" bash scripts/test/run-readonly-smoke.sh
```

The script must only call `GET /health/live`, `GET /health/ready`, `GET /version`, plus explicitly listed read-only paths in `SMOKE_READONLY_PATHS`.

### 5. Critical REST Live Coverage

Current blocker: REST per-operation live probing remains partial.

Proof artifacts to generate:

- `reports/test/rest-critical-live-coverage.json`
- `reports/test/rest-critical-live-coverage.md`

Acceptance: critical P0/P1 API groups have live evidence, while remaining OpenAPI operations are still reported as partial/blocked unless real request/response evidence exists.

## Final Reporting

After proof work, regenerate:

- `reports/test/PRODUCTION_PROOF_REPORT.md`
- `reports/test/PRODUCTION_PROOF_REPORT.json`
- `reports/test/production-readiness.md`
- `reports/test/FINAL_BACKEND_TEST_REPORT.md`
- `reports/test/FINAL_BACKEND_TEST_REPORT.json`

Final claim must be exactly one of:

- `PASS: Production-readiness proof passed for all executable local/CI/read-only gates; destructive production/hardware/provider flows remain intentionally unexecuted.`
- `FAIL: One or more executable production-readiness gates failed.`
- `BLOCKED: Production-readiness proof cannot be completed because required environment/tooling/URL/provider/hardware is missing.`
