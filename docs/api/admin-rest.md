# Admin REST API

Operator and back-office integrations use **HTTPS + JSON** under `/v1` with **User JWT** and **RBAC** (`internal/platform/auth`, `internal/httpserver`). OpenAPI is the external contract ([`docs/swagger/swagger.json`](../../docs/swagger/swagger.json), `make swagger`).

## Transport boundary

- **Admin REST** is the supported surface for catalog, fleet, inventory admin, reporting, finance reads, feature flags, OTA admin, and similar workflows (`/v1/admin/*` and related routes — routing overview in `internal/httpserver/router.go`).
- **Machine principals** (`role=machine` / `typ=machine`) are **rejected** on administrative routes via `RequireDenyMachinePrincipal` (HTTP **`403`**).
- **Machine runtime** belongs on **native gRPC** (**`avf.machine.v1`**) with **Machine JWT** — see **[`machine-grpc.md`](machine-grpc.md)**. Do not treat Admin REST as the primary vending runtime API.

See **[`../architecture/transport-boundary.md`](../architecture/transport-boundary.md)** for Admin REST vs machine gRPC vs MQTT vs webhooks.

### P1 Admin Web security contract

| Layer | Enforcement |
| ----- | ----------- |
| Authentication | `Authorization: Bearer` user access JWT on `/v1/admin/*` (`internal/platform/auth` bearer middleware). Missing/invalid token → **401** `unauthenticated`. |
| Interactive + machine deny | `RequireInteractiveAccountActive`; **machine** principals (`RequireDenyMachinePrincipal`) → **403** on `/v1/admin`. |
| RBAC | Per-route `RequireAnyPermission` / `RequirePermission` / `RequireAnyRole` in `internal/httpserver/admin*_http.go`, `server.go` groups, and **`TestRBAC_adminMountSourcesDeclareAccessControl`**. Insufficient permission → **403** `forbidden`. |
| Tenant / org scope | Path or query `organization_id` must match the interactive principal’s org (or platform-admin query rules). Mismatches typically → **400** `invalid_scope` via **`adminCatalogOrganizationID`** and sibling helpers (`admin_scope.go`). |
| Audit | Security-sensitive mutations use **`internal/app/audit`** `RecordCritical` / `RecordCriticalTx` (actor, org, action, resource identifiers, request metadata where wired; never store secrets).

OpenAPI is **`docs/swagger/swagger.json`**; internal-only gRPC URLs must not appear as public HTTP paths (see `github.com/avf/avf-vending-api/internal/httpserver` OpenAPI tests).

---

## Auth flow

| Concern | Where |
| ------- | ----- |
| Login / refresh | **`POST /v1/auth/login`**, **`POST /v1/auth/refresh`** — JSON credentials / refresh token (no Bearer on these routes). |
| Bearer usage | Subsequent **`/v1/*`** routes send **`Authorization: Bearer <access_token>`**. |
| MFA / lockout | Enforced in **`internal/app/auth`**; tunables in **`internal/config`** — operational defaults **[`../runbooks/configuration.md`](../runbooks/configuration.md)**. |
| OIDC / SSO | **[`../runbooks/oidc-sso-integration.md`](../runbooks/oidc-sso-integration.md)** when enterprise IdP is wired. |

High-level sequence: **[`../architecture/data-flow.md`](../architecture/data-flow.md)** (Admin REST diagram).

---

## Fleet operations

Fleet CRUD, machine lifecycle, technician workflows — **`/v1/admin/organizations/{organizationId}/...`** routes under fleet namespaces (OpenAPI **`DocOp*`** inventory). Operational runbooks: **[`../runbooks/technician-setup.md`](../runbooks/technician-setup.md)**, **[`setup-machine.md`](setup-machine.md)**, **[`../runbooks/machine-activation.md`](../runbooks/machine-activation.md)**.

---

## Catalog / media operations

Catalog admin, promotions, pricing, product media URLs — Admin REST + **`internal/app/catalogadmin`**, **`internal/app/mediaadmin`**. Media HTTPS/hash/cache semantics: **[`../architecture/media-sync.md`](../architecture/media-sync.md)** (architecture), **[`../runbooks/product-media-cache-invalidation.md`](../runbooks/product-media-cache-invalidation.md)**.

---

## Payment reconciliation & refunds

Admin APIs list cases, resolve/ignore, request refunds, read order timelines — **[`../runbooks/payment-reconciliation.md`](../runbooks/payment-reconciliation.md)** (endpoint table). Webhook verification and PSP semantics: **[`payment-webhook-security.md`](payment-webhook-security.md)**. Debugging: **[`../runbooks/payment-webhook-debug.md`](../runbooks/payment-webhook-debug.md)**.

---

## Inventory operations

Stock adjustments, refill forecasting, admin inventory surfaces — **`/v1/admin/...`** routes (see OpenAPI); vending-machine-side adjustments remain distinguished from kiosk **`MachineInventoryService`** on gRPC — **[`inventory-adjustments.md`](inventory-adjustments.md)**.

---

## Audit access

Enterprise audit trails (where configured), reporting exports — **`internal/app/audit`**, reporting **`/v1/reports/*`** / **`/v1/admin/.../reports/*`** as documented in OpenAPI. Classification of clients vs transports: **[`api-client-classification.md`](api-client-classification.md)**.

---

## Related

- **Machine runtime (native):** **[`machine-grpc.md`](machine-grpc.md)** — activation, bootstrap, catalog/media deltas, commerce, inventory, telemetry, offline replay, idempotency.
- **Internal read/query gRPC** (**`INTERNAL_GRPC_ENABLED`**, loopback **`avf.internal.v1`**) — **service JWT** only; **not** a substitute for Admin REST — **[`internal-grpc.md`](internal-grpc.md)**.
