# Machine-code activation — implementation report

Date: 2026-07-06

## SQL / sqlc

| Change | File |
|--------|------|
| Added `GetMachineByCode` | `db/queries/fleet.sql` |
| List queries join `machines.code AS machine_code` | `db/queries/activation.sql` |
| Generated queries | `internal/gen/db/fleet.sql.go`, `internal/gen/db/activation.sql.go` |

## Resolver

| File | Purpose |
|------|---------|
| `internal/app/activation/machineref.go` | `ResolveMachineRef`, `ResolveMachineBody`, typed errors, `^AVF[0-9]{6}$` validation |
| `internal/app/activation/machineref_test.go` | Unit tests (format, empty, invalid) |
| `internal/app/activation/machineref_integration_test.go` | DB: known code, unknown code, body conflict |

## Service DTO

| Field | Types |
|-------|-------|
| `MachineCode` | `CreateResult`, `ListRow`, `ClaimResult` |
| Enrichment | Create loads machine after insert; list maps `machine_code` from joined sqlc rows |

## HTTP routes

### Updated (path ref = UUID or machineCode)

- `POST/GET/DELETE /v1/admin/machines/{machineId}/activation-codes`
- `DELETE` includes `{activationCodeId}`

### New canonical machineCode paths

- `POST/GET /v1/admin/machine-codes/{machineCode}/activation-codes`
- `DELETE /v1/admin/machine-codes/{machineCode}/activation-codes/{activationCodeId}`

### Catalog

- `POST /v1/admin/activation-codes` body: `machineCode` / `machine_code` added; conflict → `machine_identifier_conflict`

### Response JSON

Create/list/catalog items now include `machineCode`. Claim response includes `machineCode` when non-empty.

### Error codes (HTTP)

| Error | Code | Status |
|-------|------|--------|
| Empty ref/body | `machine_identifier_required` | 400 |
| Bad format | `invalid_machine_identifier` | 400 |
| Not found | `machine_not_found` | 404 |
| Id/code mismatch | `machine_identifier_conflict` | 400 |

## OpenAPI / docs / Postman

| Artifact | Change |
|----------|--------|
| `internal/httpserver/swagger_operations.go` | Updated machine routes + 3 new machineCode ops + catalog body docs |
| `docs/swagger/swagger.json`, `docs.go` | Regenerated |
| `internal/httpserver/openapi_spec_test.go` | Added machine-codes paths to P0 required ops |
| `docs/runbooks/machine-activation.md` | machineCode routes + curl examples |
| `postman/production/avf-production-e2e.postman_collection.json` | REST-MACHINE-003 uses machineCode path; captures machineId |
| `postman/suites/production-full/...` | Catalog create body uses `{{machineId}}` / `{{machineCode}}` |
| `postman/environments/avf-production-full.postman_environment.json` | `machineCode=AVF000001` placeholder |

## Tests added/extended

| File | Coverage |
|------|----------|
| `service_machine_code_integration_test.go` | create/list/catalog machineCode; no hash/plaintext in JSON |
| `service_attachment_integration_test.go` | `machines.code=AVF000001`; board replacement keeps id+code |
| `activation_admin_http_test.go` | HTTP paths, errors, list safety, catalog body |
