# Machine-Code Activation — Implementation Report

Date: 2026-07-06

## Summary

Closed spec gaps from audit: introduced `MachineIdentityRef` struct and expanded HTTP/integration test coverage. No route, SQL, JWT, MQTT, or proto changes.

---

## Changes

### `internal/app/activation/machineref.go`

- Added `MachineIdentityRef { MachineID, MachineCode }`
- `ResolveMachineRef` and `ResolveMachineBody` now return `MachineIdentityRef` instead of `(uuid.UUID, string, error)`
- Service wrapper methods updated accordingly
- Regex unchanged: `^AVF[0-9]{6}$`

### `internal/httpserver/activation_http.go`

- `resolveAdminMachineRef` returns `activation.MachineIdentityRef`
- Create/list/revoke handlers use `identity.MachineID`
- Catalog create uses `identity.MachineID` from `ResolveMachineBody`

### Tests added/updated

| Test | File |
|------|------|
| `TestAdminCreateActivationCode_byMachineCodeInMachinePath` | `activation_admin_http_test.go` |
| `TestAdminDeleteActivationCode_byMachineCodePath` | `activation_admin_http_test.go` |
| `TestAdminCatalogCreateActivationCode_bodyMachineCodeSnake` | `activation_admin_http_test.go` |
| `TestAdminCatalogCreateActivationCode_bodyMachineIDSnake` | `activation_admin_http_test.go` |
| `TestResolveMachineBody_integration_byCodeSnake` | `machineref_integration_test.go` |
| All resolver tests updated for struct return | `machineref_test.go`, `machineref_integration_test.go` |

---

## Unchanged (by design)

- JWT `machine_id` claim
- MQTT topic identity (UUID)
- DB foreign keys
- gRPC runtime machineId requirement
- Activation SQL queries
- OpenAPI route definitions

---

## Local unit test result (short mode)

```
go test ./internal/app/activation/... ./internal/httpserver/... -short
ok  internal/app/activation
ok  internal/httpserver
```

DB-gated integration tests require `TEST_DATABASE_URL` (see `03_LOCAL_TEST_REPORT.md`).
