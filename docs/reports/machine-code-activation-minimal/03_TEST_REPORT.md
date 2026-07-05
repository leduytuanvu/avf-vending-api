# Machine-code activation — test report

Date: 2026-07-06

## Commands run

```text
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate   # OK (Makefile pins v1.31.1; generate succeeded)
python tools/build_openapi.py                               # OK
python tools/publish_production_full_json.py                # OK (461 requests)
go test ./internal/app/activation ./internal/httpserver ./internal/app/machineruntime ./internal/modules/postgres -count=1 -short   # OK
go test ./... -short -count=1                               # OK
go test ./internal/httpserver -run TestOpenAPI -count=1     # OK
```

## Skipped (environment)

| Test class | Reason |
|------------|--------|
| DB integration (`machineref_integration_test.go`, `service_machine_code_integration_test.go`, `activation_admin_http_test.go` DB cases, attachment tests) | `TEST_DATABASE_URL` not set in CI shell; skipped via `-short` or env guard |
| Production e2e Postman run | No production credentials |
| MQTT/hardware claim e2e | Out of scope |

## Unit tests (no DB)

- `machineref_test.go`: AVF format table, empty ref, invalid format, body validation
- `activation_admin_http_test.go`: route registration (`MountV1_adminMachineCodeActivationRoutesRegistered`)

## Expected with `TEST_DATABASE_URL`

- Resolve by code / UUID / unknown code / body conflict
- Create/list include `machineCode`; list JSON excludes plaintext and `codeHash`
- HTTP create by machineCode path and UUID path
- HTTP invalid identifier, not found, catalog conflict
- Board replacement preserves `machine.id` + `machines.code` + `ClaimResult.MachineCode`

## Not run

- `make sqlc-check` / `make swagger-check` / `make postman-check-json` (make unavailable on Windows host; equivalent commands run manually)
- Full non-`-short` integration suite without database
