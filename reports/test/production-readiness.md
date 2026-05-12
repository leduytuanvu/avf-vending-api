# Production Readiness Verification

Generated: 2026-05-12T02:55:00Z

## Environment

- Verification host: local Windows workstation with Docker Desktop Linux engine.
- API under test: `http://127.0.0.1:18080`
- gRPC under test: `127.0.0.1:9090`
- MQTT broker under test: `127.0.0.1:1883`
- Database: `avf_vending_test_final` on local Docker Postgres.
- Payment mode: local signed webhook mock with fixture HMAC secret; no real PSP/provider secrets used.

## Executed Local Readiness Evidence

- Clean database created, migrated from scratch through goose version 72, and seeded with only the deterministic local E2E admin account.
- API restarted from the current working tree and passed `/health/live` and `/health/ready`.
- Full local E2E harness passed with 23 passed, 0 failed, 0 skipped:
  `.e2e-runs/run-20260511T143948Z-409-30449`
- Targeted reruns passed for gRPC, MQTT, Phase8-42, critical REST sale/refund/offline/admin-inventory flows, and remote command ACK.
- Phase8-42 QR payment mock passed with signed local webhook and duplicate replay evidence.
- `go vet ./...` passed.
- `go test ./... -count=1` passed after fixing the deterministic assortment primary-binding race with the unique active-primary index.
- `go test -race ./... -count=1` is blocked on this Windows host, but passed in GitHub Actions Ubuntu production proof run `25683990468`.
- Shell syntax checks for `scripts/**/*.sh` and `tests/**/*.sh` passed.
- Python compile checks for `scripts`, `tools`, and `tests` passed.
- `actionlint` passed; `shellcheck`, `golangci-lint`, and `make` were not installed on this host.

## Coverage and Protocol Evidence

- REST OpenAPI inventory: 365 operations, 99 scripted, 266 partial.
- REST request/response evidence: `reports/test/rest-api-requests-responses.jsonl` and `reports/test/api-request-response-report.jsonl`.
- gRPC inventory: 85 methods enumerated; full local gRPC E2E suite passed.
- MQTT inventory: 5 flows enumerated; full local MQTT E2E suite passed with local broker evidence.
- Flow coverage gate: 10 flows mapped and `check-flow-coverage.py` passed.

## Production Smoke

Production/staging smoke was not run. No safe production or staging base URL was configured in:

- `PRODUCTION_BASE_URL`
- `PROD_BASE_URL`
- `STAGING_BASE_URL`
- `AVF_PRODUCTION_BASE_URL`
- `AVF_STAGING_BASE_URL`

No production writes, vends, refunds, command publishes, payment webhooks, or destructive endpoints were attempted.

## Security Release #99 Goose Blocker

Security Release #99 currently blocks production readiness because `Published Image Vulnerability Scan` failed only for the published goose image. The app image scan passed.

- Failed goose image: `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:27c4afb1723e3109ccf08be271b4276308ab66a82b66a699b1995259bf3b62dc`.
- Root cause: `/usr/local/bin/goose` embedded `go.opentelemetry.io/otel v1.40.0` from `pressly/goose v3.27.0`; Trivy reported `CVE-2026-29181` HIGH, fixed in OpenTelemetry `v1.41.0`.
- Fix in this branch: `deployments/prod/Dockerfile.goose` now builds goose from `pressly/goose v3.27.1`, which uses OpenTelemetry `v1.43.0`, and uses supported digest-pinned `alpine:3.23` for the goose runtime image.
- Local proof: rebuilt local goose image reports `goose version: v3.27.1`; local Trivy `0.57.1` scan passed with `Total: 0 (HIGH: 0, CRITICAL: 0)`.
- Local sanity: `go vet ./...`, `govulncheck ./...`, and `docker compose -f deployments/docker/docker-compose.yml config` passed. `go test ./... -count=1` failed locally in `internal/modules/postgres` at `TestOutboxRepository_LeaseOutboxForPublish_SetsPublishing`; this is outside the goose image build path and remains for CI confirmation.
- Release status before CI: **FAIL** until the updated goose image is built, published, and passes Security Release Trivy scanning.

## Operational Readiness Checklist

- DB migrations: forward goose migration path exercised locally from an empty database.
- Migration policy: production docs describe forward-only/image rollback behavior in `docs/runbooks/migration-safety.md` and production rollback runbooks.
- Idempotency: REST/gRPC/E2E suites include idempotency and duplicate/replay paths, including Phase8-42 webhook replay.
- Outbox: payment-session and webhook outbox paths exercised locally; report inventory includes outbox operability coverage.
- Payment webhook security: signed local HMAC webhook and duplicate replay passed; invalid webhook/auth cases covered by tests.
- MQTT command ACK: local MQTT suite and Phase8 remote command ACK passed.
- gRPC machine auth: gRPC suite passed with auth denial and valid machine-token paths.
- RBAC/auth denies: unit/integration tests and admin flows exercised local auth paths.
- Health/readiness: `/health/live` and `/health/ready` passed for the current API process.
- Logs: API logs captured in `reports/test/e2e-evidence/api-server.stdout.log` and `api-server.stderr.log`; no real secrets intentionally printed.
- Docker local dependencies: Postgres, Redis, NATS, EMQX, and MinIO were running locally. EMQX compose health status was reported unhealthy by Docker, but MQTT E2E passed against port 1883.
- Backup/restore and deployment docs: production runbooks exist, but no production backup/restore drill was executed in this local verification.

## Remaining Production Risks

- REST per-operation live probing remains partial and should not be represented as 100% API live coverage.
- Production smoke is not run without an explicitly configured safe production/staging URL.
- EMQX Docker health is resolved for the local compose stack by checking the in-container MQTT listener; production parity should still use the production broker health policy.
- Published goose image scan remains failed until Security Release rebuilds and scans the fixed goose image.

## Production Proof Addendum

Generated: 2026-05-12T02:55:00Z

- EMQX Docker health is resolved for local proof: the compose healthcheck now verifies the in-container MQTT listener on port 1883, Docker reports `avf-emqx` healthy, and `tests/e2e/run-mqtt-local.sh` passed after the change.
- Local affected gates passed: `go vet ./...`, `go test ./internal/grpcserver -run TestMachineGRPC_Commerce_ExpiredCheckoutWindow_Blocked -count=1`, `go test ./internal/grpcserver -count=1`, `go test ./... -count=1`, smoke script syntax, REST critical coverage Python compile, compose config, MQTT suite, and REST critical coverage generation.
- The checkout-window gRPC integration test no longer depends on sub-second sleep timing; it deterministically ages the test order before asserting `FailedPrecondition`.
- GitHub Actions production proof passed on Ubuntu: `https://github.com/leduytuanvu/avf-vending-api/actions/runs/25683990468`.
- CI-proven gates: `go test ./... -count=1`, `CGO_ENABLED=1 go test -race ./... -count=1`, `make test-short`, and `make api-contract-check`.
- Read-only staging/production smoke remains NOT RUN because no safe URL is configured; artifacts: `reports/test/readonly-smoke.json` and `reports/test/readonly-smoke.md`.
- Critical REST live coverage artifacts were generated without overclaiming full OpenAPI live coverage: `30` critical checks, `27` live 2xx passes, `3` partial/non-2xx items.
- Security scan blocker discovered: PR `203` Go Vulnerability Scan failed in run `25685173325` because CI used Go `1.25.9` and `golang.org/x/net v0.52.0`; this update moves the repository to Go `1.25.10` and `golang.org/x/net v0.53.0`.
- Local `govulncheck ./...` passed with explicit `go1.25.10`; updated CI Security workflow run `25686906495` also passed Go Vulnerability Scan.
- Final claim: **FAIL: Published goose image vulnerability scan failed until rebuilt image passes Trivy in Security Release.**
