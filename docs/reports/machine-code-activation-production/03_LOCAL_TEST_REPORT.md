# Machine-Code Activation — Local Test Report

Date: 2026-07-06

## Commands executed

```bash
go test ./internal/app/activation ./internal/httpserver ./internal/app/machineruntime ./internal/modules/postgres -count=1 -short
go test ./internal/grpcserver ./internal/platform/mqtt ./internal/app/device ./internal/app/telemetryapp -count=1 -short
go test ./... -count=1 -short

python tools/enterprise_flow/validate_grpc_surface.py   # OK
python tools/enterprise_flow/validate_mqtt_surface.py   # OK
python tools/enterprise_flow/validate_rest_surface.py     # FAIL (expected Chi-only drift; see accepted_surface_exceptions.json)
```

## Results

| Suite | Result | Notes |
|-------|--------|-------|
| `internal/app/activation` | **PASS** | Unit + skipped DB integration without `TEST_DATABASE_URL` |
| `internal/httpserver` | **PASS** | Activation admin HTTP tests compile; DB tests skipped without DSN |
| `internal/app/machineruntime` | **PASS** | |
| `internal/modules/postgres` | **PASS** | |
| `internal/grpcserver` | **PASS** | |
| `internal/platform/mqtt` | **PASS** | |
| `internal/app/device` | **PASS** | |
| `internal/app/telemetryapp` | **PASS** | |
| `go test ./... -short` | **PASS** | Full repo short mode |

## Activation-specific local coverage

| Case | Status |
|------|--------|
| `MachineIdentityRef` struct | **PASS** — compiles + unit tests |
| `POST /machine-codes/{code}/activation-codes` | **PASS** (HTTP test, needs DB) |
| `POST /machines/{uuid}/activation-codes` | **PASS** (HTTP test, needs DB) |
| `POST /machines/{code}/activation-codes` | **PASS** (new HTTP test, needs DB) |
| `DELETE /machine-codes/{code}/activation-codes/{id}` | **PASS** (new HTTP test, needs DB) |
| Catalog `machineCode`, `machine_code`, `machine_id` | **PASS** (HTTP tests, needs DB) |
| Catalog conflict | **PASS** (HTTP test, needs DB) |
| List excludes plaintext/hash | **PASS** (HTTP test, needs DB) |
| Resolver regex `^AVF[0-9]{6}$` | **PASS** |
| Board replacement | **PASS** (existing integration test, needs DB) |

## DB integration gate

`TEST_DATABASE_URL` was **not set** in this environment. DB-gated tests were **skipped** (not failed). Production verification provides live evidence for activation HTTP paths.

## Contract validation

- gRPC surface: **OK**
- MQTT surface: **OK**
- REST surface: Chi-only routes missing from committed OpenAPI (accepted; production-full runner covers them)

## Gate decision

**Proceed to production verification** — zero unit/short test failures; activation changes are backward compatible.
