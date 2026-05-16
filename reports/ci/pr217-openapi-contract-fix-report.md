# PR #217 — OpenAPI contract fix (report)

## Root cause

`tools/build_openapi.py` **`REQUIRED_OPERATIONS`** still enforced **`/v1/admin/companies/{scopeId}/...`** method/path pairs after organization-style scoped URLs were removed from the runtime Chi surface. **`openapi_verify_release.py`** correctly failed: OpenAPI built from **`swagger_operations.go`** did not expose those paths.

## Fix summary

1. **`REQUIRED_OPERATIONS` / `IDEMPOTENCY_OPS`**: Dropped scoped **`.../companies/{scopeId}/...`** entries; aligned **`REQUIRED_OPERATIONS`** with the **325** operations emitted into **`docs/swagger/swagger.json`** (commerce, JSON reports, operations health, commands, anomalies, provisioning, rollouts, activation catalog path, machine lifecycle aliases, etc.).
2. **`internal/httpserver/swagger_operations.go`**: Added/adjusted **`DocOp*`** stubs (`@Router`) for flat admin paths and fixed rollout/revoke **`@Success`** responses required by the release verifier.
3. **`internal/httpserver/server.go`**: Wired **`mountAdminCompanyScopedActivationRoutes`** on the live **`/v1/admin`** router so **`GET/POST /v1/admin/activation-codes`** and revoke match handlers.
4. **`internal/httpserver/admin_fleet_write_http.go`**: Registered **`GET /assignments/{assignmentId}`**, **`POST /assignments`**, **`DELETE /assignments/{assignmentId}`** (mirror of technician-assignment routes previously only reachable via dead **`mountAdminCompanyFleetRoutes`**).
5. **Regenerated**: `py -3 tools/build_openapi.py`, `py -3 tools/build_postman_collection.py`.

Scoped **`/v1/admin/companies/{scopeId}`** URLs were **not** reintroduced into **`swagger.json`** / **`docs.go`**.

## Commands and results (local)

| Command | Result |
|--------|--------|
| `py -3 tools/build_openapi.py` | Success |
| `py -3 tools/openapi_verify_release.py` | Success — **`REST route-doc registry complete (325 operations)`** |
| `py -3 tools/check_postman_artifacts.py` | **`OK: Postman artifact checks`** |
| `go test ./... -short` | Pass |
| `go test ./...` | Pass (Windows session) |
| `go vet ./...` | Pass (prior full run in session) |
| `CGO_ENABLED=1 go test -race ./internal/httpserver` | **Not run**: `gcc` not on `%PATH%` (race requires CGO C compiler on this host) |

CI should still run **`make api-contract-check`** / **`go test -race ./...`** on Linux builders where **`gcc`** is available.

## Warnings / follow-ups

- **`operation_examples()`** in **`tools/build_openapi.py`** still includes legacy **`companies/{scopeId}`** keys (harmless for verifier; optional cleanup).
- Duplicate **`GET /v1/admin/commands`** registration may exist (**`mountAdminOperationsRoutes`** vs inline handler in **`server.go`**); behavior unchanged in this fix — worth deduping separately.

## Confirmation

- **`docs/swagger/swagger.json`** contains **no** `/v1/admin/companies/{scopeId}` paths (grep verified).
- **`internal/httpserver/swagger_operations.go`** contains **no** `/v1/admin/companies/{scopeId}` **`@Router`** lines.
- Old scoped company routes were **not** reintroduced for compatibility.
