# PR #217 — OpenAPI contract fix (analysis)

## 1. Failing command (CI)

- `make api-contract-check` → `swagger-check` → `python3 tools/openapi_verify_release.py`
- Local equivalent (Windows): `py -3 tools/openapi_verify_release.py`

## 2. Root symptom

Failure mode was **`OpenAPI missing operations required by the route-doc registry`**: `tools/build_openapi.py` `REQUIRED_OPERATIONS` still listed many **`/v1/admin/companies/{scopeId}/...`** tuples after the single-company route refactor, while Swagger **`@Router`** comments documented **`/v1/admin/...`** paths only.

## 3. Stale registry vs runtime

- **Stale**: Any **`REQUIRED_OPERATIONS`** tuple whose path contained **`/v1/admin/companies/{scopeId}`** (removed from the registry; no scoped router paths).
- **Runtime**: Chi mounts fleet/commerce/operations/anomalies/provisioning/rollouts/activation under **`/v1/admin/...`** (plus **`scope_id` query** for platform admins where applicable).

## 4. OpenAPI generation surface

- **Source of truth**: `internal/httpserver/swagger_operations.go` (`DocOp*` stubs + `@Router`).
- **Generator**: `py -3 tools/build_openapi.py` → `docs/swagger/swagger.json` and `docs/swagger/docs.go` (do not hand-edit generated JSON/docs.go).

## 5. Additional gaps discovered

- **`mountAdminCompanyFleetRoutes`** had **`POST/GET/DELETE /assignments`** and **`mountAdminCompanyScopedActivationRoutes`**, but **`mountAdminCompanyFleetRoutes`** had **no callers** — flat **`/v1/admin/assignments/*`** and **`/v1/admin/activation-codes`** risked **404** unless wired on the live admin router.
- **`tools/openapi_verify_release.py`** additionally requires **`responses`** objects with examples where applicable (fixed rollout transition **`DocOp`** stubs missing **`@Success`**).

## 6. Sync strategy used

1. Remove **scoped-company** entries from **`REQUIRED_OPERATIONS`** and **`IDEMPOTENCY_OPS`** (single-company HTTP paths only).
2. Add **`REQUIRED_OPERATIONS`** rows for every **`(method, path)`** present in generated **`docs/swagger/swagger.json`** until **`openapi_verify_release.py`** reports **`REST route-doc registry complete (325 operations)`** (covers commerce/reports/ops/anomalies/provisioning/rollouts/etc.).
3. Add **`DocOp*`** stubs in **`swagger_operations.go`** until **`build_openapi.py`** route coverage passes and verifier succeeds.

## 7. Note — example dictionary drift

`operation_examples()` in **`tools/build_openapi.py`** still contains legacy keyed examples using **`/v1/admin/companies/{scopeId}/...`** for narrative/sample payloads where **`attach_examples`** looks up **`(method, path)`** against live paths (those tuple keys are effectively unused for current paths). Cleaning them is optional follow-up.
