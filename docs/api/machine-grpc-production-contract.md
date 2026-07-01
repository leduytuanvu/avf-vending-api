# Machine gRPC production contract (Android runtime)

**Package:** `avf.machine.v1`  
**Proto root:** [`proto/avf/machine/v1/`](../../proto/avf/machine/v1/)  
**Aggregator import:** [`machine_runtime.proto`](../../proto/avf/machine/v1/machine_runtime.proto)  
**Proto RPC index (generated):** [`android-proto-sync.md`](android-proto-sync.md)  
**Overview / migration:** [`machine-grpc.md`](machine-grpc.md)

This document is the **single production source of truth** for the Android vending app runtime. **gRPC + Machine JWT + MQTT** is primary. Legacy machine REST is **fallback only** and **off by default** in production.

---

## Production policy

| Rule | Detail |
|------|--------|
| **Primary transport** | `grpcs://` Machine gRPC (`MACHINE_GRPC_ENABLED=true`, required when `APP_ENV=production`) |
| **Primary commands** | MQTT TLS subscribe + `commands/ack` publish ([`mqtt-contract.md`](mqtt-contract.md)) |
| **Auth for runtime** | Machine access JWT in metadata: `authorization: Bearer <machine_access_jwt>` |
| **Legacy REST** | Registered only when `ENABLE_LEGACY_MACHINE_HTTP=true`; production default **false**; requires `MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION=true` to enable in prod |
| **Admin JWT** | **Rejected** on all machine runtime RPCs (`Unauthenticated`) |
| **PSP webhooks** | REST only (`POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks`) — not gRPC |

---

## Common envelopes

### `MachineRequestMeta` (reads + optional echo on writes)

| Field | Type | App persistence | Notes |
|-------|------|-----------------|-------|
| `machine_id` | string | — | Must match JWT machine when set |
| `request_id` | string | Optional trace | Server may echo in `MachineResponseMeta` |
| `idempotency_key` | string | **Required** for idempotent mutations (or use `IdempotencyContext`) | Stable per business intent |
| `occurred_at` | timestamp | — | Client event time |
| `client_sequence` | int64 | Monotonic local counter | Diagnostics |
| `offline_sequence` | int64 | Offline queue cursor | Used with offline sync |
| `app_version` | string | — | Kiosk build label |
| `config_version` | int64 | Last acked config | Bootstrap/delta hints |
| `catalog_version` | string | Catalog fingerprint / ETag | Cache invalidation |
| `client_event_id` | string | Durable event UUID | Dedup with offline |

### `IdempotencyContext` (mutations)

| Field | Required | Persistence |
|-------|----------|-------------|
| `idempotency_key` | **Yes** (when `GRPC_REQUIRE_IDEMPOTENCY=true`) | Store in Room **before** first network call; reuse on retry |
| `client_event_id` | Recommended | UUID per mutation attempt lineage |
| `client_created_at` | Recommended | Wall clock |
| `operator_session_id` | When operator flow | From HTTP operator session |

### `MachineResponseMeta`

| Field | Meaning |
|-------|---------|
| `retryable` | Safe to retry same idempotency key |
| `error_code` | Stable machine-facing code when set |
| `status` | `ACCEPTED`, `REPLAYED`, `REJECTED`, `NOT_MODIFIED` |

**Replay flag:** Many mutation responses include top-level `replay=true` on ledger replay (distinct from first execution).

---

## Auth matrix

| RPC group | Machine JWT | Notes |
|-----------|-------------|-------|
| `ClaimActivation`, `RefreshMachineToken` (+ auth service aliases) | **No** | Credentials in request body |
| All other `avf.machine.v1` RPCs | **Yes** | Enforced in `internal/grpcserver/interceptors.go` |
| User / admin JWT on machine RPC | **Rejected** | `Unauthenticated` |

**Lifecycle gates:**

- **Credential gate** (`machineCredentialGate`): blocks `suspended`, `retired`, `maintenance`, `compromised`, etc.
- **Commerce / inventory gate** (`machineRuntimeInventoryGate`): sales and inventory mutations require machine status **`online` or `offline`**.

**Sell readiness (soft gate):** `GetBootstrap` → `RuntimeHints.sell_readiness` (`catalog_synced`, `media_synced`, `inventory_synced`, `ready_for_sale`, `readiness_issues`). App **must not** offer sale UI when `ready_for_sale=false`.

---

## Idempotency & retry

| Behavior | Detail |
|----------|--------|
| **Mutations covered** | Commerce, inventory, telemetry ingest, offline push, bootstrap check-in/ack, catalog/media/inventory acks, OTA/diagnostic reports — see `isMachineIdempotentMutation` in `machine_replay_ledger.go` |
| **Missing key** | `InvalidArgument` / failed precondition when idempotency required |
| **Same key + same payload hash** | Returns stored success response; outer `replay=true` |
| **Same key + different payload** | `FailedPrecondition` / `idempotency_payload_mismatch` |
| **Token refresh / activation** | **Not** ledger-idempotent — use new refresh token rotation semantics |
| **Retry** | Safe on idempotent mutations with **identical** protobuf body; use exponential backoff on `Unavailable` |

---

## Android runtime flow index

Canonical mapping from Android kiosk flows to **primary gRPC RPC** (aliases noted). Legacy REST is never primary.

| Android flow | Primary RPC | Service | Auth | Idempotency | Legacy REST fallback |
|--------------|-------------|---------|------|-------------|----------------------|
| claim activation | `ClaimActivation` | `MachineActivationService` | None | No ledger | `POST /v1/setup/activation-codes/claim` — legacy-allowed |
| refresh machine token | `RefreshMachineToken` | `MachineTokenService` | None | No ledger | None |
| get bootstrap | `GetBootstrap` | `MachineBootstrapService` | Machine JWT | Read | `GET /v1/setup/machines/{id}/bootstrap` — legacy-only |
| check-in | `CheckIn` | `MachineBootstrapService` or `MachineTelemetryService` | Machine JWT | **Yes** | `POST /v1/machines/{id}/check-ins` — legacy-only |
| ack config version | `AckConfigVersion` | `MachineBootstrapService` | Machine JWT | **Yes** | `POST /v1/machines/{id}/config-applies` — legacy-only |
| check for updates | `CheckForUpdates` | `MachineBootstrapService` | Machine JWT | Read | None |
| sync sale catalog | `GetCatalogSnapshot` / `SyncSaleCatalog` | `MachineCatalogService` | Machine JWT | Read | `GET /v1/machines/{id}/sale-catalog` — legacy-only |
| sync catalog bundle | `SyncCatalogBundle` | `MachineCatalogService` | Machine JWT | Read | Same as sale-catalog HTTP — legacy-only |
| get catalog delta | `GetCatalogDelta` | `MachineCatalogService` | Machine JWT | Read | Same — legacy-only |
| ack catalog version | `AckCatalogVersion` | `MachineCatalogService` | Machine JWT | **Yes** | None |
| get media manifest | `GetMediaManifest` | `MachineMediaService` | Machine JWT | Read | Sale-catalog HTTP — legacy-only |
| get media delta | `GetMediaDelta` | `MachineMediaService` | Machine JWT | Read | Same — legacy-only |
| ack media version | `AckMediaVersion` | `MachineMediaService` | Machine JWT | **Yes** | None |
| get inventory snapshot | `GetInventorySnapshot` | `MachineInventoryService` | Machine JWT | Read | Machine runtime HTTP — legacy-only |
| get planogram | `GetPlanogram` | `MachineInventoryService` | Machine JWT | Read | Same — legacy-only |
| create quote | `CreateQuote` | `MachineCommerceService` | Machine JWT + inventory gate | **Yes** | None — multi-line cart pricing |
| create order from quote | `CreateOrderFromQuote` | `MachineCommerceService` | Machine JWT + inventory gate | **Yes** | None — multi-line checkout |
| create order | `CreateOrder` | `MachineCommerceService` | Machine JWT + inventory gate | **Yes** | `POST /v1/commerce/orders` — legacy-only |
| create payment session | `CreatePaymentSession` | `MachineCommerceService` | Machine JWT + inventory gate | **Yes** | `POST /v1/commerce/orders/{id}/payment-session` — legacy-only |
| confirm cash payment | `ConfirmCashPayment` | `MachineCommerceService` | Machine JWT + inventory gate | **Yes** | `POST /v1/commerce/cash-checkout` — legacy-only |
| create cash checkout | `CreateCashCheckout` | `MachineCommerceService` | Machine JWT + inventory gate | **Yes** (alias) | Same cash-checkout HTTP — legacy-only |
| get order status | `GetOrderStatus` / `GetOrder` | `MachineCommerceService` | Machine JWT | Read | `GET /v1/commerce/orders/{id}` — legacy-only |
| start vend | `StartVend` | `MachineCommerceService` | Machine JWT + inventory gate | **Yes** | `POST .../vend/start` — legacy-only |
| report vend success | `ReportVendSuccess` / `ConfirmVendSuccess` | `MachineCommerceService` | Machine JWT | **Yes** | `POST .../vend/success` — legacy-only |
| report vend failure | `ReportVendFailure` | `MachineCommerceService` | Machine JWT | **Yes** | `POST .../vend/failure` — legacy-only |
| cancel order | `CancelOrder` | `MachineCommerceService` | Machine JWT | **Yes** | `POST .../cancel` — legacy-only |
| push telemetry batch | `SubmitTelemetryBatch` / `PushTelemetryBatch` | `MachineTelemetryService` | Machine JWT | **Yes** | Device reconcile HTTP — legacy-only |
| push critical event | `PushCriticalEvent` | `MachineTelemetryService` | Machine JWT | **Yes** | None (MQTT parallel) |
| push offline events | `PushOfflineEvents` | `MachineOfflineSyncService` | Machine JWT | **Yes** | None |
| get sync cursor | `GetSyncCursor` | `MachineOfflineSyncService` | Machine JWT | Read | None |
| submit refill / stock adjustment | `SubmitFillReport`, `SubmitStockAdjustment` | `MachineOperatorService` → inventory | Machine JWT + inventory gate | **Yes** | Operator HTTP session required first — legacy-only |

**Not on gRPC (by design):** command poll/ack (`GetPendingCommands`, `AckCommand`, `RejectCommand`) → **MQTT**; operator session open/close/login → **HTTP PIN** (`OpenOperatorSession` returns `Unimplemented`).

---

## Flow reference (Android primary path)

Legend — **Legacy REST fallback:** `legacy-only` = not mounted in production default; `legacy-allowed` = mounted only if legacy HTTP explicitly enabled.

### 1. Activation / claim

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineActivationService.ClaimActivation` (preferred) or `MachineAuthService.ClaimActivation` |
| **Request** | `activation_code`, `device_fingerprint` (`android_id`, `serial_number`, `manufacturer`, `model`, `package_name`, `version_name`, `version_code`) |
| **Response** | `machine_id`, `site_id`, `access_token`, `refresh_token`, expiries, `mqtt_broker_url`, `mqtt_topic_prefix`, `bootstrap_required` |
| **Auth** | None |
| **Idempotency** | No ledger — use fresh activation code per install |
| **Persistence** | Secure storage: refresh token, machine_id, MQTT config |
| **Errors** | Invalid/expired code → `InvalidArgument` / `NotFound`; rate limit → `ResourceExhausted` |
| **Legacy REST** | `POST /v1/setup/activation-codes/claim` — **legacy-allowed** (public REST mirror); gRPC preferred |

### 2. Token refresh

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineTokenService.RefreshMachineToken` |
| **Request** | `refresh_token` |
| **Response** | New `access_token`, rotated `refresh_token`, expiries, MQTT hints |
| **Auth** | None (opaque refresh in body) |
| **Idempotency** | No — each call may rotate refresh token |
| **Persistence** | Replace stored tokens atomically |
| **Retry** | On network failure before response, retry same refresh once; if rotated server-side, use new refresh from last success |
| **Legacy REST** | None |

### 3. Bootstrap

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineBootstrapService.GetBootstrap` |
| **Request** | `MachineRequestMeta` (optional) |
| **Response** | `BootstrapMachine`, topology/planogram slots, catalog product list, fingerprints, `MqttConfigMetadata`, `RuntimeHints.sell_readiness`, `published_planogram_version_*` |
| **Auth** | Machine JWT + credential gate |
| **Idempotency** | Read-only |
| **Persistence** | Cache bootstrap blob + fingerprints locally |
| **Legacy REST** | `GET /v1/setup/machines/{machineId}/bootstrap` — **legacy-only** |

### 4. Check-in

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineBootstrapService.CheckIn` or `MachineTelemetryService.CheckIn` |
| **Request** | `boot_id`, `network_state`, attributes map, `IdempotencyContext` |
| **Response** | `replay`, connectivity snapshot |
| **Auth** | Machine JWT |
| **Idempotency** | **Yes** |
| **Persistence** | Store last successful check-in idempotency key |
| **Legacy REST** | `POST /v1/machines/{id}/check-ins` — **legacy-only** |

### 5. Config ACK

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineBootstrapService.AckConfigVersion` |
| **Request** | `acknowledged_config_version`, optional `acknowledged_planogram_version_id`, `IdempotencyContext` |
| **Response** | `replay`, ack confirmation |
| **Auth** | Machine JWT |
| **Idempotency** | **Yes** |
| **Persistence** | Persist last acked config/planogram version |
| **Legacy REST** | `POST /v1/machines/{id}/config-applies` — **legacy-only** |

### 5b. Check for updates (lightweight)

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineBootstrapService.CheckForUpdates` |
| **Request** | Local fingerprints: `catalog_fingerprint`, `pricing_fingerprint`, `planogram_fingerprint`, `media_fingerprint` |
| **Response** | `catalog_changed`, `pricing_changed`, `planogram_changed`, `media_changed`, `firmware_or_app_update_available`, server-side fingerprints |
| **Auth** | Machine JWT + credential gate |
| **Idempotency** | Read-only |
| **Retry** | Safe; use between full sync cycles to avoid heavy bootstrap |
| **Persistence** | Compare server fingerprints to cached values; trigger delta RPCs when `*_changed=true` |
| **Errors** | `Unauthenticated` (disabled/suspended), `NotFound` (machine) |
| **Legacy REST** | None |

### 6. Catalog snapshot / delta

| Item | Value |
|------|-------|
| **Primary RPCs** | `GetCatalogSnapshot` / `SyncSaleCatalog` (aliases), `SyncCatalogBundle`, `GetCatalogDelta`, `AckCatalogVersion` |
| **Key request fields** | `include_unavailable`, `include_images`, `if_none_match_config_version`, `basis_catalog_version` (delta) |
| **Key response fields** | `catalog_version` (runtime fingerprint), line items, `ProductMediaRef`, `removed_product_ids` (delta/bundle) |
| **Auth** | Machine JWT |
| **Idempotency** | Reads: no; `AckCatalogVersion`: **yes** |
| **Persistence** | Room catalog cache keyed by `catalog_version`; ack after successful apply |
| **Legacy REST** | `GET /v1/machines/{id}/sale-catalog` — **legacy-only** |

### 7. Media manifest / delta

| Item | Value |
|------|-------|
| **Primary RPCs** | `MachineMediaService.GetMediaManifest`, `GetMediaDelta`, `AckMediaVersion` |
| **Key fields** | `media_fingerprint`, `ProductMediaRef` (url, checksum, media_version, deleted tombstone) |
| **Auth** | Machine JWT |
| **Idempotency** | Ack: **yes** |
| **Persistence** | Offline image cache keyed by `checksum_sha256` + `media_version` |
| **Errors** | Oversized manifest → `ResourceExhausted` |
| **Legacy REST** | Sale-catalog HTTP — **legacy-only** |

### 8. Inventory snapshot / planogram

| Item | Value |
|------|-------|
| **Primary RPCs** | `GetInventorySnapshot`, `GetPlanogram`, `AckInventorySync`, mutations (`SubmitFillResult`, `SubmitInventoryAdjustment`, …) |
| **Auth** | Machine JWT + **runtime inventory gate** (online/offline) |
| **Idempotency** | Mutations: **yes** |
| **Persistence** | Local slot quantities; ack cursor |
| **Legacy REST** | Machine runtime HTTP — **legacy-only** |

### 9. Create quote (multi-line cart)

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.CreateQuote` |
| **Request** | `IdempotencyContext`, `lines[]` (product, slot), `currency`, optional `payment_method`, `machine_id` |
| **Response** | `quote_id`, line pricing, `payable_minor`, `expires_at`, `replay` |
| **Auth** | Machine JWT + runtime inventory gate |
| **Idempotency** | **Yes** |
| **Persistence** | Persist quote snapshot until consumed or expired |
| **Errors** | Empty lines → `InvalidArgument`; suspended/disabled → `PermissionDenied` |
| **Legacy REST** | None — **gRPC-only** |

### 9b. Create order from quote

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.CreateOrderFromQuote` |
| **Request** | `IdempotencyContext`, `quote_id`, optional `payment_method`, `machine_id` |
| **Response** | `order_id`, `order_status`, per-line `vend_session_id`, `line_sequence`, `replay` |
| **Auth** | Machine JWT + runtime inventory gate |
| **Idempotency** | **Yes** |
| **Persistence** | Consumes quote; persist `order_id` + idempotency key until terminal order state |
| **Errors** | Expired/consumed quote → `FailedPrecondition`; suspended/disabled → `PermissionDenied` |
| **Legacy REST** | None — **gRPC-only** |

### 10. Create order

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.CreateOrder` |
| **Request** | `IdempotencyContext`, `product_id`, `SlotSelection`, `currency`, optional `machine_id` |
| **Response** | `order_id`, `vend_session_id`, totals, `order_status`, `vend_state`, `replay` |
| **Auth** | Machine JWT + runtime inventory gate |
| **Idempotency** | **Yes** |
| **Persistence** | Persist `order_id` + idempotency key until terminal order state |
| **Errors** | Suspended machine → `PermissionDenied`; not online/offline → `PermissionDenied` |
| **Legacy REST** | `POST /v1/commerce/orders` — **legacy-only** |

### 10. Create payment session (QR/card)

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.CreatePaymentSession` |
| **Request** | `order_id`, `amount_minor`, `currency`, optional `provider` (must match `COMMERCE_PAYMENT_PROVIDER`) |
| **Response** | `payment_id`, `qr_payload_or_url`, `payment_state`, `replay` |
| **Auth** | Machine JWT + runtime inventory gate |
| **Idempotency** | **Yes** |
| **Persistence** | Store `payment_id`; poll `GetOrderStatus` after PSP webhook |
| **Errors** | Live PSP not wired → `FailedPrecondition` / provider error |
| **Legacy REST** | `POST /v1/commerce/orders/{id}/payment-session` — **legacy-only** (client `outbox_payload_json` — **do not use in new apps**) |

### 11. Confirm cash payment

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.ConfirmCashPayment` / `CreateCashCheckout` |
| **Request** | `order_id`, `IdempotencyContext` |
| **Response** | Payment + order state, `replay` |
| **Auth** | Machine JWT + runtime inventory gate |
| **Idempotency** | **Yes** |
| **Legacy REST** | `POST /v1/commerce/cash-checkout` — **legacy-only** |

### 11b. Get order status

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.GetOrderStatus` (alias: `GetOrder`) |
| **Request** | `order_id` |
| **Response** | `order_status`, `vend_state`, payment fields, timestamps |
| **Auth** | Machine JWT |
| **Idempotency** | Read-only |
| **Retry** | Safe; poll after PSP webhook or cash confirm |
| **Persistence** | Cache last polled status per active `order_id` |
| **Errors** | `NotFound` (unknown order), `PermissionDenied` (wrong machine scope) |
| **Legacy REST** | `GET /v1/commerce/orders/{id}` — **legacy-only** |

### 12. Start vend

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.StartVend` |
| **Request** | `order_id`, `slot_index`, `IdempotencyContext` |
| **Response** | `vend_state` (`in_progress`), `replay` |
| **Auth** | Machine JWT + runtime inventory gate |
| **Idempotency** | **Yes** |
| **Errors** | Unpaid order → failed precondition (see contract test `StartVend_BlockedBeforePayment`) |
| **Legacy REST** | `POST .../vend/start` — **legacy-only** |

### 13. Report vend success

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.ReportVendSuccess` / `ConfirmVendSuccess` |
| **Request** | `order_id`, `slot_index`, optional `correlation_id`, `IdempotencyContext` |
| **Response** | `order_status`, `vend_state`, inventory replay flags |
| **Auth** | Machine JWT |
| **Idempotency** | **Yes** |
| **Ordering** | **Requires prior `StartVend`** — otherwise orchestrator rejects |
| **Legacy REST** | `POST .../vend/success` — **legacy-only** |

### 14. Report vend failure

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.ReportVendFailure` |
| **Request** | `order_id`, `slot_index`, `failure_reason`, `IdempotencyContext` |
| **Response** | Terminal failure state, refund workflow flags |
| **Auth** | Machine JWT |
| **Idempotency** | **Yes** |
| **Ordering** | **Requires prior `StartVend`** |
| **Legacy REST** | `POST .../vend/failure` — **legacy-only** |

### 14b. Cancel order

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineCommerceService.CancelOrder` |
| **Request** | `order_id`, `IdempotencyContext`, optional cancel reason |
| **Response** | Terminal cancelled state, `replay` |
| **Auth** | Machine JWT |
| **Idempotency** | **Yes** |
| **Retry** | Same idempotency key on network failure |
| **Persistence** | Mark local order terminal; retain idempotency key until ack |
| **Errors** | Already terminal → idempotent replay or failed precondition |
| **Legacy REST** | `POST /v1/commerce/orders/{id}/cancel` — **legacy-only** |

### 15. Telemetry batch

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineTelemetryService.SubmitTelemetryBatch` / `PushTelemetryBatch` |
| **Request** | Batch of events with types/attributes, `IdempotencyContext` |
| **Response** | Accepted/replayed counts |
| **Auth** | Machine JWT |
| **Idempotency** | **Yes** (batch-level) |
| **Persistence** | Outbox until `REPLAYED` / reconcile |
| **Legacy REST** | Device reconcile HTTP — **legacy-only**; prefer gRPC `ReconcileEvents` |

### 16. Critical event

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineTelemetryService.PushCriticalEvent` |
| **Request** | Severity, event type, attributes, `IdempotencyContext` |
| **Auth** | Machine JWT |
| **Idempotency** | **Yes** |
| **MQTT parallel** | Publish to `telemetry/incident` or critical topics per [`mqtt-contract.md`](mqtt-contract.md) |

### 17. Offline sync

| Item | Value |
|------|-------|
| **Primary RPC** | `MachineOfflineSyncService.PushOfflineEvents`, `GetSyncCursor` |
| **GetSyncCursor** | Read-only; returns server acked `offline_sequence` for reconciliation |
| **Request** | Ordered events with `offline_sequence`, typed payloads |
| **Response** | Per-event `ACCEPTED` / `REPLAYED` / `REJECTED` |
| **Auth** | Machine JWT |
| **Idempotency** | **Yes** (per event + sequence monotonicity) |
| **Errors** | Out-of-order sequence → `Aborted` |
| **Persistence** | Durable offline queue until cursor advanced |

### 18. Operator refill / stock adjustment

| Item | Value |
|------|-------|
| **Primary RPCs** | `MachineOperatorService.SubmitFillReport`, `SubmitStockAdjustment` (delegate to inventory) |
| **Session open/close** | **HTTP only** — `OpenOperatorSession` / `CloseOperatorSession` return `Unimplemented` on gRPC |
| **HTTP operator** | `POST /v1/machines/{id}/operator-sessions/start|end` — **legacy-only** but **required** for human PIN proof |
| **Auth** | Machine JWT + runtime inventory gate |
| **Idempotency** | **Yes** |

### 19. OTA / diagnostics (command service)

| Item | Value |
|------|-------|
| **Primary RPCs** | `MachineCommandService.GetAssignedUpdate`, `ReportUpdateStatus`, `ReportDiagnosticBundleResult` |
| **Deprecated (Unimplemented)** | `GetPendingCommands`, `AckCommand`, `RejectCommand` — use **MQTT** |
| **Auth** | Machine JWT |
| **Idempotency** | Report RPCs: **yes** |

---

## RPC reference (production Android)

Per-RPC contract for `avf.machine.v1`. **Auth** = Machine JWT unless noted. **Idempotency** = ledger replay when `GRPC_REQUIRE_IDEMPOTENCY=true`. **REST fallback** is never primary in production.

### MachineActivationService / MachineTokenService / MachineAuthService

| RPC | Purpose | Request (key fields) | Response (key fields) | Auth | Idempotency | Retry | Local persistence | Errors | REST fallback | Fallback prod |
|-----|---------|----------------------|------------------------|------|-------------|-------|-------------------|--------|---------------|-----------------|
| `ClaimActivation` | Bind device to machine | `activation_code`, `device_fingerprint` | tokens, `machine_id`, MQTT hints | None | No | New code per install | Secure token store | Invalid code, rate limit | `POST /v1/setup/activation-codes/claim` | legacy-allowed |
| `RefreshMachineToken` | Rotate access token | `refresh_token` | new access + refresh | None | No | Once on network fail | Atomic token replace | Revoked refresh | None | — |
| `ActivateMachine`, auth `ClaimActivation`/`RefreshMachineToken` | Compatibility aliases | Same as above | Same | None | No | Same | Same | Same | Same | legacy-allowed |

### MachineBootstrapService

| RPC | Purpose | Request | Response | Auth | Idempotency | Retry | Persistence | Errors | REST | Fallback |
|-----|---------|---------|----------|------|-------------|-------|-------------|--------|------|----------|
| `GetBootstrap` | Full runtime config | optional meta | machine, catalog, MQTT, sell readiness | JWT | Read | Safe | Cache blob + fingerprints | disabled/suspended | `GET .../bootstrap` | legacy-only |
| `CheckIn` | Boot / connectivity | `boot_id`, meta, idempotency | replay, snapshot | JWT | **Yes** | Same key | Last check-in key | invalid meta | `POST .../check-ins` | legacy-only |
| `AckConfigVersion` | Confirm config apply | version ids, idempotency | replay | JWT | **Yes** | Same key | Last acked versions | version mismatch | `POST .../config-applies` | legacy-only |
| `CheckForUpdates` | Lightweight change probe | local fingerprints | changed flags + server fingerprints | JWT | Read | Safe | Fingerprints | credential gate | None | — |

### MachineCatalogService / MachineMediaService

| RPC | Purpose | Request | Response | Auth | Idempotency | Retry | Persistence | Errors | REST | Fallback |
|-----|---------|---------|----------|------|-------------|-------|-------------|--------|------|----------|
| `GetCatalogSnapshot`, `SyncSaleCatalog`, `GetSaleCatalog` | Full catalog | filters, etag | products, `catalog_version` | JWT | Read | Safe | Room catalog | too large | sale-catalog GET | legacy-only |
| `SyncCatalogBundle` | Bundle sync | basis version | bundle + removals | JWT | Read | Safe | Catalog cache | — | sale-catalog GET | legacy-only |
| `GetCatalogDelta` | Incremental catalog | basis version | delta items, removals | JWT | Read | Safe | Merge delta | — | sale-catalog GET | legacy-only |
| `AckCatalogVersion` | Confirm catalog apply | version, idempotency | replay | JWT | **Yes** | Same key | Acked version | — | None | — |
| `GetMediaManifest` | Media index | fingerprint | refs + urls | JWT | Read | Safe | Media index | manifest size | sale-catalog GET | legacy-only |
| `GetMediaDelta` | Incremental media | basis fingerprint | added/changed/removed | JWT | Read | Safe | Image cache | — | sale-catalog GET | legacy-only |
| `AckMediaVersion` | Confirm media apply | version, idempotency | replay | JWT | **Yes** | Same key | Acked media version | — | None | — |

### MachineInventoryService

| RPC | Purpose | Request | Response | Auth | Idempotency | Retry | Persistence | Errors | REST | Fallback |
|-----|---------|---------|----------|------|-------------|-------|-------------|--------|------|----------|
| `GetInventorySnapshot` | Slot quantities | meta | slots, quantities | JWT + gate | Read | Safe | Local inventory | not operational | runtime HTTP | legacy-only |
| `GetPlanogram` | Layout | meta | slots, products | JWT | Read | Safe | Planogram cache | — | runtime HTTP | legacy-only |
| `AckInventorySync` | Confirm inventory sync | cursor, idempotency | replay | JWT | **Yes** | Same key | Sync cursor | — | None | — |
| `SubmitFillReport`, `SubmitFillResult`, `SubmitStockAdjustment`, … | Refill / adjust | deltas, idempotency | replay | JWT + gate | **Yes** | Same key | Updated slots | gate fail | operator HTTP | legacy-only |

### MachineCommerceService / MachineSaleService

| RPC | Purpose | Request | Response | Auth | Idempotency | Retry | Persistence | Errors | REST | Fallback |
|-----|---------|---------|----------|------|-------------|-------|-------------|--------|------|----------|
| `CreateQuote` | Price multi-line cart | lines, currency, idempotency | quote snapshot | JWT + gate | **Yes** | Same key | quote + key | suspended/disabled | None | gRPC-only |
| `CreateOrderFromQuote` | Checkout from quote | quote_id, idempotency | order + vend lines | JWT + gate | **Yes** | Same key | order + key | quote expired | None | gRPC-only |
| `CreateOrder` | Start checkout | product, slot, idempotency | `order_id`, status | JWT + gate | **Yes** | Same key | order + key | suspended/disabled | orders POST | legacy-only |
| `CreatePaymentSession` | QR/card session | order, amount | payment id, QR | JWT + gate | **Yes** | Same key | payment id | PSP not live | payment-session POST | legacy-only |
| `ConfirmCashPayment`, `CreateCashCheckout` | Cash paid | order, idempotency | paid state | JWT + gate | **Yes** | Same key | payment state | unpaid order | cash-checkout POST | legacy-only |
| `GetOrderStatus`, `GetOrder` | Poll order | order_id | status, vend state | JWT | Read | Safe | cached status | not found | orders GET | legacy-only |
| `StartVend` | Motor start | order, slot, idempotency | in_progress | JWT + gate | **Yes** | Same key | vend state | unpaid | vend/start POST | legacy-only |
| `ReportVendSuccess`, `ConfirmVendSuccess` | Vend OK | order, slot, idempotency | completed | JWT | **Yes** | Same key | terminal | before StartVend | vend/success POST | legacy-only |
| `ReportVendFailure` | Vend fail | order, reason, idempotency | failed | JWT | **Yes** | Same key | terminal | before StartVend | vend/failure POST | legacy-only |
| `CancelOrder` | Abort checkout | order, idempotency | cancelled | JWT | **Yes** | Same key | terminal | terminal replay | cancel POST | legacy-only |
| `MachineSaleService.*` | Naming alias | Same shapes | Same | JWT + gate | **Yes** | Same | Same | Same | Same | legacy-only |

### MachineTelemetryService / MachineOfflineSyncService

| RPC | Purpose | Request | Response | Auth | Idempotency | Retry | Persistence | Errors | REST | Fallback |
|-----|---------|---------|----------|------|-------------|-------|-------------|--------|------|----------|
| `SubmitTelemetryBatch`, `PushTelemetryBatch` | Metrics batch | events, idempotency | counts, replay | JWT | **Yes** | Same key | Outbox | — | reconcile HTTP | legacy-only |
| `PushCriticalEvent` | Incident | severity, idempotency | ack | JWT | **Yes** | Same key | Outbox | — | None | — |
| `CheckIn` (telemetry) | Alias check-in | same as bootstrap | same | JWT | **Yes** | Same key | Same | — | check-ins POST | legacy-only |
| `PushOfflineEvents` | Offline queue drain | events + sequence | per-event status | JWT | **Yes** | Same key | Queue until ack | sequence gap | None | — |
| `GetSyncCursor` | Server offline cursor | meta | `offline_sequence` | JWT | Read | Safe | Reconcile pointer | — | None | — |

### MachineOperatorService / MachineCommandService

| RPC | Purpose | Request | Response | Auth | Idempotency | Retry | Persistence | Errors | REST | Fallback |
|-----|---------|---------|----------|------|-------------|-------|-------------|--------|------|----------|
| `SubmitFillReport`, `SubmitStockAdjustment` | Operator refill | fill/adjust + idempotency | replay | JWT + gate | **Yes** | Same key | inventory | gate | operator HTTP | legacy-only |
| `OpenOperatorSession`, `CloseOperatorSession`, `LoginOperator`, `LogoutOperator` | PIN session | — | — | — | — | — | — | **Unimplemented** | operator session HTTP | legacy-only (required) |
| `GetAssignedUpdate`, `ReportUpdateStatus`, `ReportDiagnosticBundleResult` | OTA / diagnostics | update ids, idempotency | status | JWT | reports **Yes** | Same key | OTA state | — | None | — |
| `GetPendingCommands`, `AckCommand`, `RejectCommand` | Command poll | — | — | — | — | — | — | **Unimplemented** | None | use **MQTT** |

---

## Services (proto registration)

All services below are registered when machine gRPC starts. See [`android-proto-sync.md`](android-proto-sync.md) for the full RPC list.

| Service | Production role |
|---------|-----------------|
| `MachineActivationService` | **Primary** activation |
| `MachineTokenService` | **Primary** token refresh |
| `MachineAuthService` | Compatibility aliases for activation/refresh |
| `MachineBootstrapService` | **Primary** bootstrap / check-in / config ack |
| `MachineCatalogService` | **Primary** catalog |
| `MachineMediaService` | **Primary** media offline cache |
| `MachineInventoryService` | **Primary** inventory |
| `MachineCommerceService` | **Primary** checkout |
| `MachineSaleService` | Alias naming — same orchestrator |
| `MachineTelemetryService` | **Primary** telemetry |
| `MachineOfflineSyncService` | **Primary** offline replay |
| `MachineOperatorService` | **Partial** — fill/adjust only |
| `MachineCommandService` | **Partial** — OTA/diagnostics only |

---

## Error handling (app)

| gRPC code | Typical cause | App action |
|-----------|---------------|------------|
| `Unauthenticated` | Missing/expired/wrong JWT | Refresh token; re-activate if refresh fails |
| `PermissionDenied` | Scope mismatch, suspended machine, wrong machine_id | Stop sale; show operator message |
| `FailedPrecondition` | Idempotency mismatch, vend ordering, unpaid | Fix payload or advance state machine |
| `Aborted` | Offline sequence gap | Rewind/resend offline queue |
| `ResourceExhausted` | Rate limit / manifest too large | Backoff; reduce batch size |
| `Unavailable` | Dependency down | Retry with backoff (same idempotency key) |
| `Unimplemented` | Deprecated command polling RPC | Switch to MQTT |

Structured details: `GrpcErrorDetail` in responses for deprecated paths (points to MQTT docs).

---

## Proto compatibility

| Gate | Command |
|------|---------|
| Lint + breaking changes | `make proto-check` (`buf lint`, `scripts/ci/check_proto_breaking.py`, generated `.pb.go` drift) |
| Android sync index | `python scripts/ci/generate_android_proto_sync_doc.py` |
| Doc coverage | `python scripts/ci/check_machine_grpc_production_contract.py` |

**Breaking field renumbering** is rejected by Buf FILE breaking rules (`proto/buf.yaml`). Android must pin to the same proto revision as the server release.

---

## Contract tests (Go)

| Test | File |
|------|------|
| All runtime RPCs require Machine JWT (except activation/refresh) | `machine_grpc_production_contract_test.go` |
| No token rejected | `machine_grpc_production_contract_test.go`, `machine_grpc_auth_test.go` |
| Admin user JWT rejected | `machine_grpc_production_contract_test.go`, `machine_grpc_auth_test.go` |
| Valid machine token accepted | `machine_grpc_production_contract_test.go`, `machine_grpc_auth_test.go` |
| Idempotency replay + mismatch | `machine_grpc_production_contract_test.go`, `machine_replay_ledger_integration_test.go` |
| Idempotent retry returns same result | `machine_grpc_production_contract_test.go`, `machine_commerce_grpc_integration_test.go` |
| Suspended / disabled (maintenance) machine cannot sell | `machine_grpc_production_contract_test.go` |
| Bootstrap sell readiness hints | `machine_grpc_production_contract_test.go` |
| Vend success/failure before StartVend blocked | `machine_grpc_production_contract_test.go` |
| Required mutating RPCs ledger-idempotent | `machine_grpc_production_contract_test.go` |

Run:

```bash
go test ./internal/grpcserver/... -run MachineProductionContract
go test ./internal/grpcserver/... -run Machine
go test ./internal/grpcserver/... -run Idempotency
```

---

## Related audits

- Android gap analysis: [`../archive/audits/audit/BACKEND_PRODUCTION_APP_CONTRACT_AUDIT.md`](../archive/audits/audit/BACKEND_PRODUCTION_APP_CONTRACT_AUDIT.md)
- Transport boundary: [`../architecture/transport-boundary.md`](../architecture/transport-boundary.md)
