# Unit-level Go tests — report

**Scope:** `internal/domain/...`, `internal/app/...`, `internal/platform/...`, `internal/observability/...`  
**Flags:** `-count=1` (no cache reuse for pass/fail semantics)  
**Date:** produced by automated run on the developer workstation.

## Summary

| Group | Result |
|-------|--------|
| `./internal/domain/...` | **PASS** (exit `0`) |
| `./internal/app/...` | **PASS** (exit `0`) |
| `./internal/platform/...` | **PASS** (exit `0`) |
| `./internal/observability/...` | **PASS** (exit `0`) |

**Failing packages before fix:** **none** — all groups passed on first run; **no code or test edits** were required for this pass.

**Policy scan (`*_test.go` only):** searched under the four roots for substrings `organization`, `organization_id`, `organizationId`, `OrganizationID`, `org_admin`, `tenant` — **no matches**.

---

## Commands executed

```powershell
Set-Location <repo-root>

go test ./internal/domain/... -count=1
go test ./internal/app/... -count=1
go test ./internal/platform/... -count=1
go test ./internal/observability/... -count=1
```

---

## Results (abbreviated)

### `./internal/domain/...`

- Packages with tests: `compliance`, `operator` — **ok**
- Other packages: `[no test files]`

### `./internal/app/...`

- All packages with `_test.go` files reported **ok** (activation, anomalies, api, artifacts, audit, background, catalogadmin, commerce, device, featureflags, fleetadmin, inventoryadmin, inventoryapp, machineidempotency, machineruntime, mediaadmin, operator, otaadmin, outbox, payments, planogram, pricingengine, reliability, reporting, retention, rollout, salecatalog, setupapp, telemetryapp, workfloworch, plus background metrics subpackages).

### `./internal/platform/...`

- Tested packages **ok**: auth, clickhouse, db, mqtt, nats, objectstore, productionmetrics, payments, ratelimit, redis, telemetry.

### `./internal/observability/...`

- Tested packages **ok**: observability, mqttprom.

---

## Root causes

**N/A** — no failures.

---

## Files changed

**None** (tests green without modifications).

---

## Re-run (verification)

To reproduce:

```powershell
go test ./internal/domain/... ./internal/app/... ./internal/platform/... ./internal/observability/... -count=1
```

Expected: exit code **0**.

---

## Notes

- Integration-style tests under these trees that require `TEST_DATABASE_URL` or live deps still ran successfully in this environment (packages that need DB spun infra where required).
- Broader suites (e.g. `./internal/httpserver/...`, `./internal/modules/...`, `./internal/e2e/...`) were **out of scope** for this ticket.
