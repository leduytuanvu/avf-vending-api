# P0 hardening report (pilot production)

**Date:** 2026-04-24 (session). **Scope:** route/OpenAPI alignment, production wiring validation, doc de-stale, enterprise verify gates.

## P0 routes mounted (Chi)

Verified in `internal/httpserver/server.go` and `mount*` helpers:

- **Activation:** `mountPublicActivationClaim`, `mountAdminActivationRoutes` (`activation_http.go`).
- **Sale catalog:** `mountSaleCatalogRoute` (`sale_catalog_http.go`).
- **Telemetry reconcile:** `mountDeviceTelemetryReconcileRoutes` (`telemetry_reconcile_http.go`).
- **Commerce cancel/refunds:** `mountCommerceRoutes` (`commerce_http.go`).
- **Cash settlement:** `mountAdminCashSettlementRoutes` (`admin_cash_http.go`).
- **Catalog CRUD + images:** `mountAdminCatalogRoutes` (`admin_catalog_http.go`).
- **Admin isolation:** `/v1/admin/*` and `/v1/reports/*` use `auth.RequireDenyMachinePrincipal` before role gates.

## P0 Swagger paths

- `tools/openapi_verify_release.py` enforces the full REST route-doc registry (`tools/build_openapi.py` **`REQUIRED_OPERATIONS`**)—every listed operation must exist under `paths` with the correct HTTP method.
- `servers[0]` = `https://api.ldtv.dev`, `servers[1]` = `http://localhost:8080`.
- Mirror guard: `TestOpenAPI_embeddedJSON_requiredP0PathsPresent` in `internal/httpserver/openapi_spec_test.go` (keep lists in sync).

## Production dependency wiring

- **`httpserver.ValidateP0HTTPApplication`** (`p0_validate.go`) — requires non-nil `Auth`, `CatalogAdmin`, `InventoryAdmin`, `Commerce`, `Activation`, `TelemetryStore`, `MachineShadow`, `RemoteCommands` when `APP_ENV=production`.
- Invoked from **`internal/bootstrap/api.go`** immediately after `api.NewHTTPApplication` and before `httpserver.NewHTTPServer`.

## Tests added/updated

- `internal/httpserver/p0_validate_test.go` — production validation expectations.
- `internal/httpserver/openapi_spec_test.go` — embedded OpenAPI P0 path matrix.
- `internal/platform/auth/middleware_machine_test.go` — **fixed** to use `httptest.NewRecorder` (client `http.DefaultClient` does not propagate request context to the handler).

## Stale docs

- Handoff files and `docs/README.md` updated to **shipped** status; `mqtt-contract.md` reconcile section corrected.
- `openapi-enterprise-upgrade-handoff.md` reframed as optional UX backlog.
- `enterprise-api-backend-audit-report.md` marked **historical snapshot**; section G relabeled.
- **`scripts/check_stale_p0_docs.sh`** — fails on forbidden phrases (excludes `roadmap.md` and the historical audit report).

## Commands (run on Linux CI or Git Bash)

| Command | Expected |
| --- | --- |
| `go test ./...` | pass |
| `make swagger` | pass |
| `make swagger-check` | pass |
| `make verify-enterprise-release` | pass (needs `bash`, `docker`, `python3`) |

## Related

- [Production release readiness](./production-release-readiness.md)
- [API surface audit](../api/api-surface-audit.md)
