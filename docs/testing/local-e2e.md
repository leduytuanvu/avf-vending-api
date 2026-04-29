# Local end-to-end correctness tests (P0.6)

**vs field / P2 go-live:** Automated packages here prove **DB correctness** (idempotency, replay ordering, ledger rules). They **do not** satisfy **[`field-test-cases.md`](field-test-cases.md)** pilot rows (hardware, broker TLS, real PSP, operator sign-off) — treat as **dev/CI** gates; attach **[`operations/field-pilot-checklist.md`](../operations/field-pilot-checklist.md)** + **`field-test-cases`** matrix for production field evidence.

**Normative prod transport:** **[`../architecture/production-final-contract.md`](../architecture/production-final-contract.md)**.

These integration tests validate money-path and reliability behaviors against PostgreSQL: machine mutation idempotency, offline replay ordering, MQTT command ledger vs ACK rules, commerce vend inventory idempotency, payment webhook replay, reconciliation cases, and machine credential gates.

## Prerequisites

1. **PostgreSQL** reachable from your machine (Docker Compose stack under `deployments/docker/` is typical; see `docs/runbooks/local-dev.md`).
2. **`TEST_DATABASE_URL`** — connection string for an empty database or a disposable scratch DB (tests run **goose up** on each invocation).

Example:

```bash
export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/avf_test?sslmode=disable"
```

On Windows Git Bash or PowerShell, set the same variable before running Make.

## Commands

From the repo root:

```bash
make test-e2e-local
```

Equivalent:

```bash
go test -count=1 -timeout=45m ./internal/e2e/correctness/... ./internal/grpcserver \
  -run 'TestP06_|TestMachineReplayLedger_|TestMachineOfflineSync_'
```

Git Bash wrapper:

```bash
bash scripts/test-local/run-e2e-local.sh
```

Additional Postgres-heavy suites live under `./internal/modules/postgres/` (commerce, MQTT commands). Run full integration coverage when needed:

```bash
go test -count=1 -timeout=45m ./internal/modules/postgres/...
```

## CI vs local

- Default CI (`make test-short`) skips integration tests that require `TEST_DATABASE_URL` or use `-short`.
- Run **`make test-e2e-local`** locally or attach **`TEST_DATABASE_URL`** in a dedicated CI job when you need full correctness validation.

## Related tests

| Concern | Package / notes |
|--------|------------------|
| Admin JWT blocked on machine gRPC | `internal/grpcserver/unary_internal_user_auth_test.go` |
| Machine JWT blocked on Admin REST | `internal/platform/auth/middleware_machine_test.go` |
| Webhook HMAC | `internal/httpserver/commerce_webhook_hmac_test.go`, `commerce_webhook_public_policy_test.go` |
| Offline MQTT samples | `internal/platform/mqtt/offline_replay_contract_test.go` |
