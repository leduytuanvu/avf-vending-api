# OpenAPI / Swagger enterprise upgrade — implementation handoff

**Status:** **Superseded for core P0 routing.** Activation, sale catalog, refunds/cancel, cash settlement, device reconcile, and catalog CRUD are **mounted** in `internal/httpserver/server.go` and documented in `docs/swagger/swagger.json`. `make swagger-check` and `tools/openapi_verify_release.py` enforce production-first servers, Bearer on protected `/v1` routes, P0 path presence, and no planned-only paths.

This document remains a **backlog** for optional Swagger UX polish (extra examples, tags, named error aliases) that is not required for pilot HTTP correctness.

---

## 0. Scope honesty

OpenAPI **must** list only paths that exist on the Chi router (`server.go` and `mount*` helpers). Planned-only capabilities belong in [roadmap.md](../api/roadmap.md) and **must not** appear under `paths` until implemented (`tools/openapi_verify_release.py` + `go test` OpenAPI guards).

---

## 1. `tools/build_openapi.py`

### 1.1 `info` / servers

After loading `gen` in `main()`, append to `info["description"]`:

```text

**Default server:** Production (`https://api.ldtv.dev`). Use the Servers dropdown in Swagger UI for local development.

**JSON errors:** `{"error":{"code","message","details","requestId"}}` (camelCase `requestId`). Plain text may be used for `/health/ready` 503.
```

Keep `servers[0]` production; assertion already present.

### 1.2 Error components

- Keep `V1APIErrorEnvelope` / `V1StandardError` aligned with `internal/apierr` and `openapi_types.go`.
- Add **aliases** (same `$ref` or duplicate schema) with different `title` / `description` for discoverability:
  - `ErrorResponse`, `ValidationErrorResponse`, `ConflictErrorResponse`, `UnauthorizedErrorResponse`, `ForbiddenErrorResponse`, `NotFoundErrorResponse`, `RateLimitErrorResponse`, `InternalErrorResponse`, `ServiceUnavailableErrorResponse`
- Add `components.examples` with named samples, e.g. `Unauthenticated`, `ValidationFailed`, `ConflictState`, `RateLimited`.

Helper:

```python
def error_example(code: str, message: str, details: dict | None = None, request_id: str = "01ARZ3NDEKTSV4RRFFQ69G5FAV") -> dict:
    return {"error": {"code": code, "message": message, "details": details or {}, "requestId": request_id}}
```

### 1.3 Global parameters

- Extend **`IdempotencyKeyHeader`** description:

  > Use a **stable client-generated** key for retries on this operation. Reusing the same key with the **same** payload returns `replay: true` when supported. Reusing the same key with a **different** payload may return **409** (`idempotency_key_conflict` or domain-specific conflict).

- Add example value: `order-3fa85f64-5717-4562-b3fc-2c963f66afa6-local-000123`

- Optional: `ContentTypeJSON` parameter (`application/json`) for POST-heavy ops — or document in operation description only.

### 1.4 Public vs protected `security`

After building `paths`, run `postprocess_security(paths)`:

| Condition | `op["security"]` |
| --- | --- |
| `GET /health/*`, `/metrics`, `/swagger/*`, `/version` | `[]` |
| `POST .../payments/{paymentId}/webhooks` | `[]` (HMAC only) |
| `POST /v1/auth/login`, `POST /v1/auth/refresh` | `[]` |
| Other `/v1/*` with Bearer in current spec | `[{"bearerAuth": []}]` |

Ensure webhook **does not** get `Idempotency-Key` from `IDEMPOTENCY_OPS` — **remove** webhook entry from `IDEMPOTENCY_OPS`.

### 1.5 Tag taxonomy

Post-process `op["tags"]` from path/method, e.g.:

- **System** — health, metrics, swagger, version  
- **Auth** — `/v1/auth/*`  
- **Machine Setup** — `/v1/setup/*`, admin topology/planogram/sync  
- **Operator Sessions** — operator-sessions, operator-insights  
- **Catalog Admin** — admin products, price-books, planograms (read)  
- **Inventory** — slots, inventory, inventory-events, stock-adjustments  
- **Machine Admin** — admin machines list, technicians, assignments, commands, ota  
- **Commerce** — `/v1/commerce/*`, `/v1/orders`, `/v1/payments`  
- **Device Runtime** — `/v1/device/*`, `commands/dispatch` (optional: split Remote Commands)  
- **Machine Telemetry** — shadow, telemetry snapshot/incidents/rollups  
- **Machine Runtime** — check-ins, config-applies  
- **Reporting** — `/v1/reports/*`  
- **Artifacts** — admin artifacts  

### 1.6 Request body: named `examples` (Swagger UI)

Extend `attach_examples`:

- For `application/json` request bodies, support `requestBodyExamples: dict[str, Any]` in the bag from `operation_examples()`.
- Emit `content["application/json"]["examples"] = { name: {"summary": name, "value": obj} }`.
- Keep top-level `example` for default (first named or primary).

Add **CashSaleVND** for `POST /v1/commerce/cash-checkout` (currency `VND`, `total_minor` realistic) and **CreateOrderWallet** for `POST /v1/commerce/orders`.

### 1.7 Fill gaps in `operation_examples()`

Add request + response examples for paths currently missing from the dict, including:

- `POST /v1/auth/login`, `refresh`, `logout` (204 empty for logout)  
- `GET /v1/auth/me`  
- `POST /v1/admin/machines/{machineId}/sync`  
- `POST /v1/machines/{machineId}/check-ins`, `config-applies`  
- `POST .../operator-sessions/logout`, `heartbeat`  
- All four `/v1/reports/*` (query `from`, `to`, `organization_id`)  
- `GET /v1/commerce/orders/{orderId}/reconciliation`  
- `POST` webhook (body + success `replay`)  
- Admin catalog GETs (minimal list envelope)  
- `GET /v1/machines/{machineId}/commands/receipts`  

### 1.8 Standard responses

Optional `enrich_responses(op, path, method)` to add **503** `auth_misconfigured` for bearer ops if missing — only where realistic.

Do **not** add 304 unless ETag is implemented.

### 1.9 `annotate_transport` (optional)

Keep `x-avf-client-classification: device_http_fallback` on `commands/poll`.

---

## 2. `internal/httpserver/swagger_operations.go`

Add:

```go
// DocOpVersion godoc
// @Summary Build metadata
// @Description Returns process version JSON (no authentication). Same family as `/health/*`.
// @Tags System
// @Produce json
// @Success 200 {object} object "{\"version\":\"...\",\"build\":{...}}"
// @Router /version [get]
func DocOpVersion() {}
```

Add `("get", "/version")` to `REQUIRED_OPERATIONS` in `build_openapi.py`.

Optional: expand `@Failure` lines on high-traffic DocOps where still thin — generator already maps `V1StandardError`.

---

## 3. `internal/httpserver/swagger_openapi_quality_test.go` (new)

- `go test` loads `docs/swagger/swagger.json` (embed or `os.ReadFile` relative to module root).  
- Assert: valid JSON, `openapi == 3.0.3`, `servers[0].url == https://api.ldtv.dev`.  
- Assert: `components.securitySchemes.bearerAuth` exists.  
- For each path under `/v1/` except `/v1/auth/login`, `/v1/auth/refresh`, webhook POST: operation has `security` with `bearerAuth` **or** explicit `[]` for known public.  
- Spot-check: every `post`/`put` in a required list has `requestBody` with `example` or `examples`.  
- Assert: webhook POST has no `Idempotency-Key` in parameters (after postprocess).

---

## 4. Regenerate

```bash
make swagger
make swagger-check
go test ./internal/httpserver ./internal/config
go test ./...
```

---

## 5. Final report (after implementation)

Fill in:

| Item | Count / note |
| --- | --- |
| Endpoints improved | +`/version`; enriched examples on N paths |
| Schemas added | Error aliases + `components.examples` |
| Remaining undocumented | Roadmap routes not in Chi |
| Commands | pass/fail |

---

## 6. Related docs

- [swagger-openapi-appendix.md](../api/swagger-openapi-appendix.md)  
- [enterprise-api-backend-audit-report.md](enterprise-api-backend-audit-report.md) (honest API completeness)
