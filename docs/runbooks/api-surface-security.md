# API surface security (enterprise checklist)

Operational checklist for **auth**, **tenant isolation**, and **client-appropriate** use of HTTP endpoints. Pair with [API client classification](../api/api-client-classification.md).

## 1. Intended clients vs routes

- **Kiosk runtime** should use machine-scoped JWTs for `/v1/machines/{machineId}/*` (shadow, telemetry reads, check-ins, config-applies, commerce) and **MQTT** for telemetry + commands.
- **Admin portal** uses `platform_admin` / `org_admin` for `/v1/admin/*`, `/v1/reports/*`, and command dispatch.
- **Technician setup** uses bootstrap, inventory/planogram admin routes, and operator-session APIs.
- **Payment providers** use **only** the HMAC-verified webhook route (no Bearer JWT).

## 2. Role guards (code reference)

Registration lives in `internal/httpserver/server.go`:

- `/v1/admin/*` — `RequireAnyRole(platform_admin, org_admin)`.
- `/v1/reports/*` — same; machine-only tokens must not carry these roles.
- `/v1/commerce/*` (except public webhook) — Bearer JWT + `RequireOrganizationScope`.
- `/v1/device/machines/{machineId}/*` — Bearer JWT + `RequireMachineURLAccess` + **`RequireAnyRole(platform_admin, org_admin)`** today (`internal/httpserver/device_http.go`). **Hardening:** add `CanAccessHTTPDeviceBridge`-style policy so machine-scoped JWTs with `machine_ids` may call the bridge without org-admin role, still subject to tenant binding middleware in §3.
- `/metrics` on the **main** HTTP server — **unauthenticated** when `METRICS_ENABLED=true`; restrict by **bind address**, reverse proxy ACL, or private network. Optional **ops** HTTP server (`OPS_HTTP_ADDR`) hosts readiness and may expose metrics separately — see `internal/observability/ops_http.go`.
- `/swagger/*` — intentionally **public documentation** when `HTTP_SWAGGER_UI_ENABLED=true`; `/v1/*` still requires Bearer JWT except explicitly public routes (e.g. commerce webhook).

## 3. Tenant isolation for machine URLs

**Issue (historical):** `CanAccessMachineRead` allowed any `org_admin` with an org claim to pass the **HTTP** machine-URL check for **arbitrary** `machineId` UUIDs before the handler ran; some read handlers (e.g. machine shadow) did not re-check `machines.organization_id`.

**Hardening (recommended in code):** after Bearer auth, resolve `machines.organization_id` for the path `machineId` and enforce `AuthorizePrincipalForMachineRow` (see `internal/platform/auth/machine_access.go` when implemented) on:

- `GET /v1/machines/{machineId}/shadow`
- Telemetry GETs under `/v1/machines/{machineId}/telemetry/*`
- Operator-session routes and setup bootstrap
- Machine runtime POSTs

**Implementation sketch:** middleware in `internal/httpserver` using `app.TelemetryStore.Pool()` + `GetMachineByID` (sqlc). When `TelemetryStore` is nil (incomplete test doubles), decide explicitly: fail closed for org-scoped principals or skip only in tests.

## 4. Device HTTP fallback

- `POST /v1/device/machines/{machineId}/commands/poll` is an **HTTP fallback** when MQTT is degraded — **not** the primary high-volume command path.
- Document in OpenAPI descriptions that integrators should prefer **MQTT** command delivery (`docs/api/mqtt-contract.md`).
- Telemetry firehose belongs on **MQTT → JetStream**, not on HTTP telemetry read models.

## 5. Commerce webhook verification

- **Mode:** `COMMERCE_PAYMENT_WEBHOOK_VERIFICATION=avf_hmac` (default). **HMAC** headers `X-AVF-Webhook-Timestamp` and `X-AVF-Webhook-Signature` over `{timestamp}.{rawBody}` when `COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET` (or legacy `PAYMENT_WEBHOOK_SECRET`) is set.
- **Skew:** `COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS` (default 300; min 30, max 86400). Stale timestamps → **400** `webhook_timestamp_skew`; bad signature → **401** `webhook_auth_failed`.
- **Provider:** body `provider` must match the payment’s provider → **403** `webhook_provider_mismatch` if not.
- **Production:** unsigned webhooks → **403** `webhook_hmac_required` unless `COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION=true` (documented unsafe). Non-production may use `COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=true` with an empty secret for local testing.
- **Replay:** `provider_reference` and optional `webhook_event_id` (per-provider unique). See `docs/api/payment-webhook-security.md`.
- **Tests:** `internal/httpserver/commerce_webhook_*_test.go`, `internal/modules/postgres/commerce_webhook_replay_integration_test.go`, `internal/modules/postgres/commerce_webhook_provider_mismatch_integration_test.go`, `internal/config/config_test.go`.

## 6. Verification commands (CI / local)

- `go test ./...`
- `make swagger` / `python tools/build_openapi.py`
- Manual: with Swagger enabled, `GET /swagger/doc.json` returns OpenAPI; protected `/v1/...` returns **401** without `Authorization`.

## 7. Deprecation

- Do not delete routes without evidence of non-use and documentation agreement.
- Mark `deprecated` in OpenAPI (`swagger_operations.go` + regenerate) before removal.

## 8. Implementation backlog (requires Agent mode / non–plan-mode edits)

The following are **not** fully implemented as code in the same pass as the markdown docs; apply in the repo when edits are permitted:

1. **Tenant-bound machine middleware** — After `RequireMachineURLAccess`, resolve `machines.organization_id` and call `AuthorizePrincipalForMachineRow` for shadow, telemetry, setup bootstrap, operator-session groups, and machine runtime POSTs.
2. **Device bridge actor** — Replace `RequireAnyRole(platform_admin, org_admin)`-only policy on `/v1/device/...` with `RequireHTTPDeviceBridgeAccess` (admins **or** `machine_ids` allow-list for the path machine).
3. **OpenAPI** — Enrich `DocOp*` descriptions for `commands/poll` (HTTP fallback, MQTT primary) and telemetry routes (projection only, not MQTT firehose). Optionally extend `tools/build_openapi.py` tag blurbs for **Admin** vs **Fleet** vs **Device**.
4. **Tests** (`internal/httpserver/*_test.go`) — Configurable JWT stub validator returning principals: machine-scoped token → `GET /v1/admin/machines` → **403**; org A admin → `GET /v1/machines/{orgB_machine}/shadow` → **403** (with DB middleware); webhook unsigned → **401** (existing); Swagger enabled → `GET /swagger/index.html` **200**; no `Authorization` on `/v1/machines/.../shadow` → **401**.
