# Kiosk app — backend API contract checklist (Android)

Backend-oriented checklist for **correct API usage order**, **local persistence**, **offline**, and **security**. Pair with:

- Narrative flow: [kiosk-app-flow.md](kiosk-app-flow.md)
- MQTT wire + replay: [mqtt-contract.md](mqtt-contract.md), [examples/device-offline-replay-samples.md](examples/device-offline-replay-samples.md)
- Payload / idempotency tables: [examples/kiosk-implementation-payloads.md](examples/kiosk-implementation-payloads.md)
- Fixture index: [../../testdata/api-contract/README.md](../../testdata/api-contract/README.md)

**OpenAPI** (`docs/swagger/swagger.json`, regenerate with `make swagger`) is authoritative for **request/response shapes** on HTTP. This checklist names routes and behavior; it does not duplicate every field.

**Version gate:** Before shipping, confirm each **“Target / confirm”** row against the deployed API revision (some routes are product targets and may not be on every environment yet).

---

## 1. Local storage (Room / DataStore)

| # | Item | Requirement | Backend touchpoint |
| --- | --- | --- | --- |
| 1.1 | **Machine binding** | Persist `machineId` (UUID), optional `organizationId`, site hints from bootstrap/claims. Never hardcode in customer build. | `GET /v1/setup/machines/{machineId}/bootstrap`; activation/claim (**confirm** OpenAPI) |
| 1.2 | **Token storage** | Store **access** JWT in memory or encrypted storage; store **refresh** token in encrypted storage (Android Keystore-backed). Never log tokens. | `POST /v1/auth/login`, `POST /v1/auth/refresh`; machine-scoped JWT from provisioning (**confirm**) |
| 1.3 | **Sale catalog cache** | Room table: SKU/slot lines, prices, currency, `catalogVersion` or `updatedAt` from server, TTL policy. Load **cache first** on cold start. | `GET /v1/machines/{machineId}/sale-catalog` (**target** — confirm mounted); interim: bootstrap `catalog` + admin reads during setup |
| 1.4 | **Image cache** | Cache product images keyed by **`contentHash`** (or server-provided SHA) + optional URL; invalidate when hash changes. | Sale catalog / product media fields (**confirm** response schema) |
| 1.5 | **Durable telemetry outbox** | SQLite/Room queue: critical MQTT envelopes pending broker ACK. Store `dedupe_key`, `event_type`, `occurred_at`, raw payload, retry count, next_attempt_at. | [mqtt-contract.md](mqtt-contract.md) offline replay; fixtures under `testdata/telemetry/` |

---

## 2. Startup sequence (strict order)

Complete steps **in order** on each cold start (after binding exists). Skip HTTP steps when offline until network is available.

| Step | Action | Endpoint / transport | Auth | Idempotency / retry |
| --- | --- | --- | --- | --- |
| S1 | **Load Room cache first** | Local only | — | — |
| S2 | **Refresh token if near expiry** | `POST /v1/auth/refresh` | Refresh token | Safe retry; rotate refresh if API returns new one |
| S3 | **Sale catalog sync** (online) | `GET /v1/machines/{machineId}/sale-catalog` (**target**) | Bearer (machine-scoped) | GET retry safe; backoff + jitter |
| S3′ | **Interim** if sale-catalog missing | `GET /v1/setup/machines/{machineId}/bootstrap` | Bearer | GET retry safe |
| S4 | **Check-in** | `POST /v1/machines/{machineId}/check-ins` | Bearer | Retry with **same** business key in body/metadata if API supports correlation; else acceptable duplicate rows per product policy |
| S5 | **Shadow optional** | `GET /v1/machines/{machineId}/shadow` | Bearer | GET retry safe |
| S6 | **Connect MQTT** | TLS to broker; subscribe per [mqtt-contract.md](mqtt-contract.md) | Device certs / policy per deployment | Auto-reconnect with backoff |
| S7 | **Replay telemetry outbox** | Publish queued messages QoS 1 | N/A | **Jitter + pacing** between messages; respect `next_attempt_at`; never stampede after reconnect |
| S8 | **Critical reconcile** (when HTTP API available) | `POST /v1/machines/{machineId}/events/reconcile` (**target**) | Bearer | **Idempotency-Key** per batch or per logical reconcile (**confirm** spec) |

**MQTT primary:** Commands arrive on `…/commands/dispatch` (API → broker). Use **`POST /v1/device/machines/{machineId}/commands/poll` only as HTTP fallback** (integration/admin policy today — confirm JWT roles for your deployment).

---

## 3. Sale flow (happy path + failure)

Use a **single logical Idempotency-Key** per customer intent where the backend requires it (commerce POSTs). Keys must be **deterministic** from client: e.g. `{machineId}:{localSaleId}`.

| Phase | Step | Endpoint | Method | Idempotency-Key |
| --- | --- | --- | --- | --- |
| P1 | Create order **or** cash checkout | `/v1/commerce/orders` **or** `/v1/commerce/cash-checkout` | POST | **Required** |
| P2 | Wallet / PSP path | `/v1/commerce/orders/{orderId}/payment-session` | POST | **Required** |
| P3 | Poll order state if needed | `/v1/commerce/orders/{orderId}` | GET | — |
| P4 | Reconciliation snapshot | `/v1/commerce/orders/{orderId}/reconciliation` | GET | — |
| P5 | Start vend | `/v1/commerce/orders/{orderId}/vend/start` | POST | **Required** |
| P6 | **Local hardware vend** | GPIO / MDB / recycler (device) | — | — |
| P7a | Success | `/v1/commerce/orders/{orderId}/vend/success` | POST | **Required** |
| P7b | **Or** bridge | `/v1/device/machines/{machineId}/vend-results` | POST | **Required** (policy) |
| P8 | Failure | `/v1/commerce/orders/{orderId}/vend/failure` | POST | **Required** |

**Refund after online payment + vend failure**

| Step | Endpoint | Notes |
| --- | --- | --- |
| R1 | Report failure | `POST .../vend/failure` first (domain truth) |
| R2 | Refund | `POST /v1/commerce/orders/{orderId}/refunds` or admin workflow (**target** — confirm OpenAPI) |

Until refund routes are on your server, coordinate **manual** refund via ops + `GET /v1/orders` / `GET /v1/payments`.

**Telemetry mirror (critical):** After vend/payment/cash events, publish MQTT envelopes per [mqtt-contract.md](mqtt-contract.md) with `dedupe_key` / `event_id` / `boot_id`+`seq_no` so offline replay is accepted.

---

## 4. Offline behavior

| Topic | Rule |
| --- | --- |
| **Critical events** | Vend, payment, cash insert, inventory deltas (see `internal/platform/telemetry` criticality): **retain** in outbox until broker ACK + (when applicable) **reconcile** HTTP returns success. |
| **Heartbeat / metrics** | May **drop** or **compact** under storage pressure; prefer sampling; never block sale path. |
| **Retry** | Exponential backoff with **full jitter** on HTTP and MQTT publish failures; cap max delay; persist `next_attempt_at`. |
| **Ordering** | Preserve per-`boot_id` `seq_no` monotonicity where possible; replay with pacing to avoid broker pressure. |

Fixture references: `testdata/telemetry/valid_*.json`, `duplicate_replay_vend.json`, invalid identity samples.

---

## 5. Settlement & operator workflows

Customer sale UI must **not** call these unless product is explicitly an operator mode.

| Step | Endpoint | Method | Auth | Idempotency |
| --- | --- | --- | --- | --- |
| Operator login | `/v1/machines/{machineId}/operator-sessions/login` | POST | Bearer | — |
| Heartbeat | `/v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat` | POST | Bearer | — |
| Stock adjustment | `/v1/admin/machines/{machineId}/stock-adjustments` | POST | **Admin** Bearer + operator session body | **Required** |
| Cash summary / collection | `/v1/admin/machines/{machineId}/cashbox`, `.../cash-collections`, `.../close` (**target**) | GET/POST | **Admin** + operator session | Close: **Required** |

Technician **setup** may also use: `PUT .../topology`, `PUT .../planograms/draft`, `POST .../planograms/publish`, `POST .../sync` — all **admin** routes.

---

## 6. Security & UX boundaries

| Rule | Detail |
| --- | --- |
| **Customer sale screen** | Only **machine-scoped** + **commerce** + **telemetry** paths. **Do not** embed calls to `/v1/admin/*`, `/v1/reports/*`, `/v1/operator-insights/*`, or command **dispatch** unless the build is an **internal ops** variant. |
| **Passwords** | Never persist **admin** or **technician** passwords; use short-lived tokens. Technician flows may use `login` only in setup apps with secure keyboard. |
| **Refresh strategy** | Proactively refresh before `exp`; on **401**, try refresh once, then operator re-auth or safe degradation (no sale) per product policy. |
| **Unbind / wipe** | On factory reset or unbind: `POST /v1/auth/logout` if session exists; delete Room **including** refresh token, outbox (or wipe after upload), image cache; revoke refresh server-side if API supports it. |
| **Webhooks** | Kiosk never calls PSP webhook route; that is **server-side** only. |

---

## Acceptance criteria (Android team)

- [ ] Machine id + tokens + catalog + image hash strategy documented in app ADR.
- [ ] Startup order matches **§2**; sale order matches **§3**; offline rules match **§4**.
- [ ] All commerce POSTs send **Idempotency-Key** per §3 table.
- [ ] MQTT outbox uses **dedupe_key** / identity rules from [mqtt-contract.md](mqtt-contract.md).
- [ ] Customer build excludes **admin** base URLs or feature-flags them off.
- [ ] OpenAPI / backend version verified for **target** routes before GA.
