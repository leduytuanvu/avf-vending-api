# API surface audit (enterprise)

**OpenAPI:** `docs/swagger/swagger.json` (generated). Regenerate: `make swagger` or `python tools/build_openapi.py`.

**Classification:** HTTP paths in OpenAPI are **`public` operator** (Bearer JWT admin), **device/bridge** (machine-scoped or technician flows; **not registered in production** unless `ENABLE_LEGACY_MACHINE_HTTP=true` with allow flag), **webhook** (HMAC), or **legacy/deprecated** machine REST. **`avf.machine.v1`** and **`avf.internal.v1`** are **not** Admin REST — do not publish them as `/v1/admin` or anonymous HTTP substitutes.

**Telemetry:** High-volume device telemetry and primary command delivery are **MQTT-first**. HTTP device routes (`/v1/device/...`) are **integration / fallback**; `POST .../commands/poll` is **not** the primary command path.

**Swagger / OpenAPI hosting:** Treat `/swagger/*` as **public only when intentionally enabled** at the edge. Operators should assume documentation endpoints can be disabled in locked-down production.

**Metrics:** `GET /metrics` is an **internal / protected** ops surface (private bind, scrape token, or network ACL). Do not expose unauthenticated metrics on a public listener.

**Reports / admin lists:** `GET /v1/reports/*`, `GET /v1/orders`, `GET /v1/payments`, and most `GET /v1/admin/*` routes are **admin portal** surfaces — **not** kiosk runtime dependencies for sale execution.

**Verification:** `make verify-enterprise-release` runs `go test ./...`, Swagger drift checks, `tools/openapi_verify_release.py` (production server first, Bearer on protected `/v1` routes, write examples, success+error examples, no planned-only paths, no secret-like examples), shell/compose checks, and doc secret heuristics.

**Regenerate the matrix table:** `python scripts/compose_api_surface_audit_md.py` (preferred). For inspection only: `python scripts/gen_api_surface_audit_table.py`.

---

## Legend

| Column | Meaning |
| --- | --- |
| **intended client** | Primary consumer class: kiosk runtime app, technician setup app, admin portal, payment provider, device HTTP fallback, DevOps/monitoring. |
| **idempotency required** | `yes` = use `Idempotency-Key` / `X-Idempotency-Key` per OpenAPI; `no` = not required by header contract; `n/a` = read-only. |
| **offline retry safe** | `yes` = safe GET or clearly safe retry; `yes w/ key` = retries must reuse idempotency key; `caution` = may duplicate side effects without care; `no` = online-only. |
| **status** | `keep` = primary HTTP surface; `fallback` = secondary to MQTT or non-primary design; `internal` = ops-only expectations; `deprecated` / `roadmap` = not used in current OpenAPI (must not appear for shipped routes). |

---

## Full route matrix

The table lists **every** `paths` entry in `docs/swagger/swagger.json` (one row per HTTP method).

| endpoint | method | intended client | auth type | role/scope | idempotency required | offline retry safe | status | production risk |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| /health/live | GET | DevOps/monitoring | none | (n/a) | n/a | yes | keep | low |
| /health/ready | GET | DevOps/monitoring | none | (n/a) | n/a | yes | keep | low |
| /metrics | GET | DevOps/monitoring | none on ops by default; **Bearer** when `METRICS_SCRAPE_TOKEN` set (ops + public); private bind preferred | ops; metrics reader | n/a | yes | internal | high if exposed on public listener without ACL |
| /swagger/doc.json | GET | DevOps/monitoring; integrators (when enabled) | none (public only when intentionally enabled) | (n/a) | n/a | yes | keep | med: disable on edge or protect if secrets could leak via misconfig |
| /swagger/index.html | GET | DevOps/monitoring; integrators (when enabled) | none (public only when intentionally enabled) | (n/a) | n/a | yes | keep | med: disable on edge or protect if secrets could leak via misconfig |
| /v1/admin/assignments | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/brands | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/brands | POST | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/brands/{brandId} | DELETE | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/brands/{brandId} | PATCH | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/brands/{brandId} | PUT | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/categories | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/categories | POST | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/categories/{categoryId} | DELETE | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/categories/{categoryId} | PATCH | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/categories/{categoryId} | PUT | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/commands | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/machines | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/machines/{machineId} | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/machines/{machineId}/activation-codes | GET | admin portal (provisioning; not kiosk runtime) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | med: code issuance abuse |
| /v1/admin/machines/{machineId}/activation-codes | POST | admin portal (provisioning; not kiosk runtime) | Bearer JWT | org_admin or platform_admin | no | caution | keep | med: code issuance abuse |
| /v1/admin/machines/{machineId}/activation-codes/{activationCodeId} | DELETE | admin portal (provisioning; not kiosk runtime) | Bearer JWT | org_admin or platform_admin | no | caution | keep | med: code issuance abuse |
| /v1/admin/machines/{machineId}/cash-collections | GET | technician setup app; admin portal | Bearer JWT | org scope + operator session (writes) | n/a | yes | keep | high: cash settlement |
| /v1/admin/machines/{machineId}/cash-collections | POST | technician setup app; admin portal | Bearer JWT | org scope + operator session (writes) | yes | yes w/ key | keep | high: cash settlement |
| /v1/admin/machines/{machineId}/cash-collections/{collectionId} | GET | technician setup app; admin portal | Bearer JWT | org scope + operator session (writes) | n/a | yes | keep | high: cash settlement |
| /v1/admin/machines/{machineId}/cash-collections/{collectionId}/close | POST | technician setup app; admin portal | Bearer JWT | org scope + operator session (writes) | yes | yes w/ key | keep | high: cash settlement |
| /v1/admin/machines/{machineId}/cashbox | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/machines/{machineId}/inventory | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/machines/{machineId}/inventory-events | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/machines/{machineId}/planograms/draft | PUT | technician setup app; admin portal | Bearer JWT | org_admin or platform_admin | no | caution | keep | med |
| /v1/admin/machines/{machineId}/planograms/publish | POST | technician setup app | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/machines/{machineId}/slots | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/machines/{machineId}/stock-adjustments | POST | technician setup app; admin portal | Bearer JWT | org_admin or platform_admin plus operator session | yes | yes w/ key | keep | med: inventory truth |
| /v1/admin/machines/{machineId}/sync | POST | technician setup app | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | low |
| /v1/admin/machines/{machineId}/topology | PUT | technician setup app; admin portal | Bearer JWT | org_admin or platform_admin | no | caution | keep | med |
| /v1/admin/organizations/{orgId}/artifacts | GET | admin portal | Bearer JWT | org or platform (artifact storage route) | n/a | yes | keep | low |
| /v1/admin/organizations/{orgId}/artifacts | POST | admin portal | Bearer JWT | org or platform (artifact storage route) | yes | yes w/ key | keep | med |
| /v1/admin/organizations/{orgId}/artifacts/{artifactId} | DELETE | admin portal | Bearer JWT | org or platform (artifact storage route) | yes | yes w/ key | keep | med |
| /v1/admin/organizations/{orgId}/artifacts/{artifactId} | GET | admin portal | Bearer JWT | org or platform (artifact storage route) | n/a | yes | keep | low |
| /v1/admin/organizations/{orgId}/artifacts/{artifactId}/content | PUT | admin portal | Bearer JWT | org or platform (artifact storage route) | yes | yes w/ key | keep | med |
| /v1/admin/organizations/{orgId}/artifacts/{artifactId}/download | GET | admin portal | Bearer JWT | org or platform (artifact storage route) | n/a | yes | keep | low |
| /v1/admin/ota | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/planograms | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/planograms/{planogramId} | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/price-books | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/products | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/products | POST | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/products/{productId} | DELETE | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/products/{productId} | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/products/{productId} | PATCH | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/products/{productId} | PUT | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/products/{productId}/image | DELETE | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/products/{productId}/image | PUT | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/tags | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/admin/tags | POST | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/tags/{tagId} | DELETE | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/tags/{tagId} | PATCH | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/tags/{tagId} | PUT | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | yes | yes w/ key | keep | med |
| /v1/admin/technicians | GET | admin portal; technician setup app (where machine-scoped) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/auth/login | POST | technician setup app; admin portal | none (body credentials / refresh token) | (n/a) | n/a | caution | keep | med: credential handling |
| /v1/auth/logout | POST | technician setup app; admin portal | Bearer JWT | authenticated principal | n/a | caution | keep | low |
| /v1/auth/me | GET | technician setup app; admin portal | Bearer JWT | authenticated principal | n/a | yes | keep | low |
| /v1/auth/refresh | POST | technician setup app; admin portal | none (body credentials / refresh token) | (n/a) | n/a | caution | keep | med: credential handling |
| /v1/commerce/cash-checkout | POST | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | yes | yes w/ key | keep | high |
| /v1/commerce/orders | POST | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | yes | yes w/ key | keep | high |
| /v1/commerce/orders/{orderId} | GET | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | n/a | yes | keep | med |
| /v1/commerce/orders/{orderId}/cancel | POST | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | yes | yes w/ key | keep | high |
| /v1/commerce/orders/{orderId}/payment-session | POST | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | yes | yes w/ key | keep | med |
| /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks | POST | payment provider | HMAC (no Bearer) | PSP callback | n/a | no | keep | high: financial integrity; replay protection |
| /v1/commerce/orders/{orderId}/reconciliation | GET | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | n/a | yes | keep | med |
| /v1/commerce/orders/{orderId}/refunds | GET | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | n/a | yes | keep | high |
| /v1/commerce/orders/{orderId}/refunds | POST | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | yes | yes w/ key | keep | high |
| /v1/commerce/orders/{orderId}/refunds/{refundId} | GET | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | n/a | yes | keep | med |
| /v1/commerce/orders/{orderId}/vend/failure | POST | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | yes | yes w/ key | keep | high |
| /v1/commerce/orders/{orderId}/vend/start | POST | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | yes | yes w/ key | keep | high |
| /v1/commerce/orders/{orderId}/vend/success | POST | kiosk runtime app; admin portal (reads/cancel/refund) | Bearer JWT | org and order access | yes | yes w/ key | keep | high |
| /v1/device/machines/{machineId}/commands/poll | POST | device HTTP fallback; kiosk integration | Bearer JWT | machine + dispatch roles (see server) | no | caution | fallback | med: not primary command path; prefer MQTT |
| /v1/device/machines/{machineId}/events/reconcile | POST | kiosk runtime app; device HTTP fallback | Bearer JWT | machine, org, or platform | no (batch keys inside body; follow contract) | caution | keep | MQTT-first for volume; HTTP batch for critical reconcile only per contract |
| /v1/device/machines/{machineId}/events/{idempotencyKey}/status | GET | kiosk runtime app | Bearer JWT | machine, org, or platform | n/a | yes | keep | low |
| /v1/device/machines/{machineId}/vend-results | POST | device HTTP fallback | Bearer JWT | machine / bridge roles | yes | yes w/ key | fallback | med |
| /v1/machines/{machineId}/check-ins | POST | kiosk runtime app | Bearer JWT | machine tenant | no | caution | keep | med |
| /v1/machines/{machineId}/commands/dispatch | POST | admin portal; kiosk (dispatch receipts) | Bearer JWT | admin roles / machine tenant (see route) | yes | yes w/ key | keep | med |
| /v1/machines/{machineId}/commands/receipts | GET | admin portal; kiosk (dispatch receipts) | Bearer JWT | admin roles / machine tenant (see route) | n/a | yes | keep | low |
| /v1/machines/{machineId}/commands/{sequence}/status | GET | admin portal; kiosk (dispatch receipts) | Bearer JWT | admin roles / machine tenant (see route) | n/a | yes | keep | low |
| /v1/machines/{machineId}/config-applies | POST | kiosk runtime app | Bearer JWT | machine tenant | no | caution | keep | med |
| /v1/machines/{machineId}/operator-sessions/action-attributions | GET | technician setup app | Bearer JWT | machine URL access + operator session | n/a | yes | keep | low |
| /v1/machines/{machineId}/operator-sessions/auth-events | GET | technician setup app | Bearer JWT | machine URL access + operator session | n/a | yes | keep | low |
| /v1/machines/{machineId}/operator-sessions/current | GET | technician setup app | Bearer JWT | machine URL access + operator session | n/a | yes | keep | low |
| /v1/machines/{machineId}/operator-sessions/history | GET | technician setup app | Bearer JWT | machine URL access + operator session | n/a | yes | keep | low |
| /v1/machines/{machineId}/operator-sessions/login | POST | technician setup app | Bearer JWT | machine URL access + operator session | no | caution | keep | low |
| /v1/machines/{machineId}/operator-sessions/logout | POST | technician setup app | Bearer JWT | machine URL access + operator session | no | caution | keep | low |
| /v1/machines/{machineId}/operator-sessions/timeline | GET | technician setup app | Bearer JWT | machine URL access + operator session | n/a | yes | keep | low |
| /v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat | POST | technician setup app | Bearer JWT | machine URL access + operator session | no | caution | keep | low |
| /v1/machines/{machineId}/sale-catalog | GET | kiosk runtime app | Bearer JWT | machine tenant | n/a | yes | keep | low; not for admin bulk export |
| /v1/machines/{machineId}/shadow | GET | kiosk runtime app; admin portal | Bearer JWT | machine tenant | n/a | yes | keep | low |
| /v1/machines/{machineId}/telemetry/incidents | GET | kiosk runtime app; admin portal | Bearer JWT | machine tenant | n/a | yes | keep | med: snapshot/rollups are low-rate HTTP; flood telemetry is MQTT-first, not these GETs |
| /v1/machines/{machineId}/telemetry/rollups | GET | kiosk runtime app; admin portal | Bearer JWT | machine tenant | n/a | yes | keep | med: snapshot/rollups are low-rate HTTP; flood telemetry is MQTT-first, not these GETs |
| /v1/machines/{machineId}/telemetry/snapshot | GET | kiosk runtime app; admin portal | Bearer JWT | machine tenant | n/a | yes | keep | med: snapshot/rollups are low-rate HTTP; flood telemetry is MQTT-first, not these GETs |
| /v1/operator-insights/technicians/{technicianId}/action-attributions | GET | admin portal | Bearer JWT | platform, org_admin, or org_member | n/a | yes | keep | low |
| /v1/operator-insights/users/action-attributions | GET | admin portal | Bearer JWT | platform, org_admin, or org_member | n/a | yes | keep | low |
| /v1/orders | GET | admin portal (not kiosk runtime) | Bearer JWT | org-scoped lists | n/a | yes | keep | low |
| /v1/payments | GET | admin portal (not kiosk runtime) | Bearer JWT | org-scoped lists | n/a | yes | keep | low |
| /v1/reports/fleet-health | GET | admin portal (not kiosk runtime) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/reports/inventory-exceptions | GET | admin portal (not kiosk runtime) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/reports/payments-summary | GET | admin portal (not kiosk runtime) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/reports/sales-summary | GET | admin portal (not kiosk runtime) | Bearer JWT | org_admin or platform_admin | n/a | yes | keep | low |
| /v1/setup/activation-codes/claim | POST | kiosk runtime app (first install) | none (public claim) | activation code + device fingerprint | n/a | no | keep | med: provisioning abuse; rate limits |
| /v1/setup/machines/{machineId}/bootstrap | GET | kiosk runtime app; technician setup app | Bearer JWT | machine tenant | n/a | yes | keep | low |
| /version | GET | DevOps/monitoring | none | (n/a) | n/a | yes | keep | low |

---

## Planned-only HTTP (not in OpenAPI)

Product ideas that are **not** shipped as public `paths` belong in [roadmap.md](roadmap.md) only. If a path appears in OpenAPI, it is **implemented** for this revision (subject to nil wiring returning 503 where applicable).

---

## Related

- [Kiosk app flow](kiosk-app-flow.md)
- [API client classification](api-client-classification.md)
- [API surface security](../runbooks/api-surface-security.md)
- [MQTT contract](mqtt-contract.md)
