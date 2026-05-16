# Local full E2E report (post organization removal)

**Generated:** 2026-05-16 (after green run)

## Latest `.e2e-runs` directory

`D:/admin/development/avf/avf-vending-system/avf-vending-api/.e2e-runs/run-20260516T095457Z-725-10318`

Canonical summaries under that path:

- `reports/summary.md`
- `reports/remediation.md` (empty failures on this run)

## Command

```bash
bash tests/e2e/run-all-local.sh --fresh-data
```

**Harness exit code:** `0`

## Pass / fail / skip (top-level harness steps)

From the runner footer (`events.jsonl` rollup):

| Metric   | Count |
|----------|------:|
| Passed   |    23 |
| Failed   |     0 |
| Skipped  |     0 |

All major steps completed:

| Step | Result |
|------|--------|
| preflight | passed |
| rest-local-suite | passed |
| web-admin-flows | passed |
| vending-app-flows | passed |
| grpc-local-suite | passed |
| mqtt-local-suite | passed |
| phase8 E2E-40 … E2E-47 | passed |

## Organization-related env (verification)

Under `tests/e2e`, ripgrep finds **no** references to:

- `E2E_ORGANIZATION_ID`
- `organizationId`
- `organization_id`
- `canary_organization_id`

## Submodule skips (intentional / documented caveats)

The harness reports **0 skipped steps**, but individual protocol/module logs still record **optional or conditional skips** (these did not fail the run):

- **gRPC contract suite:** e.g. `GRPC-24/report-update-status` → `skip` with reason `no_assigned_update_in_get_assigned_response` (see `reports/summary.md` gRPC table).
- **MQTT contract suite:** several rows marked `skip` (e.g. `no_ADMIN_TOKEN`, `partial_no_per_event_mqtt_read_api_documented`) while broker connectivity and publish paths still **pass** — see MQTT table in `reports/summary.md`.
- **Web admin / vending REST modules:** `reports/summary.md` reports row-level skips inside `wa-module-results.jsonl` / `va-rest-results.jsonl` (optional endpoints or role gates); the **web-admin-flows** and **vending-app-flows** steps still exited **passed**.

These are **not** treated as harness failures; they are documented in the per-run `reports/summary.md` and JSONL artifacts.

## Root causes fixed (this session)

1. **`catalogadmin` + single-company scope:** `adminCatalogScopeID` always returns `uuid.Nil`. Guards that rejected `companyID == uuid.Nil` caused bogus `ErrCompanyRequired` (e.g. primary image load after product create). **Fix:** require only resource IDs where the SQL is single-tenant scoped (`internal/app/catalogadmin/service.go`, `pricebook.go`, `writes.go`).
2. **Broken catalog SQL after scope stripping:** Several `catalog_writes.sql` updates used `WHERE TRUE`, so statements targeted **all rows** (e.g. `CatalogWriteUpdateProduct`), producing duplicate SKU conflicts and corrupt data. **Fix:** add proper `WHERE` predicates by primary key / foreign key (`db/queries/catalog_writes.sql`, callers updated).
3. **Broken catalog admin reads:** `CatalogAdminGetBrand`, `GetCategory`, `GetTag`, `GetPriceBook`, list helpers for price-book items/targets, and `GetMachineSiteForOrg` used `WHERE TRUE`. **Fix:** filter by id / `price_book_id` / `machine_id` (`db/queries/catalog_admin.sql`, callers updated).
4. **Machine idempotency UPSERT:** `UpsertMachineIdempotencyKey` uses `ON CONFLICT (machine_id, operation, idempotency_key)` but the table lacked a matching unique index after consolidation. **Fix:** migration `00074_machine_idempotency_unique.sql` + `CREATE UNIQUE INDEX ux_machine_idempotency_machine_op_key` in `db/schema/01_platform.sql`.
5. **E2E harness bug:** `13_web_admin_support_ops.sh` referenced unset `path` under `set -u`. **Fix:** initialize `path="/v1/orders"` before optional `machine_id` query string (`tests/e2e/scenarios/13_web_admin_support_ops.sh`).

## Final status

**PASS** — full local E2E (`run-all-local.sh --fresh-data`) completed with harness exit code **0**, **23/23** steps passed, **0** failed harness steps.
