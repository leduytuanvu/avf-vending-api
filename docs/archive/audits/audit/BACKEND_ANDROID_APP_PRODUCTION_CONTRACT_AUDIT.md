# Backend ↔ Android app production contract audit

**Date:** 2026-05-29  
**Repository:** `avf-vending-api`  
**Git SHA:** `7035ee229938663ae14847a7a41fb7a356249ad5`  
**Branch:** `chore/clean-project-nonessential-files`  
**Scope:** Lock the **canonical production contract** the Android vending app must implement. **No runtime code changed** in this audit pass — documentation and verification only.

**Normative references**

| Doc | Role |
|-----|------|
| [`docs/api/machine-grpc-production-contract.md`](../api/machine-grpc-production-contract.md) | 19 Android primary flows (single source of truth) |
| [`docs/api/android-proto-sync.md`](../api/android-proto-sync.md) | Generated RPC index (88 RPCs) |
| [`docs/api/mqtt-contract.md`](../api/mqtt-contract.md) | Enterprise MQTT tree + ACK contract |
| [`docs/payments/PRODUCTION_PAYMENT_PROVIDER_STATUS.md`](../payments/PRODUCTION_PAYMENT_PROVIDER_STATUS.md) | Cash-only vs live PSP |
| [`docs/api/kiosk-app-implementation-checklist.md`](../api/kiosk-app-implementation-checklist.md) | Android migration checklist |
| [`docs/release/BACKEND_MARKET_READY_REPORT.md`](../release/BACKEND_MARKET_READY_REPORT.md) | Latest readiness verdict |

**Related audit:** [`BACKEND_PRODUCTION_APP_CONTRACT_AUDIT.md`](BACKEND_PRODUCTION_APP_CONTRACT_AUDIT.md) (2026-05-28, broader backend surface).

---

## Executive verdict

| Verdict | **GO-CANARY-ONLY** (cash-only pilot) |
|---------|--------------------------------------|
| Paid QR/card market launch | **NO-GO** (no wired live PSP) |
| Android still on legacy REST checkout/catalog | **NO-GO** for production default (legacy HTTP **404/disabled** unless explicitly enabled) |

### What this means

| Path | Backend readiness | Android requirement |
|------|-------------------|-------------------|
| **Cash-only canary** on marked test machine | **GO-CANARY-ONLY** — gRPC commerce + MQTT + JWT enforced in code; tests pass | Migrate off legacy REST; use `MachineCommerceService` + enterprise MQTT |
| **Fleet cash rollout** | Blocked until production canary E2E evidence + deploy of contract fixes | Same migration + field QA |
| **Live QR/wallet (MoMo, ZaloPay, VNPay, Stripe)** | **NO-GO** — placeholder adapters only | Do not expose QR/card UI until bootstrap `qr_card_enabled=true` |

**Do not claim market-ready** without: deployed backend at this contract, full production readonly smoke (gRPC + admin + machine token), and guarded canary live-sale artifact under `reports/e2e/`.

---

## Validation run (this audit)

| Command | Environment | Result |
|---------|-------------|--------|
| `go test ./...` | Local (Windows), 2026-05-29 | **PASS** (exit 0) |
| `bash scripts/e2e/production-readonly-smoke.sh` | `https://api.ldtv.dev` | **PARTIAL** — HTTP health/version PASS; gRPC/admin/payment probes SKIP (credentials unset). Artifact: `reports/e2e/production-readonly-smoke/20260529T175143Z/` |
| `bash scripts/e2e/production-canary-live-sale.sh` | Production | **NOT RUN** — requires `PRODUCTION_LIVE_TEST_CONFIRMATION` + canary machine env |

---

## Canonical production app flow

Production Android builds **must** follow this transport stack. Legacy REST is **not** mounted when `ENABLE_LEGACY_MACHINE_HTTP=false` (production default).

```mermaid
sequenceDiagram
  participant App as Android kiosk
  participant gRPC as avf.machine.v1 (grpcs)
  participant MQTT as MQTT broker (TLS)
  participant API as AVF API / ingest

  App->>gRPC: ClaimActivation (no JWT)
  gRPC-->>App: machine JWT, refresh, mqtt broker/prefix/layout
  App->>gRPC: RefreshMachineToken (as needed)
  App->>gRPC: GetBootstrap + catalog/media/inventory RPCs
  App->>MQTT: Subscribe …/machines/{id}/commands
  App->>MQTT: Publish heartbeat, events, commands/ack
  App->>gRPC: CreateOrder → ConfirmCashPayment → StartVend → ConfirmVendSuccess
  App->>gRPC: PushOfflineEvents (after reconnect)
  Note over App,API: QR/card: CreatePaymentSession only when qr_card_enabled; PSP webhooks hit REST (not app)
```

### Phase map (strict order)

| Phase | Canonical (production) | Auth |
|-------|------------------------|------|
| 1. Provision | `MachineActivationService.ClaimActivation` | None |
| 2. Token | `MachineTokenService.RefreshMachineToken` | Refresh token in body |
| 3. Bootstrap | `MachineBootstrapService.GetBootstrap`, `CheckForUpdates`, `CheckIn`, `AckConfigVersion` | Machine JWT |
| 4. Catalog / media | `MachineCatalogService.*`, `MachineMediaService.*` | Machine JWT |
| 5. Inventory | `MachineInventoryService.GetInventorySnapshot`, `GetPlanogram`, mutations when restocking | Machine JWT |
| 6. MQTT | TLS connect; subscribe command topic; publish device channels | Broker ACL + machine credential |
| 7. Cash sale | `MachineCommerceService`: `CreateOrder` → `ConfirmCashPayment` → `StartVend` → `ConfirmVendSuccess` | Machine JWT; machine status `online` or `offline` |
| 8. QR/card sale (future) | `CreatePaymentSession` → poll `GetOrderStatus`; PSP webhook on REST | Only when `payment_methods.qr_card_enabled=true` |
| 9. Offline | Durable local queue → `MachineOfflineSyncService.PushOfflineEvents` + MQTT replay | Machine JWT |
| 10. Operator refill | `MachineInventoryService` fill/adjust RPCs; **HTTP** operator session PIN (gRPC session RPCs unimplemented) | Machine JWT + active operator session |

**Sell readiness gate:** Do not offer sale UI when `GetBootstrap` → `runtime_hints.sell_readiness.ready_for_sale=false`.

---

## RPCs the Android app must call

Proto root: `proto/avf/machine/v1/`. Sync index: [`android-proto-sync.md`](../api/android-proto-sync.md).

### Required (production primary)

| Service | RPCs | Android use |
|---------|------|-------------|
| `MachineActivationService` | `ClaimActivation` | First boot / reinstall |
| `MachineTokenService` | `RefreshMachineToken` | Access token rotation |
| `MachineBootstrapService` | `GetBootstrap`, `CheckForUpdates`, `CheckIn`, `AckConfigVersion` | Runtime config, MQTT metadata (`broker_url`, `topic_prefix`, `topic_layout`), payment_methods |
| `MachineCatalogService` | `GetCatalogSnapshot`, `GetCatalogDelta`, `SyncCatalogBundle`, `AckCatalogVersion` | Sale catalog (replaces HTTP sale-catalog) |
| `MachineMediaService` | `GetMediaManifest`, `GetMediaDelta`, `AckMediaVersion` | Product media offline cache |
| `MachineInventoryService` | `GetInventorySnapshot`, `GetPlanogram`, fill/adjust/restock RPCs | Stock truth + operator flows |
| `MachineCommerceService` | `CreateOrder`, `ConfirmCashPayment`, `StartVend`, `ConfirmVendSuccess`, `ReportVendFailure`, `CancelOrder`, `GetOrder`, `GetOrderStatus` | Checkout lifecycle |
| `MachineCommerceService` | `CreatePaymentSession` | QR/card only when enabled (currently returns `FailedPrecondition` / `provider_unavailable` in cash-only prod) |
| `MachineOfflineSyncService` | `PushOfflineEvents`, `GetSyncCursor` | Post-outage replay |
| `MachineTelemetryService` | `PushTelemetryBatch`, `PushCriticalEvent`, `ReconcileEvents`, `GetEventStatus` | Batch/critical telemetry; optional HTTP-equivalent reconcile |
| `MachineCommandService` | `GetAssignedUpdate`, `ReportUpdateStatus`, `ReportDiagnosticBundleResult` | OTA/diagnostics only |

### Alias services (optional naming — same handlers)

| Service | Notes |
|---------|-------|
| `MachineSaleService` | Mirrors commerce (`CreateSale`, `ConfirmCashReceived`, `CompleteVend`, …) — prefer **`MachineCommerceService`** for new code |
| `MachineAuthService` | Facade duplicating activation/token — prefer dedicated services |

### Must NOT call (returns `Unimplemented` by design)

| RPC | Use instead |
|-----|-------------|
| `MachineCommandService.GetPendingCommands` | MQTT subscribe `{prefix}/machines/{machineId}/commands` |
| `MachineCommandService.AckCommand` | MQTT publish `…/commands/ack` |
| `MachineCommandService.RejectCommand` | MQTT ACK with failed/nacked status |
| `MachineOperatorService.OpenOperatorSession` | HTTP `POST /v1/machines/{machineId}/operator-sessions/login` (legacy-gated) or admin `…/operator-sessions/start` |
| `MachineOperatorService.CloseOperatorSession` | HTTP operator logout/end routes |

Implementation: `internal/grpcserver/machine_contract_grpc.go`, `machine_operator_grpc.go`, `machine_grpc_services.go`.

---

## REST routes: allowed fallback vs legacy/deprecated

**Production gate:** `internal/httpserver/transport_legacy_guard.go` — when `MachineRESTLegacyEnabled=false`, legacy machine routes return **404** `legacy_machine_rest_disabled`.

### Always mounted (not legacy-gated)

| Route | Audience | Android use |
|-------|----------|-------------|
| `POST /v1/setup/activation-codes/claim` | Public | **Allowed fallback** for activation if gRPC unavailable; **prefer gRPC `ClaimActivation`** |
| `POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks` | PSP → cloud | **Not app** — HMAC webhook ingress |
| `GET/POST /v1/admin/*` | Backoffice | **Not app** |
| `GET /v1/machines/{machineId}/commands/{sequence}/status` | Admin/operator JWT | Optional diagnostics — **not primary** |
| `GET /v1/machines/{machineId}/commands/receipts` | Admin/operator JWT | Optional diagnostics |
| `POST /v1/machines/{machineId}/commands/dispatch` | Admin/operator JWT | Fleet dispatch — **not app** |

Router: `internal/httpserver/server.go` — device command routes are **outside** the legacy gate but require **admin/operator** permissions, not machine checkout.

### Legacy / deprecated — gated off in production default

**Android must stop using these.** They are absent on production unless `ENABLE_LEGACY_MACHINE_HTTP=true` **and** `MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION=true`.

| Legacy REST (Android anti-pattern) | Canonical replacement |
|--------------------------------------|------------------------|
| `GET /v1/setup/machines/{machineId}/bootstrap` | `MachineBootstrapService.GetBootstrap` |
| `GET /v1/machines/{machineId}/sale-catalog` | `MachineCatalogService.GetCatalogSnapshot` / `GetCatalogDelta` |
| `POST /v1/commerce/orders` | `MachineCommerceService.CreateOrder` |
| `POST /v1/commerce/cash-checkout` | `MachineCommerceService.ConfirmCashPayment` / `CreateCashCheckout` |
| `POST /v1/commerce/orders/{orderId}/payment-session` | `MachineCommerceService.CreatePaymentSession` (**wallet/QR path**) |
| `GET /v1/commerce/orders/{orderId}` | `MachineCommerceService.GetOrder` / `GetOrderStatus` |
| `POST /v1/commerce/orders/{orderId}/vend/start` | `MachineCommerceService.StartVend` |
| `POST /v1/commerce/orders/{orderId}/vend/success` | `MachineCommerceService.ConfirmVendSuccess` |
| `POST /v1/commerce/orders/{orderId}/vend/failure` | `MachineCommerceService.ReportVendFailure` |
| `POST /v1/device/machines/{machineId}/vend-results` | `ConfirmVendSuccess` / `ReportVendFailure` |
| `POST /v1/device/machines/{machineId}/commands/poll` | MQTT command subscribe |
| `POST /v1/device/machines/{machineId}/events/reconcile` | `MachineTelemetryService.ReconcileEvents` or `PushOfflineEvents` |
| `POST /v1/machines/{machineId}/check-ins` | `MachineBootstrapService.CheckIn` or `MachineTelemetryService.CheckIn` |
| `POST /v1/machines/{machineId}/config-applies` | `MachineBootstrapService.AckConfigVersion` |
| `GET /v1/machines/{machineId}/shadow` | Shadow via bootstrap + MQTT `shadow/reported` |
| `GET /v1/machines/{machineId}/telemetry/*` | gRPC telemetry + MQTT ingest |
| `/v1/machines/{machineId}/operator-sessions/*` | HTTP operator PIN (until gRPC session exists) — **operator UI only** |

Source files: `commerce_http.go`, `sale_catalog_http.go`, `device_http.go`, `machine_runtime_http.go`, `operator_http.go`, `telemetry_reconcile_http.go`.

---

## Payment provider status

Registry: `internal/platform/payments/registry.go`, `runtime.go`, `placeholder_provider.go`.

| Key | Implementation | `CreatePaymentSession` | Production card/QR | Android signal |
|-----|----------------|------------------------|--------------------|----------------|
| **`cash`** | `cashPaymentProvider` | N/A — use `ConfirmCashPayment` | **Cash checkout only** | `payment_methods.cash_enabled=true` |
| **`mock`, `sandbox`, `test`, `psp_fixture`, `dev`, `psp_grpc_int`** | `SandboxProvider` | Fake QR URL | **Blocked** in `APP_ENV=production` | Staging/dev only |
| **`stripe`** | `PlaceholderLiveProvider` | **`ErrLiveProviderNotWired`** | **Placeholder** — webhook parse only | `qr_card_enabled=false`, `provider_unavailable` |
| **`momo`** | Placeholder | Same | **Placeholder** | Same |
| **`zalopay`** | Placeholder | Same | **Placeholder** | Same |
| **`vnpay`** | Placeholder | Same | **Placeholder** | Same |
| Custom `WiredLiveProvider` | Not registered | Would work when wired | **Future Path A** | `qr_card_enabled=true` |

### Config rules (`internal/config/deployment_env.go`)

| `PAYMENT_ENV` | Production behavior |
|---------------|---------------------|
| **`cash_only`** | QR/card hidden; `COMMERCE_PAYMENT_PROVIDER` must be **unset** |
| **`live`** | Requires wired PSP key; **placeholder keys fail config load** |
| **`sandbox`** | Staging only |

### Live PSP fully implemented?

**No.** All four named live keys (`stripe`, `momo`, `zalopay`, `vnpay`) are **adapter shells** — inbound webhook HMAC verification works; **outbound session creation, cancel, refund, and query are not implemented**. Production E2E `GRPC-COMM-QR-001` validates **AVF webhook + state machine** with a signed fixture, not a real PSP API.

---

## MQTT topic contract (Android)

**Source:** `internal/platform/mqtt/topics.go`, `router.go`, [`mqtt-contract.md`](../api/mqtt-contract.md).

### Configuration (from bootstrap / activation)

| Field | Source | Android rule |
|-------|--------|--------------|
| `mqtt.broker_url` | `GetBootstrap`, `ClaimActivation` | TLS connect target |
| `mqtt.topic_prefix` | Same | Never hardcode — trim, no trailing slash |
| `mqtt.topic_layout` | Same | **`enterprise`** in production; build topics with `…/machines/{machineId}/…` |

Production config requires explicit `MQTT_TOPIC_LAYOUT=enterprise` (`internal/config/config.go`).

### Enterprise layout (production)

| Direction | Topic pattern |
|-----------|---------------|
| **Subscribe (commands)** | `{prefix}/machines/{machineId}/commands` |
| **Publish ACK** | `{prefix}/machines/{machineId}/commands/ack` |
| **Publish telemetry** | `…/state/heartbeat`, `…/presence`, `…/telemetry`, `…/telemetry/snapshot`, `…/telemetry/incident` |
| **Publish events** | `…/events`, `…/events/vend`, `…/events/cash`, `…/events/inventory` |
| **Publish shadow** | `…/shadow/reported` |

Build helpers (backend reference): `DevicePublishTopicStrict(layout, prefix, machineId, rel)`.

### Command ACK payload (required fields)

`command_id`, `machine_id`, `status`, `occurred_at`, `sequence`, `dedupe_key`; **`error_reason` required** when status is `failed` / `nacked` / `timeout`.

### Legacy layout (dev/staging only)

`{prefix}/{machineId}/…` with outbound `…/commands/dispatch` — **not production default**.

---

## Idempotency rules

| Layer | Rule | Evidence |
|-------|------|----------|
| **gRPC mutations** | `IdempotencyContext.idempotency_key` required when `GRPC_REQUIRE_IDEMPOTENCY=true` (production) | `internal/grpcserver/interceptors.go`, `machine_replay_ledger.go` |
| **Replay** | Same key + **identical** protobuf body → `replay=true` / `ACCEPTED` | `TestMachineGRPC_Commerce_CreateOrder_IdempotentReplay` |
| **Conflict** | Same key + different body → `idempotency_payload_mismatch` | `machine_replay_ledger_integration_test.go` |
| **Commerce** | One stable key per customer intent; **new keys** for distinct vend steps (`CreateOrder` vs `StartVend` vs `ConfirmVendSuccess`) | [`kiosk-app-implementation-checklist.md`](../api/kiosk-app-implementation-checklist.md) §3 |
| **MQTT ACK** | `dedupe_key` unique per ACK; duplicates idempotent in OLTP | `ApplyCommandReceiptTransition`, `mqtt_command_integration_test.go` |
| **MQTT telemetry** | Prefer `dedupe_key`, else `boot_id`+`seq_no`, else `event_id` | `TelemetryIdempotencyKey()` |
| **Offline sync** | Each `OfflineEvent` carries `meta.idempotency_key` + monotonic `offline_sequence` | `PushOfflineEvents` ledger in `machine_contract_grpc.go` |
| **Legacy REST** | `Idempotency-Key` header on POST commerce/device routes | Only when legacy HTTP enabled |

**Android rule:** Persist idempotency keys in Room **before** first network call; reuse on retry.

---

## Machine JWT rules

| Rule | Detail |
|------|--------|
| **Header** | gRPC metadata: `authorization: Bearer <machine_access_jwt>` |
| **Pre-auth RPCs** | Only: `ClaimActivation`, `RefreshMachineToken` (+ auth service aliases) — 5 methods total |
| **Audience** | Production requires `MACHINE_AUTH_REQUIRE_AUDIENCE=true` |
| **Scope** | JWT `machine_id` must match `MachineRequestMeta.machine_id` when set |
| **Rejected** | Admin/user JWT on machine RPCs → `Unauthenticated` |
| **Lifecycle** | `suspended`, `retired`, `maintenance`, `compromised` blocked by credential gate |
| **Commerce gate** | Sales/inventory mutations require machine status **`online` or `offline`** |
| **Storage** | Access token in memory/encrypted storage; refresh token in Keystore |

Tests: `machine_grpc_auth_test.go`, `machine_grpc_production_contract_test.go`.

---

## Vend success / failure rules

| Step | Rule |
|------|------|
| Order | `CreateOrder` with idempotency; machine must pass commerce gate |
| Payment (cash) | `ConfirmCashPayment` after order; blocked if `cash_enabled=false` |
| Payment (QR) | `CreatePaymentSession` only if `qr_card_enabled=true`; poll `GetOrderStatus` until paid |
| Vend start | `StartVend` **after** payment settled; idempotency required |
| **Success** | `ConfirmVendSuccess` / `ReportVendSuccess` **only after** `StartVend` accepted — calling success before start is **rejected** |
| **Failure** | `ReportVendFailure` / `FailVend` with idempotency; inventory not decremented on failure path |
| **Cancel** | `CancelOrder` where order state allows |
| **MQTT mirror** | Publish `events/vend`, `events/cash` with dedupe for ops telemetry (parallel to gRPC truth) |

Tests: `TestMachineProductionContract_VendSuccessRejectedBeforeStartVend`, commerce integration tests.

---

## Offline sync rules

**Service:** `MachineOfflineSyncService.PushOfflineEvents`

### Supported `event_type` values (replay dispatches to same handlers as live gRPC)

| event_type | Maps to |
|------------|---------|
| `commerce.create_order`, `sale.create_order` | `CreateOrder` |
| `commerce.create_payment_session`, `sale.create_payment_session`, `commerce.attach_payment_result` | `CreatePaymentSession` |
| `commerce.confirm_cash_payment`, `sale.report_cash_payment`, `commerce.create_cash_checkout` | `ConfirmCashPayment` |
| `commerce.start_vend`, `sale.start_vend` | `StartVend` |
| `commerce.confirm_vend_success`, `sale.confirm_vend_success` | `ConfirmVendSuccess` |
| `commerce.confirm_vend_failure`, `sale.confirm_vend_failure` | `ReportVendFailure` |
| `commerce.cancel_order`, `sale.cancel_sale` | `CancelOrder` |
| `inventory.*`, `telemetry.batch`, `telemetry.critical` | Inventory / telemetry RPCs |
| Unknown types | `Unimplemented` per event |

**Rules:** Monotonic `offline_sequence`; stable idempotency per event; batch size capped (`PushOfflineEvents` guardrails test); durable local queue before network; after reconnect prefer `PushOfflineEvents` then drain MQTT outbox.

---

## Production E2E current status

| Harness | Status | Notes |
|---------|--------|-------|
| `scripts/e2e/production-readonly-smoke.sh` | **PARTIAL** (2026-05-29) | HTTP live/ready/version PASS on `api.ldtv.dev`; gRPC/admin skipped without creds; deployed `/version` lacks `payment_runtime` field |
| `scripts/e2e/production-canary-live-sale.sh` | **NOT RUN** | Gated: `PRODUCTION_LIVE_TEST_CONFIRMATION`, canary machine, price cap, admin auth |
| `tests/e2e/production/e2e-manifest*.yaml` | **Documented** | Cash gRPC `GRPC-COMM-CASH-001`; QR fixture `GRPC-COMM-QR-001`; MQTT partial |
| `go test ./...` | **PASS** | Full unit/integration corpus |

Runbook: [`docs/testing/PRODUCTION_E2E_CANARY_RUNBOOK.md`](../testing/PRODUCTION_E2E_CANARY_RUNBOOK.md).

---

## Backend blockers before paid market launch

| # | Blocker | Blocks |
|---|---------|--------|
| B1 | No `WiredLiveProvider` for any PSP | Live QR/card/wallet |
| B2 | `PAYMENT_ENV=live` + placeholder key fails config | Misconfigured prod |
| B3 | Android on legacy REST commerce/catalog | Production 404 when legacy off |
| B4 | No production canary sale evidence | Fleet rollout sign-off |
| B5 | Deployed API may lag repo (e.g. `payment_runtime`, `mqtt.topic_layout`) | Contract drift |
| B6 | MQTT `events/vend` / `events/cash` not in prod E2E manifest | Ops visibility gap |
| B7 | gRPC operator session RPCs unimplemented | Operator PIN stays on HTTP (acceptable for pilot) |

**Not blockers for cash-only canary:** B1–B2 if scope is cash-only and QR UI hidden.

---

## Exact Android app changes required

### P0 — Stop using legacy REST in production builds

| Remove / replace | Replace with |
|------------------|--------------|
| HTTP `GET …/setup/…/bootstrap` | gRPC `GetBootstrap` |
| HTTP `GET …/sale-catalog` | gRPC `GetCatalogSnapshot` + `GetCatalogDelta` + `MachineMediaService` |
| HTTP `POST /v1/commerce/orders` | gRPC `CreateOrder` |
| HTTP `POST /v1/commerce/cash-checkout` | gRPC `ConfirmCashPayment` |
| HTTP `POST …/payment-session` (wallet/QR) | gRPC `CreatePaymentSession` — **hide UI until bootstrap says enabled** |
| HTTP vend start/success/failure | gRPC `StartVend`, `ConfirmVendSuccess`, `ReportVendFailure` |
| HTTP `POST …/device/…/vend-results` | gRPC vend RPCs |
| HTTP `POST …/commands/poll` | MQTT subscribe enterprise command topic |
| HTTP `POST …/events/reconcile` | gRPC `ReconcileEvents` or `PushOfflineEvents` |

### P0 — Wire production transport

| Item | Action |
|------|--------|
| gRPC channel | `grpcs://` from `GRPC_PUBLIC_BASE_URL` / deployment config |
| Metadata | Machine JWT on every runtime RPC |
| Proto sync | Copy `proto/avf/machine/v1/*.proto`; track [`android-proto-sync.md`](../api/android-proto-sync.md) |
| MQTT | Read `broker_url`, `topic_prefix`, `topic_layout` from bootstrap; use **enterprise** topic builder |
| Payment UI | Read `GetBootstrap.payment_methods`; hide QR/card unless `qr_card_enabled==true` |

### P1 — Idempotency and offline

| Item | Action |
|------|--------|
| Room | Store idempotency keys per sale step before RPC |
| Offline queue | `PushOfflineEvents` with supported `event_type` strings |
| MQTT outbox | Durable queue for telemetry/command ACK with `dedupe_key` |
| Crash recovery | On cold start: `GetOrderStatus` before resuming vend |

### P2 — Operator / diagnostics

| Item | Action |
|------|--------|
| Operator PIN | Keep HTTP operator session routes **only on operator screens** (or admin-started session) |
| OTA | `MachineCommandService.GetAssignedUpdate` + MQTT `OTA_APPLY` handling |
| Command ACK | MQTT `commands/ack` with full ACK contract (not gRPC `AckCommand`) |

---

## Canonical contract statement (locked)

> **Production Android vending app contract:**  
> **`avf.machine.v1` gRPC over TLS with Machine JWT** for bootstrap, catalog, media, inventory, commerce, telemetry, and offline sync;  
> **MQTT TLS enterprise layout** (`{prefix}/machines/{machineId}/…`) for command delivery and device-originated telemetry/events;  
> **REST** limited to **activation claim fallback**, **operator session HTTP** (PIN), and **PSP webhooks** (cloud-side);  
> **Legacy machine REST and `/v1/commerce/*` checkout paths are deprecated and disabled in production by default.**  
> **Payments:** cash-only pilot via `ConfirmCashPayment`; live QR/card requires a future wired PSP and `PAYMENT_ENV=live`.

---

## Appendix: implementation map

| Concern | Primary files |
|---------|---------------|
| gRPC registration | `internal/grpcserver/machine_grpc_services.go` |
| JWT interceptors | `internal/grpcserver/interceptors.go` |
| Commerce | `internal/grpcserver/machine_commerce_grpc.go` |
| Catalog/media | `internal/grpcserver/machine_catalog_grpc.go` |
| Offline sync | `internal/grpcserver/machine_contract_grpc.go` |
| REST legacy gate | `internal/httpserver/server.go`, `transport_legacy_guard.go` |
| REST commerce (legacy) | `internal/httpserver/commerce_http.go` |
| Payments | `internal/platform/payments/registry.go`, `runtime.go` |
| MQTT | `internal/platform/mqtt/` |
| Production config | `internal/config/deployment_env.go` |
| Contract tests | `internal/grpcserver/machine_grpc_production_contract_test.go` |
