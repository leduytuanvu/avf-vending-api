# P0.6 local end-to-end / storm tests

Deterministic integration tests for machine runtime, offline queue, payment webhooks, MQTT commands, inventory, and auth/RBAC live under:

- `internal/e2e/correctness/` — Postgres-backed HTTP + domain scenarios (`TestP06_E2E_*`)
- `internal/grpcserver/` — Machine gRPC + offline sync (`TestP06_*`, `TestMachineReplayLedger_*`, `TestMachineOfflineSync_*`)
- `internal/platform/auth/` — Admin REST guardrails (`TestP06_Auth_*`)
- `internal/app/background/` — Reconciliation tick invariants (`TestP06_Reconciler_*`)

## Prerequisites

1. **PostgreSQL** reachable from your machine (schema migrated). The repo uses [goose](https://github.com/pressly/goose); tests run `goose up` automatically when `TEST_DATABASE_URL` is set.
2. **Go** (same version as CI / `go.mod`).

## Windows (Git Bash) + Docker Desktop

1. Start Docker Desktop.
2. Run Postgres, for example:

   ```bash
   docker run --name avf-p06-pg -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=avf -p 5432:5432 -d postgres:16
   ```

3. In **Git Bash** (or any shell):

   ```bash
   export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/avf?sslmode=disable'
   cd <path-to-avf-vending-api-checkout>
   ./scripts/test-local/run-e2e-local.sh
   ```

   Or:

   ```bash
   make test-e2e-local
   ```

## CI-safe subset

Default CI runs `make test-short` (`go test ./... -short`), which **skips** DB-dependent tests. To exercise the full P0.6 slice in CI, use a job with Postgres service env and `make test-e2e-local` (or the same `go test` line as in the `Makefile`).

## Makefile target

`make test-e2e-local` runs:

```text
go test -count=1 -timeout=45m \
  ./internal/e2e/correctness/... \
  ./internal/grpcserver \
  ./internal/platform/auth \
  ./internal/app/background \
  -run 'TestP06_|TestMachineReplayLedger_|TestMachineOfflineSync_'
```

## Notes

- Tests are written to be **deterministic** (fixed seeds via `internal/testfixtures`, isolated UUIDs where needed, and cleanup on shared dev machine rows in offline/inventory storm cases).
- Some scenarios intentionally mutate the **dev seed machine** (`5555…`); those tests register `t.Cleanup` to restore `machine_slot_state` and offline ledger rows.
