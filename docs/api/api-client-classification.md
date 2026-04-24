# API client classification

This document maps **who should call which HTTP surface** on the AVF Vending API. It complements OpenAPI (`docs/swagger/swagger.json`) and the route comments in `internal/httpserver/router.go`.

For enterprise release reviews, use this table together with [API surface security](../runbooks/api-surface-security.md).

## Client profiles

| Client | Typical credential | Primary transports | Must not use for |
| --- | --- | --- | --- |
| **Kiosk runtime app** | Machine-scoped JWT (`machine_ids` claim) or equivalent | MQTT (telemetry, commands), on-device commerce UX | Bulk admin APIs, reporting, raw fleet lists |
| **Technician setup app** | User JWT (`org_admin` / `org_member` / `technician`) + operator sessions | HTTPS: bootstrap, planogram, inventory, operator-session APIs | Payment provider webhooks |
| **Admin portal** | User JWT (`platform_admin` / `org_admin`) | HTTPS: `/v1/admin/*`, `/v1/reports/*`, command dispatch | N/A (full control plane) |
| **Payment provider / webhook** | HMAC (`X-AVF-Webhook-*`) on dedicated callback route | HTTPS POST webhook | Bearer JWT (not used on that route) |
| **Device HTTP fallback** | **Today:** fleet JWT (`platform_admin` / `org_admin`) with machine URL access. **Target hygiene:** also allow machine-scoped JWT for the path `machineId` (see runbook). | HTTPS: `POST /v1/device/machines/{machineId}/vend-results`, `.../commands/poll` | High-volume telemetry (use MQTT / JetStream) |
| **DevOps / monitoring** | Network access to ops bind / health | `GET /health/*`, optional `GET /metrics`, ops mux | Business JWT for metrics (see runbook) |

## Route groups (summary)

Columns: **Auth** = mechanism at the edge. **Roles** = coarse RBAC after Bearer validation. **Idempotency** = required headers for writes where applicable. **Offline retry** = safe to retry POST with same idempotency key after transport loss. **Kiosk** = should kiosk runtime call this in normal operation?

> **Note:** Exact behavior is defined in code (`internal/httpserver/server.go`, `internal/platform/auth/middleware.go`). If this doc disagrees with code, treat code as authoritative until docs are updated.

### System (no `/v1` prefix)

| Path | Intended client | Auth | Roles | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| `GET /health/live`, `GET /health/ready` | DevOps | None | — | — | Yes (GET) | Optional |
| `GET /version` | DevOps, support | None | — | — | Yes | Optional |
| `GET /metrics` (when `METRICS_ENABLED=true`) | DevOps | None (see runbook: bind / network policy) | — | — | Yes | No |
| `GET /swagger/*` (when Swagger enabled) | Humans, integrators | None (docs only) | — | — | Yes | No |

### Auth (`/v1/auth`)

| Path | Intended client | Auth | Roles | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /v1/auth/login`, `refresh` | Technician setup, Admin portal | Body + optional headers (see OpenAPI) | — | — | Varies | No (interactive) |
| `GET /v1/auth/me`, `POST /v1/auth/logout` (bearer group) | Same | Bearer JWT | — | — | — | No |

### Admin (`/v1/admin`)

| Path | Intended client | Auth | Roles | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| Catalog, fleet lists, machine directory, inventory, artifacts, planogram writes | Admin portal (sometimes technician tooling) | Bearer JWT | `platform_admin` **or** `org_admin` | On mutating routes where OpenAPI lists `Idempotency-Key` | Yes when key reused | **No** |

`platform_admin` often supplies `organization_id` query parameter to pick a tenant.

### Reporting (`/v1/reports`)

| Path | Intended client | Auth | Roles | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| Sales / payments / fleet-health / inventory-exceptions | Admin portal | Bearer JWT | `platform_admin` **or** `org_admin` | — (GET) | Yes | **No** |

Machine tokens must **not** be used here; edge RBAC rejects roles outside the allow-list.

### Operator insights (`/v1/operator-insights`)

| Path | Intended client | Auth | Roles | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| Cross-machine read-only insights | Admin portal, support | Bearer JWT | `platform_admin`, `org_admin`, **or** `org_member` | — | Yes | **No** |

### Commerce (`/v1/commerce`)

| Path | Intended client | Auth | Roles / scope | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| Checkout, orders, payment-session, vend transitions | Kiosk runtime (via app), integrations | Bearer JWT + org scope | `RequireOrganizationScope` (see middleware) | Required on listed POSTs | Yes with same key | Yes (subset) |
| `POST .../payments/{paymentId}/webhooks` | Payment provider | **HMAC** (no Bearer) | Provider secret | Provider-managed | Provider retries | No |

Tenant isolation is enforced from JWT org scope plus commerce service checks (e.g. slot / assortment resolution against `organization_id` + `machine_id`).

### Tenant lists (`/v1/payments`, `/v1/orders`)

| Path | Intended client | Auth | Roles | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| List payments / orders | Admin portal, back-office | Bearer JWT + org scope | Non–platform users need org on token | — | Yes | No |

### Machine-scoped runtime (`/v1/machines/{machineId}/...`)

| Path | Intended client | Auth | Roles | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| `GET .../shadow`, telemetry snapshot/incidents/rollups | Kiosk, technician, admin (debug) | Bearer JWT | Machine URL access + **tenant binding** (see runbook) | — | Yes | Yes (subset) |
| `POST .../check-ins`, `.../config-applies` | Kiosk runtime | Bearer JWT + machine access | Same | — | Yes (idempotent design) | Yes |
| `POST .../commands/dispatch`, command status/receipts | Admin portal (ops) | Bearer JWT | Machine access + **`platform_admin` or `org_admin`** | `Idempotency-Key` on dispatch | Yes | Rarely |
| `.../operator-sessions/*` | Technician setup | Bearer JWT + machine access | Operator flows | Varies | Varies | No (operator UX) |

### Setup (`/v1/setup/machines/{machineId}/bootstrap`)

| Path | Intended client | Auth | Roles | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| Bootstrap | Technician setup | Bearer JWT + machine access | Same as machine reads | — | Yes | During provisioning |

### Device HTTP bridge (`/v1/device/machines/{machineId}/...`)

| Path | Intended client | Auth | Roles | Idempotency | Offline retry | Kiosk |
| --- | --- | --- | --- | --- | --- | --- |
| `POST .../vend-results` | Device integration / fallback | Bearer JWT | Machine access + **`platform_admin` or `org_admin`** today; machine-scoped bridge is a documented hardening item | Required | Yes | Fallback path |
| `POST .../commands/poll` | Device integration / fallback | Same | Same | — | Yes | **Fallback only** — primary command delivery is MQTT |

High-volume **telemetry** remains on **MQTT → JetStream** (see `docs/api/mqtt-contract.md`); do not push firehose workloads through these HTTP read models.

## Deprecation policy

- No route is removed without proof of non-use (tests + docs agreement).
- Obsolete routes should be marked **`deprecated: true`** in OpenAPI first, then removed in a later release.

## Related

- [MQTT contract & HTTP fallbacks](mqtt-contract.md)
- [API surface security runbook](../runbooks/api-surface-security.md)
