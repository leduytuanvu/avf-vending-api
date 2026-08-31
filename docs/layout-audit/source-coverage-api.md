# Source Coverage Ledger — API

**Audit status:** COMPLETE for layout/planogram scope (Phase 2 gate).

## Excluded categories

`.git`, `node_modules`, `vendor`, `bin/`, coverage/cache, binary artifacts.

## Ledger

| Path | Classification |
|------|----------------|
| `migrations/*.sql` | READ_FULL |
| `db/schema/01_platform.sql` | READ_RELEVANT_SECTIONS |
| `db/queries/planogram.sql`, `topology.sql`, `fleet_admin.sql`, `catalog_admin.sql`, `catalog_writes.sql`, `inventory_admin.sql`, `operator_domains.sql`, `machine_idempotency.sql`, `device.sql`, `device_config.sql` | READ_FULL / READ_RELEVANT_SECTIONS |
| `internal/app/planogram/**` | READ_FULL |
| `internal/app/setupapp/**` | READ_FULL |
| `internal/app/machineidempotency/**` | READ_RELEVANT_SECTIONS |
| `internal/app/machineruntime/**` | READ_RELEVANT_SECTIONS |
| `internal/httpserver/admin_inventory_http.go`, `admin_planogram_http.go`, `admin_catalog_http.go`, `machine_runtime_http.go` | READ_FULL |
| `internal/grpcserver/machine_grpc*.go` | READ_RELEVANT_SECTIONS |
| `internal/modules/postgres/setup_repository.go`, `machine_config_snapshot.go` | READ_FULL |
| `proto/avf/machine/v1/bootstrap.proto`, `common.proto` | READ_FULL |
| `internal/gen/db/**` | GENERATED_CONTRACT_VERIFIED |
| `docs/swagger/swagger.json` | GENERATED_CONTRACT_VERIFIED |
| `Makefile`, `.github/workflows/ci.yml` | READ_RELEVANT_SECTIONS |
