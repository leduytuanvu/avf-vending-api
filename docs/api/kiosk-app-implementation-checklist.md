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
| 1.3 | **Sale catalog cache** | Room table: SKU/slot lines, prices, currency, composite **`catalog_version`** (**`RuntimeSaleCatalogFingerprint`**) from **`GetCatalogSnapshot`**, plus **`generated_at`** and last seen **`Meta.server_time`**; optional separate **`media_fingerprint`** cache key. Refresh via **`GetCatalogDelta`** / **`GetMediaDelta`**. TTL is product policy — see [kiosk-app-flow.md §5](kiosk-app-flow.md). | `GET …/sale-catalog` **or** **`MachineCatalogService/GetCatalogSnapshot`**; [machine-grpc.md](machine-grpc.md) |
| 1.4 | **Image cache** | For each `ProductMediaVariant`: persist **`kind`**, **`media_asset_id`**, **`checksum_sha256`**, **`media_version`**, **`etag`**, **`expires_at`**; store files on disk under a path derived from those fields (not the raw URL). Verify download bytes against **`checksum_sha256`**. Invalidate when **`media_fingerprint`** or per-variant hash/version changes. See [media-sync.md](../architecture/media-sync.md) and [product-media-cache-invalidation.md](../runbooks/product-media-cache-invalidation.md). | `GetCatalogSnapshot` / `GetMediaManifest` (`primary_media.media_variants`) |
| 1.5 | **Durable telemetry outbox** | SQLite/Room queue: critical MQTT envelopes pending broker ACK. Store `dedupe_key`, `event_type`, `occurred_at`, raw payload, retry count, next_attempt_at. | [mqtt-contract.md](mqtt-contract.md) offline replay; fixtures under `testdata/telemetry/` |

**Production contract:** [`production-final-contract.md`](../architecture/production-final-contract.md) — vending apps use **`avf.machine.v1` + Machine JWT**; Admin uses **REST `/v1` + User JWT**; **MQTT TLS** for commands; **legacy machine HTTP off** in production by default.

---

## 2. Startup sequence (strict order)

Complete steps **in order** on each cold start (after binding exists). **Production:** prefer **gRPC** for every step that has an `avf.machine.v1` RPC; **HTTP** paths below are for **staging/lab** or **legacy machine HTTP enabled** environments only.

| Step | Action | Endpoint / transport | Auth | Idempotency / retry |
| --- | --- | --- | --- | --- |
| S1 | **Load Room cache first** | Local only | — | — |
| S2 | **Refresh token if near expiry** | `MachineAuthService/RefreshMachineToken` **or** `POST /v1/auth/refresh` (non-machine) | Refresh token / opaque refresh | Safe retry; persist new refresh material |
| S3 | **Sale catalog sync** (online) | **Primary:** `MachineCatalogService/GetCatalogSnapshot` + `GetCatalogDelta` / `MachineMediaService/*` on **gRPC**. **Legacy:** `GET /v1/machines/{machineId}/sale-catalog` only when HTTP enabled | Machine JWT (gRPC metadata) | GET retry safe; store **`catalog_version`**; see [kiosk-app-flow.md §5](kiosk-app-flow.md) |
| S3′ | **Interim** if snapshot blocked | `MachineBootstrapService/GetBootstrap` **or** `GET /v1/setup/machines/{machineId}/bootstrap` (legacy) | Bearer / Machine JWT | GET retry safe |
| S4 | **Check-in** | `MachineBootstrapService/CheckIn` **or** `POST /v1/machines/{machineId}/check-ins` (legacy) | Machine JWT | Same idempotency key on retry |
| S5 | **Shadow optional** | Poll fields via `GetBootstrap` / catalog responses per product policy | Machine JWT | GET retry safe |
| S6 | **Connect MQTT** | TLS to broker; subscribe per [mqtt-contract.md](mqtt-contract.md) | Client policy per deployment | **Exponential backoff + jitter** reconnect; resubscribe all topics; drain outbox with pacing |
| S7 | **Replay telemetry outbox** | Publish queued messages QoS 1 | N/A | **Jitter + pacing**; respect `next_attempt_at`; never stampede after reconnect |
| S8 | **Critical reconcile** (when HTTP API available) | `MachineTelemetryService/ReconcileEvents` with stable idempotency scope | Machine JWT | See [machine-grpc.md](machine-grpc.md) ledger rules |

**MQTT primary:** Commands arrive on `…/commands/dispatch` (API → broker). Use **`POST /v1/device/machines/{machineId}/commands/poll` only as HTTP fallback** (integration/admin policy today — confirm JWT roles for your deployment).

---

## 3. Sale flow (happy path + failure)

### 3a. Production — gRPC (`MachineCommerceService` / `MachineSaleService`)

Use **one logical idempotency key** per customer intent on **every** mutating RPC that accepts `IdempotencyContext` / `MachineRequestMeta.idempotency_key`. Example stable key: `"{machineId}:{localSaleId}"` (store `localSaleId` in Room before first RPC).

| Phase | Step | gRPC method | Idempotency |
| --- | --- | --- | --- |
| P1 | Create order | `CreateOrder` (**or** `CreateSale`) | **Required** stable key |
| P2 | Wallet / PSP path | `CreatePaymentSession` (**or** `AttachPaymentResult` after external wallet step if applicable) | **Required** |
| P3 | Poll order state | `GetOrder` / `GetOrderStatus` | — |
| P4 | Reconciliation read | Use admin REST or `GetOrder` fields exposing settlement state per proto — **confirm** deployed revision | — |
| P5 | Start vend | `StartVend` | **Required** |
| P6 | Local hardware vend | Device GPIO/MDB | — |
| P7a | Success | `ConfirmVendSuccess` / `ReportVendSuccess` / `CompleteVend` (alias family) | **Required** |
| P7b | Failure | `ReportVendFailure` / `FailVend` | **Required** |
| P8 | Cancel | `CancelOrder` / `CancelSale` where applicable | **Required** |

**Payment recovery after app crash:** Persist **`order_id`**, last **`idempotency_key`** used for `CreatePaymentSession`, and PSP **display state** (not secrets). On cold start: (1) load Room; (2) `GetOrder` / `GetOrderStatus`; (3) if **paid** and vend not terminal → resume at **`StartVend`** with **new** vend idempotency key only if server shows vend not started—if uncertain, **block sale UI** until ops confirms (see [kiosk-app-flow.md](kiosk-app-flow.md) payment offline policy).

### 3b. Legacy HTTP (staging / migration only)

When **`ENABLE_LEGACY_MACHINE_HTTP=true`** (not production default), the table below maps to Chi routes; prefer **§3a** on production builds.

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

| Step | Production (gRPC) | Legacy HTTP |
| --- | --- | --- |
| R1 | `ReportVendFailure` / `FailVend` first (domain truth) | `POST .../vend/failure` |
| R2 | Refund via **admin REST** / orchestrator per org policy — **`CancelOrder`** on machine surface if applicable to your proto revision; confirm OpenAPI + proto before GA | `POST /v1/commerce/orders/{orderId}/refunds` or admin workflow |

Until refund routes are on your server, coordinate **manual** refund via ops + admin reads.

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

## 7. Final Android handoff — Room cache, offline queue, security (~TPM sign-off)

Pair with **[`field-test-cases.md`](../testing/field-test-cases.md)** **FT-MED-02**, **FT-OFF-01**, **FT-PAY-04**, **FT-VND-03**.

| # | Area | Implementation requirement | Verify (concrete) |
| --- | --- | --- | --- |
| 7.1 | **Room / local DB catalog cache** | Store snapshot rows: slot/SKU, prices, **`catalog_version`**, **`generated_at`**, **`Meta.server_time`**, currency; index by `(machineId, catalog_version)` | After airplane mode, UI reads **only** Room for catalog; compare hash/version to server after reconnect via **`GetCatalogDelta`**. |
| 7.2 | **Offline event queue** | Durable queue table(s): pending RPC payloads **or** `PushOfflineEvents` envelopes with **`offline_sequence`** monotonic per boot; **`client_event_id`** UUID; persist **`next_attempt_at`**, retry count | **FT-OFF-01**: duplicate `client_event_id` → **REPLAYED**; intentional gap → **Aborted** + clear error string. |
| 7.3 | **Idempotency key generation** | Stable string per **business intent** (order create, payment session, vend start/success/fail); never random per retry; store key in Room **before** first network call | Re-run same flow after process kill → server returns **replay=true** / same outcome (see [machine-grpc.md](machine-grpc.md) ledger). |
| 7.4 | **gRPC auth interceptor** | Attach `authorization: Bearer <machine_access>` on **every** unary call except `ClaimActivation` / `RefreshMachineToken`; validate `401` → single refresh attempt then block UI | grpcurl / integration: call `GetCatalogSnapshot` **without** header → **Unauthenticated**; with Machine JWT → **OK**. |
| 7.5 | **MQTT reconnect** | On disconnect: exponential backoff + **full jitter**; resubscribe command topic(s); pause bursty publish until **session present**; drain HTTP/gRPC outbox **after** stable connection | Broker flap test: 3× 30s outages → no duplicate **critical** side effects; command backlog still processed. |
| 7.6 | **Local media durable file cache** | Disk cache path = f(`media_asset_id`, `checksum_sha256`, `media_version`); atomic write (temp file + rename); LRU cap per product policy | **FT-MED-02**: SHA-256(bytes) **equals** catalog `checksum_sha256`; corrupt file triggers re-download. |
| 7.7 | **Hash verification** | After download, compute SHA-256 before Bitmap decode; on mismatch delete file + retry with backoff; respect **`expires_at`** on signed URLs | Forced bad payload (proxy) → mismatch detected; **no** display of wrong image. |
| 7.8 | **Payment recovery after app crash** | Persist: `order_id`, payment **`idempotency_key`**, last known **order status** enum, `localSaleId`; on resume call **`GetOrder`** before offering “pay again” | **FT-PAY-04** / **FT-VND-03**: never double-charge; if paid + vend unknown → service policy (**block** or **operator**) documented in runbook. |

---

## Acceptance criteria (Android team)

- [ ] Machine id + tokens + catalog + image hash strategy documented in app ADR.
- [ ] Startup order matches **§2**; sale order matches **§3**; offline rules match **§4**.
- [ ] All commerce POSTs send **Idempotency-Key** per §3 table.
- [ ] MQTT outbox uses **dedupe_key** / identity rules from [mqtt-contract.md](mqtt-contract.md).
- [ ] Customer build excludes **admin** base URLs or feature-flags them off.
- [ ] OpenAPI / backend version verified for **target** routes before GA.
- [ ] **§7** final handoff (Room, offline queue, idempotency, gRPC interceptor, MQTT reconnect, media cache, hash verify, payment crash recovery) signed with **FT-*** references where applicable.
