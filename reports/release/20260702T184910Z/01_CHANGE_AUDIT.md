# Change Audit — avf-vending-api

**Timestamp:** 20260702T184910Z

## What changed

Enterprise flow backend: lifecycle reason/audit, reattach-device, runtime-session admin APIs, ClaimContext on activation claim, ended_reason normalization, offline gRPC aliases, OpenAPI +9 payment/media routes, migration 00016, validators/tests/inventories.

## Commit IN

- `migrations/00016_enterprise_flow_accountability.sql`, `db/schema/`, `db/queries/`
- `internal/gen/db/*`, all production Go + tests listed in plan
- `docs/swagger/swagger.json`, `swagger_operations.go`, `tools/build_openapi.py`
- `tools/enterprise_flow/*`
- `postman/suites/production-full/*` (after validation)
- `reports/enterprise-flow-verification/20260703T013119Z/`
- `reports/enterprise-flow/20260703T011828Z/` (if untracked)
- `reports/release/20260702T184910Z/`

## Commit OUT

- `.env*`, secrets, IDE metadata
- Local scratch not matching sources

## Secrets

No live tokens in diff. Postman environment retains `{{variable}}` placeholders.

## Migrations

`00016` adds accountability columns to `machine_activation_claims` — must run on production during deploy (goose).

## Missing before commit

None identified; pre-commit validation required.
