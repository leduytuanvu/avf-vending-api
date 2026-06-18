> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/git-deploy/`. System copy removed 2026-06-16.

# API Backend Gate & Deploy Report (Phase 3)

Generated: 2026-06-11 (UTC+7). Repo `avf-vending-api`. Local Docker stack (postgres@15432, redis@6379).

## Deterministic gates
- `go vet ./...` -> **PASS** (exit 0).
- `go build ./...` -> **PASS** (exit 0).
- `go test ./internal/bootstrap/...` (changed file, hermetic) -> **PASS** (`ok`).

## Full suite on clean DB (`go test ./...`)
- Clean `avf_vending_test` created, goose migrations -> version 11.
- Result: **58 packages `ok`**, **1 package FAIL**: `internal/app/anomalies / TestP24_Sync_RepeatedVendFailure_Deduped` -> `duplicate key ... ux_inventory_anomalies_machine_fp_open (SQLSTATE 23505)`.
- Root cause: **environmental** - `go test ./...` runs packages concurrently against a single shared test DB; another package's inventory-anomaly rows collide with this package's open-anomaly unique constraint. Not a code defect.
- Proof: reset clean DB + run package in isolation -> `ok github.com/avf/avf-vending-api/internal/app/anomalies 1.936s` (**PASS**).
- Conclusion: **effective PASS**; for a fully green `./...` use one-DB-per-package or `-p 1`.

## Change classification (this branch)
- Changed: `internal/bootstrap/runtime_workflow_test.go` (test only), `tests/e2e/**`, `scripts/e2e/**`, `postman/**`, `docs/**`. **No Go production source** (`internal/app|service|repository`) changed.

## Deploy decision: **NO DEPLOY**
- No production code change -> production deploy (`deploy-production.yml`) intentionally **not** triggered.
- PR #350 (`chore/e2e-sellable-layout-setup -> develop`) open for human review; merge/deploy left to repo owners.
