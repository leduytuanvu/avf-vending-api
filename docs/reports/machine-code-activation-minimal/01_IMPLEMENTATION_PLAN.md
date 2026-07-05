# Machine-code activation — implementation plan (frozen)

Date: 2026-07-06

Scope: activation admin HTTP/SQL/DTO/tests/docs only. Runtime claim stays UUID-based. No MQTT/JWT/DB FK/proto refactors.

## Decisions

- Activation admin `machineCode` format: `^AVF[0-9]{6}$` (exactly six digits after `AVF`).
- Applied in `internal/app/activation/machineref.go` only (not fleet-wide create validation).
- Existing `/v1/admin/machines/{machineId}/activation-codes` routes accept UUID **or** machineCode in path segment.
- New canonical routes under `/v1/admin/machine-codes/{machineCode}/activation-codes`.
- Catalog create body accepts `machineId` / `machine_id` and/or `machineCode` / `machine_code`.
- Create/list responses include `machineCode` plus existing fields.

## Steps executed

1. Audit report (`00_CURRENT_STATE_AUDIT.md`).
2. SQL: `GetMachineByCode`; list queries join `machines.code AS machine_code`; `sqlc generate`.
3. Resolver: `machineref.go` + unit/integration tests.
4. Service DTO: `MachineCode` on `CreateResult` / `ListRow`; create/list enrichment.
5. HTTP: resolver wiring, new routes, catalog body, response JSON, optional claim `machineCode`.
6. OpenAPI swagger regen + `openapi_spec_test` paths; Postman/runbook updates.
7. Tests in `activation` and `httpserver` packages; board replacement assertion with `machines.code`.
8. Verification + reports (`02`–`04`).

## Out of scope

MQTT/EMQX ACL, JWT claim changes, proto renames, fleet CRUD refactors, full REST identity refactor, Postman v3 script fixup.
