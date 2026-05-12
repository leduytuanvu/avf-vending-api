# Production Proof Report

Generated: `2026-05-12T02:55:00Z`

## Final Claim

> **FAIL: Published goose image vulnerability scan failed until rebuilt image passes Trivy in Security Release.**

Security Release #99 failed after the app image scan passed because the published goose image `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:27c4afb1723e3109ccf08be271b4276308ab66a82b66a699b1995259bf3b62dc` embedded vulnerable Go module metadata in `/usr/local/bin/goose`.

## Security Release #99 Goose Blocker

- Failed workflow: Security Release #99, `Published Image Vulnerability Scan`.
- Failed image: `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:27c4afb1723e3109ccf08be271b4276308ab66a82b66a699b1995259bf3b62dc`.
- App image status: pass.
- Goose OS scan: Alpine HIGH/CRITICAL count `0`.
- Goose binary finding: `/usr/local/bin/goose` embedded `go.opentelemetry.io/otel v1.40.0`, affected by `CVE-2026-29181` HIGH, fixed in `v1.41.0`.
- Root cause: `deployments/prod/Dockerfile.goose` cloned `pressly/goose v3.27.0`, whose module graph contains `go.opentelemetry.io/otel v1.40.0`.
- Fix in this branch: build goose from `pressly/goose v3.27.1`, which pulls `go.opentelemetry.io/otel v1.43.0`, and move the goose runtime base from unsupported `alpine:3.20` to digest-pinned `alpine:3.23`.
- Local proof: `docker build -f deployments/prod/Dockerfile.goose -t avf-vending-api-goose:local .` passed; `goose -version` reports `v3.27.1`; local Trivy `0.57.1` image scan passed with `Total: 0 (HIGH: 0, CRITICAL: 0)`.
- Local sanity: `go vet ./...`, `govulncheck ./...`, and `docker compose -f deployments/docker/docker-compose.yml config` passed. `go test ./... -count=1` failed locally in `internal/modules/postgres` at `TestOutboxRepository_LeaseOutboxForPublish_SetsPublishing`; this is outside the goose image build path and remains for CI confirmation.
- Release status before CI: still failed until Security Release rebuilds and scans the newly published goose image.

The earlier Go Vulnerability Scan blocker is resolved: the repository uses Go `1.25.10` and `golang.org/x/net v0.53.0`; Security, CI, and Production Proof workflows passed for that update on commit `2482821c7222bb008dd1ceb913b3c64c602cadfc`.

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
- Local `govulncheck ./...` passed with explicit `go1.25.10` on `PATH` after installing `golang.org/x/vuln/cmd/govulncheck@v1.3.0`.
- Local `go test ./... -count=1` did not complete cleanly on this Windows workstation due to integration setup failures/timeouts; CI remains the authoritative proof for the updated branch.

## Security Scan Blocker Discovered

- Failed CI run: `https://github.com/leduytuanvu/avf-vending-api/actions/runs/25685173325`
- Failed check: `Go Vulnerability Scan`
- Findings: Go standard library vulnerabilities fixed in Go `1.25.10`, plus `golang.org/x/net@v0.52.0` fixed in `v0.53.0`.
- Fix applied: `go.mod` toolchain and CI/Docker Go references moved from `1.25.9` to `1.25.10`; `golang.org/x/net` moved to `v0.53.0`.
- Updated Security workflow: `https://github.com/leduytuanvu/avf-vending-api/actions/runs/25686906495` passed, including `Go Vulnerability Scan`.
- Updated Production Proof workflow: `https://github.com/leduytuanvu/avf-vending-api/actions/runs/25686909181` passed, including `go test`, `go test -race`, `make test-short`, and `make api-contract-check`.

## Proven In GitHub Actions

- PR: `https://github.com/leduytuanvu/avf-vending-api/pull/203`
- Workflow run: `https://github.com/leduytuanvu/avf-vending-api/actions/runs/25683990468`
- Run ID: `25683990468`
- Job: `Linux race and contract gates`
- Head SHA: `28ca1200d4458da256c1d79569b86d6a492f3d22`
- `go test ./... -count=1`: passed.
- `CGO_ENABLED=1 go test -race ./... -count=1`: passed.
- `make test-short`: passed.
- `make api-contract-check`: passed.

## Still Blocked

- Read-only staging/production smoke was not run because none of `STAGING_BASE_URL`, `PRODUCTION_BASE_URL`, `PROD_BASE_URL`, or `BASE_URL_PROD` is configured.
- Full REST OpenAPI live coverage remains partial and must not be represented as 100% live API coverage.

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
