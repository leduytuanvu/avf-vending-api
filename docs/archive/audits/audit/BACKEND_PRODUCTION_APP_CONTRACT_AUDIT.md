# Backend production readiness audit — Android vending app contract

**Date:** 2026-05-28  
**Repository:** `avf-vending-api`  
**Scope:** Machine-facing contract (gRPC, MQTT, REST fallback, payments, production E2E) for the Android kiosk app. **No runtime code changed** in this audit pass.  
**Normative references:** [`docs/architecture/production-final-contract.md`](../architecture/production-final-contract.md), [`docs/api/machine-grpc.md`](../api/machine-grpc.md), [`docs/api/mqtt-contract.md`](../api/mqtt-contract.md), [`docs/api/kiosk-app-implementation-checklist.md`](../api/kiosk-app-implementation-checklist.md)

---

## Executive verdict

| Question | Answer |
|----------|--------|
| **Is the backend ready for real paid card/QR sales in production?** | **No.** Live PSP adapters (`stripe`, `momo`, `zalopay`, `vnpay`) are **placeholder shells** — `CreatePaymentSession` returns `ErrLiveProviderNotWired`. Production config requires `PAYMENT_ENV=live` and forbids sandbox-family providers. |
| **Is the backend ready for cash sales on Android (gRPC primary)?** | **Yes, with operational gates.** Cash checkout via `MachineCommerceService` (`ConfirmCashPayment` / `CreateCashCheckout` → vend lifecycle) is implemented, idempotent, and covered by integration + production E2E (`GRPC-COMM-CASH-001`). Machine must be **`online` or `offline`** lifecycle for commerce/inventory gates. |
| **Is the machine runtime contract (catalog, bootstrap, offline, telemetry) production-primary on gRPC?** | **Yes.** All ten listed services are registered when `MACHINE_GRPC_ENABLED=true` (required in `APP_ENV=production`). Partial **by design:** command poll/ack RPCs and operator session open/close on gRPC are **Unimplemented**; Android must use **MQTT** and **HTTP operator routes** respectively. |
| **Can Android rely on legacy REST machine HTTP in production?** | **No (default).** `ENABLE_LEGACY_MACHINE_HTTP` defaults **off** in production; enabling requires `MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION=true`. Primary integration is **gRPC + MQTT**. |

**Bottom line:** Ship **cash + catalog + commands + telemetry** on gRPC/MQTT today. **Do not launch live card/QR/money movement** until at least one real PSP adapter is wired and validated end-to-end in `PAYMENT_ENV=live`.

---

## Validation run (this audit)

| Command | Environment | Result |
|---------|-------------|--------|
| `go test ./...` | Windows, branch `chore/clean-project-nonessential-files` | **PASS** |
| `go test -race ./...` | Windows | **SKIP** — `go: -race requires cgo` (`CGO_ENABLED=1` not available) |
| `make test` / `make test-short` | Windows | **SKIP** — `make` not on PATH |
| OpenAPI / Postman validators | Not re-run for this doc-only audit | See [`docs/audits/market-readiness-test-results.md`](../audits/market-readiness-test-results.md) on `develop` |

Race and Linux CI gates should be treated as authoritative for merge; unit/integration corpus passed locally.

---

## 1. Machine gRPC services

**Registration:** `internal/bootstrap/api.go` → `internal/grpcserver/machine_grpc_services.go`  
**Auth:** Machine JWT via `authorization: Bearer` (`internal/grpcserver/interceptors.go`). Pre-auth: activation + token refresh only.  
**Production gate:** `MACHINE_GRPC_ENABLED=true` mandatory when `APP_ENV=production` (`internal/config/deployment_env.go`).

### Summary table

| Service | Implemented | Placeholder / stub | Fallback only | Android primary? | Production-ready |
|---------|-------------|-------------------|-----------------|-------------------|------------------|
| `MachineAuthService` | `ClaimActivation`, `RefreshMachineToken`, `ActivateMachine` alias | — | Legacy naming; prefer `MachineActivationService` / `MachineTokenService` | **Pre-auth only** | **Yes** |
| `MachineBootstrapService` | `GetBootstrap`, `CheckForUpdates`, `CheckIn`, `AckConfigVersion` | — | — | **Yes** | **Yes** |
| `MachineCatalogService` | Full snapshot/delta/bundle/ack + `GetSaleCatalog` aliases | — | HTTP sale-catalog if legacy enabled | **Yes** | **Yes** |
| `MachineMediaService` | `GetMediaManifest`, `GetMediaDelta`, `AckMediaVersion` | — | Catalog manifest overlap | **Yes** (offline cache) | **Yes** |
| `MachineInventoryService` | Snapshots, planogram, adjustments, fill/restock, acks | — | — | **Yes** (runtime gate: online/offline) | **Yes** |
| `MachineCommerceService` | Full order/payment/vend/cancel + reads | Live PSP session **delegates to registry** → fails for placeholder providers | REST commerce if legacy enabled | **Yes** | **Yes (cash)** / **No (live card/QR)** |
| `MachineTelemetryService` | Batch, critical, check-in, reconcile, event status | — | MQTT ingest parallel path | **Yes** (batch/check-in); MQTT for firehose | **Yes** |
| `MachineOfflineSyncService` | `PushOfflineEvents`, `GetSyncCursor` | Unsupported event types → per-event rejected | — | **Yes** (bounded event types) | **Yes** |
| `MachineCommandService` | OTA + diagnostics RPCs | **`GetPendingCommands`, `AckCommand`, `RejectCommand` → Unimplemented** | MQTT command plane | **MQTT primary**; gRPC for OTA only | **Partial (by design)** |
| `MachineOperatorService` | Fill/adjust + heartbeat | **`OpenOperatorSession`, `CloseOperatorSession`, `LoginOperator`, `LogoutOperator` → Unimplemented** | HTTP operator sessions | **gRPC for fill/adjust**; **HTTP for session PIN** | **Partial** |

### RPC / file map (implementation)

| Service | Proto | Primary implementation files |
|---------|-------|------------------------------|
| Auth / activation / token | `proto/avf/machine/v1/auth.proto`, `machine_activation.proto`, `machine_token.proto` | `machine_grpc_services.go` |
| Bootstrap | `bootstrap.proto` | `machine_grpc_services.go`, `machine_contract_grpc.go` |
| Catalog / media | `catalog.proto`, `media.proto` | `machine_catalog_grpc.go` |
| Inventory | `inventory.proto` | `machine_inventory_grpc.go`, `machine_contract_grpc.go` |
| Commerce | `commerce.proto` | `machine_commerce_grpc.go` |
| Telemetry | `telemetry.proto` | `machine_telemetry_grpc.go`, `machine_contract_grpc.go` |
| Offline sync | `offline_sync.proto` | `machine_contract_grpc.go` |
| Commands | `command.proto` | `machine_contract_grpc.go` (`machineUnimplemented` for poll/ack) |
| Operator | `operator_grpc.proto` | `machine_operator_grpc.go` |

### Android must call (gRPC)

1. **Provisioning:** `MachineActivationService.ClaimActivation` (or auth alias) → `MachineTokenService.RefreshMachineToken`.
2. **Runtime:** `MachineBootstrapService.GetBootstrap` / `CheckForUpdates` / `CheckIn` / `AckConfigVersion`.
3. **Catalog + media:** `SyncSaleCatalog` or `GetCatalogSnapshot`, `SyncCatalogBundle` / `GetCatalogDelta`, `MachineMediaService.GetMediaManifest` / `GetMediaDelta`.
4. **Sales:** `MachineCommerceService` — cash: `CreateOrder` → `ConfirmCashPayment` → `StartVend` → `ConfirmVendSuccess`; QR/card: `CreatePaymentSession` then poll `GetOrderStatus` (requires **live** PSP).
5. **Offline:** `MachineOfflineSyncService.PushOfflineEvents` with monotonic sequence.
6. **Inventory (operator/restock):** `MachineInventoryService` mutations when machine is online/offline.
7. **Do not use:** gRPC command polling/ack; gRPC operator session open/close.

Companion (optional naming): `MachineSaleService` aliases in `machine_contract_grpc.go` — same orchestrator as commerce.

---

## 2. REST fallback routes

**Router:** `internal/httpserver/server.go` (`mountV1`), documented in `internal/httpserver/router.go`.  
**Legacy gate:** `ENABLE_LEGACY_MACHINE_HTTP` / `MACHINE_REST_LEGACY_ENABLED` + `internal/httpserver/transport_legacy_guard.go`.  
**Deprecation headers:** `internal/httpserver/api_surface_deprecation.go` (admin alias routes only; not all legacy machine paths).

### Setup / activation

| Route | Status | Android use |
|-------|--------|-------------|
| `POST /v1/setup/activation-codes/claim` | **Primary** (always mounted, public) | **Yes** — first boot claim (or gRPC `ClaimActivation` equivalent) |
| `GET /v1/admin/machines/{id}/activation-codes` etc. | **Primary** (admin) | No — backoffice |
| `GET /v1/setup/machines/{machineId}/bootstrap` | **Legacy** (gated) | **No** — use gRPC `GetBootstrap` |

### Commerce

| Route | Status | Android use |
|-------|--------|-------------|
| `POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks` | **Primary** (PSP → AVF HMAC) | Indirect — PSP callback, not app |
| `/v1/commerce/*` (orders, payment-session, vend, cash-checkout) | **Legacy** (gated off in prod default) | **Fallback only** if legacy explicitly enabled |

**Gap:** Legacy HTTP `payment-session` still accepts client `outbox_payload_json`; gRPC path is server-owned but useless until live PSP wired (`internal/httpserver/commerce_http.go` vs `internal/app/commerce/machine_payment_session.go`).

### Device / commands

| Route | Status | Android use |
|-------|--------|-------------|
| `POST /v1/admin/machines/{id}/commands` | **Primary** (admin dispatch) | No — cloud/admin |
| `GET /v1/machines/{id}/commands/{seq}/status` | **Primary** | Optional readback |
| `POST /v1/device/machines/{id}/commands/poll` | **Legacy** + OpenAPI `deprecated` | **No** — use MQTT |
| `POST /v1/device/machines/{id}/vend-results` | **Legacy** + deprecated | **No** — use gRPC vend RPCs |
| `POST /v1/device/machines/{id}/events/reconcile` | **Legacy** + deprecated | **No** — use gRPC `ReconcileEvents` / offline sync |

### Legacy machine runtime (all gated)

Examples: `/v1/machines/{id}/sale-catalog`, `/check-ins`, `/config-applies`, `/shadow`, `/telemetry/*`, `/operator-sessions/*` — **deprecated / legacy HTTP**. Successor: corresponding gRPC services.

### Admin

`/v1/admin/*` — **Primary** for backoffice only. Dual-mount aliases (`/v1/admin/users` → `/v1/admin/auth/users`, media/product image paths) emit `Deprecation: true` + `Link` successor headers.

---

## 3. Payment providers

**Registry:** `internal/platform/payments/registry.go`  
**Session orchestration:** `internal/app/commerce/machine_payment_session.go`

| Provider key | Implementation | CreatePaymentSession | Webhook HMAC | Query/Cancel/Refund | Live-ready |
|--------------|----------------|----------------------|--------------|---------------------|------------|
| `mock`, `sandbox`, `test`, `psp_fixture`, `dev`, `psp_grpc_int` | `SandboxProvider` | Fake URL/QR | Yes | Not implemented | **Staging only** — blocked in production |
| `cash` | `cashPaymentProvider` | N/A (use cash RPC) | Yes | Not implemented | **Yes** |
| `stripe`, `momo`, `zalopay`, `vnpay` | `PlaceholderLiveProvider` | **`ErrLiveProviderNotWired`** | Parse/verify only | Not implemented | **No** |

**Config rules** (`internal/config/deployment_env.go`, `internal/config/config.go`):

- Staging: `PAYMENT_ENV=sandbox` required.
- Production: `PAYMENT_ENV=live` + `COMMERCE_PAYMENT_PROVIDER` must not be sandbox-family.
- Example prod env template points at `vnpay` — **config alone does not enable live sessions**.

**Production E2E note:** `GRPC-COMM-QR-001` uses provider `"stripe"` in request + signed AVF webhook fixture — validates **webhook + state machine**, not a real Stripe API call (`tests/e2e/production/lib/grpc_handlers.sh`). Excluded from default “no-online-payment” suite profiles.

---

## 4. MQTT contract

**Source of truth:** `internal/platform/mqtt/topics.go`, `internal/platform/mqtt/router.go`, [`docs/api/mqtt-contract.md`](../api/mqtt-contract.md).

### Layouts

| Layout | Env | Machine inbound example | Outbound command |
|--------|-----|-------------------------|------------------|
| **Legacy** (default) | `MQTT_TOPIC_LAYOUT=legacy` | `{prefix}/{machineId}/state/heartbeat` | `{prefix}/{machineId}/commands/dispatch` |
| **Enterprise** | `MQTT_TOPIC_LAYOUT=enterprise` | `{prefix}/machines/{machineId}/state/heartbeat` | `{prefix}/machines/{machineId}/commands` |

### Topic inventory

| Relative tail | Direction | Handler | Android primary? | Prod E2E |
|---------------|-----------|---------|-------------------|----------|
| `commands/dispatch` (legacy) / `commands` (enterprise) | Cloud → device | Subscribe | **Yes** | Partial |
| `commands/ack` | Device → cloud | Ingest + ledger | **Yes** | **Yes** (`MQTT-CMD-001`) |
| `commands/receipt` | Device → cloud | Alias of ack path | Optional alias | **No** (local scenario only) |
| `commands/down` | Cloud → device | Alias | Rare | **No** |
| `state/heartbeat` | Device → cloud | Ingest | **Yes** | **Yes** |
| `presence` | Device → cloud | Ingest | **Yes** | **Yes** |
| `telemetry/snapshot` | Device → cloud | Ingest | **Yes** | **Yes** |
| `telemetry/incident` | Device → cloud | Ingest | **Yes** | **No** |
| `events/vend` | Device → cloud | Critical commerce | **Yes** | **No** |
| `events/cash` | Device → cloud | Cash telemetry | **Yes** | **No** |
| `events/inventory` | Device → cloud | Ingest | **Yes** | **Yes** |
| `shadow/reported`, `shadow/desired` | Device → cloud | Shadow | Optional | **No** |
| `telemetry` (legacy envelope) | Device → cloud | Ingest | Fallback | **No** (prod E2E) |
| `{prefix}/machines/+/events` | Device → cloud | Enterprise umbrella | If enterprise layout | **No** |

**Command ACK contract:** `dedupe_key` / command sequence alignment with REST command status — see mqtt-contract.md. gRPC `AckCommand` is **not** a substitute.

---

## 5. Production E2E coverage

**Harness:** `tests/e2e/production/` — manifests: `e2e-manifest.yaml`, `e2e-manifest-grpc.yaml`, `e2e-manifest-mqtt.yaml`, `e2e-manifest-rest-coverage.yaml`.

| Scenario | Covered? | Flow IDs / notes |
|----------|----------|------------------|
| Health / readiness | **Yes** | `REST-PREFLIGHT-001/002/003` |
| Activation | **Yes** | REST machine create + claim; `GRPC-BOOT-*` after token |
| gRPC catalog/media/inventory/offline | **Yes** | `GRPC-CAT-*`, `GRPC-MEDIA-*`, `GRPC-INV-*`, `GRPC-OFFLINE-001` |
| gRPC token refresh | **Yes** | `GRPC-TOKEN-001` |
| MQTT connect | **Yes** | `MQTT-CONN-001/002` |
| MQTT command ACK | **Yes** | `MQTT-CMD-001` (admin dispatch → device ack) |
| MQTT telemetry | **Partial** | heartbeat, presence, snapshot, inventory — **missing** vend/cash/incident/shadow |
| Cash sale | **Yes (gRPC)** | `GRPC-COMM-CASH-001` |
| QR / online sale | **Conditional** | `GRPC-COMM-QR-001` + webhook; skipped in many suite profiles; **sandbox/Stripe label only** |
| Card terminal | **No** | No physical card flow |
| Vend success | **Yes** | REST commerce flows + gRPC cash/QR paths |
| Vend failure | **Partial** | `GRPC-COMM-FAIL-001`; REST vend-failure/refund not in prod manifest |
| Reconciliation | **Partial** | Optional `REST-REPORT-004` read queue; **no** `POST .../events/reconcile` E2E |
| Cleanup | **Attestation only** | `lib/cleanup.sh` — no automated DELETE |
| Postman parity | **CI expected** | `scripts/ci/verify_production_postman_parity.sh` + `postman/production/*` |

**Local-only scenarios** (not prod manifest): `tests/e2e/scenarios/23_grpc_inventory_telemetry_offline.sh` (`ReconcileEvents`), `45_e2e_remote_command_ack.sh` (`commands/receipt`), REST vend failure scripts.

---

## 6. Exact missing tests

| ID | Gap | Suggested location |
|----|-----|-------------------|
| T1 | Live PSP `CreatePaymentSession` against real Stripe/MoMo/ZaloPay/VNPay sandbox APIs | New integration tests in `internal/platform/payments/` + prod E2E profile |
| T2 | Production E2E with `PAYMENT_ENV=live` + wired provider (not placeholder) | `tests/e2e/production/e2e-manifest-grpc.yaml` |
| T3 | MQTT `events/vend` after gRPC vend success | `e2e-manifest-mqtt.yaml`, `lib/mqtt_handlers.sh` |
| T4 | MQTT `events/cash` during cash sale | Same |
| T5 | MQTT `telemetry/incident` critical path | Same + `testdata/telemetry/` |
| T6 | MQTT `shadow/reported` + desired sync | Same |
| T7 | MQTT `commands/receipt` alias parity | Port from `tests/e2e/scenarios/45_e2e_remote_command_ack.sh` |
| T8 | Enterprise topic layout end-to-end (no legacy prefix override) | `lib/mqtt_common.sh` profile |
| T9 | gRPC `MachineTelemetryService.ReconcileEvents` in prod manifest | Extend `e2e-manifest-grpc.yaml` |
| T10 | Device HTTP `POST .../events/reconcile` + status GET | Remove `documented_skip` in `rest-route-overrides.yaml`; add REST flows |
| T11 | REST vend failure + refund path in prod | `e2e-manifest.yaml` or extend gRPC fail handler |
| T12 | Operator session via HTTP + gRPC fill report combined | New flow |
| T13 | Card-present / terminal payment (if product requires) | **Undefined** — no backend fixture today |
| T14 | Automated post-deploy cleanup | `lib/cleanup.sh`, `run-cleanup-production-e2e.sh` |
| T15 | `go test -race` on Linux CI | Already in `.github/workflows/production-proof.yml` — not run on Windows agent |

---

## 7. Production launch blockers

| # | Blocker | Severity | Evidence |
|---|---------|----------|----------|
| **B1** | **No live PSP adapter** — production cannot create real payment sessions | **P0 — blocks paid QR/card** | `internal/platform/payments/placeholder_provider.go` |
| **B2** | **`PAYMENT_ENV=live` + placeholder provider key** → runtime payment session failure | **P0** | `registry.go`, prod env examples |
| **B3** | **No outbound PSP status query / cancel / refund** — reconciliation relies on webhooks + optional HTTP probe | **P1** | `PlaceholderLiveProvider`, `SandboxProvider` |
| **B4** | **Production E2E does not prove live money path** | **P1** | `GRPC-COMM-QR-001` uses webhook fixture only |
| **B5** | **MQTT critical commerce telemetry not in prod E2E** (`events/vend`, `events/cash`) | **P1** — ops visibility | `e2e-manifest-mqtt.yaml` |
| **B6** | **Device event reconcile HTTP not E2E-tested** | **P2** | `rest-route-matrix.json` documented_skip |
| **B7** | **Android must not depend on legacy REST machine HTTP** (default off) | **P1 — integration** | `deployment_env.go`, `transport_legacy_guard.go` |
| **B8** | **`MACHINE_GRPC_ENABLED=true` required** — misconfig fails prod validation | **P0 — deployment** | `deployment_env.go` |

**Not blockers for cash-only pilot:** B1–B4 if scope is **cash-only** and `ConfirmCashPayment` path only; still require B5–B8 for operational confidence.

---

## 8. Files / modules to modify (later phases)

### Phase A — Live payments (P0)

| Area | Files |
|------|-------|
| Stripe adapter | `internal/platform/payments/stripe_provider.go` (new), `registry.go` |
| MoMo / ZaloPay / VNPay | `internal/platform/payments/momo_provider.go`, etc. (new) |
| Session + webhook mapping | `internal/app/commerce/machine_payment_session.go`, `internal/httpserver/commerce_webhook_public.go` |
| Config / secrets | `internal/config/config.go`, `deployments/prod/.env.production.example`, `docs/api/payment.md` |
| Tests | `internal/platform/payments/*_test.go`, `tests/e2e/production/lib/grpc_handlers.sh`, `e2e-manifest-grpc.yaml` |

### Phase B — MQTT / telemetry completeness (P1)

| Area | Files |
|------|-------|
| Router / ingest | `internal/platform/mqtt/router.go` |
| Android contract docs | `docs/api/mqtt-contract.md`, `docs/api/kiosk-app-implementation-checklist.md` |
| E2E | `tests/e2e/production/e2e-manifest-mqtt.yaml`, `lib/mqtt_handlers.sh`, `testdata/telemetry/*.json` |

### Phase C — REST legacy retirement (P2)

| Area | Files |
|------|-------|
| Legacy HTTP | `internal/httpserver/machine_runtime_http.go`, `commerce_http.go`, `device_http.go` |
| Deprecation surface | `internal/httpserver/api_surface_deprecation.go`, `docs/swagger/swagger.json` |
| Android migration | Confirm zero dependency on gated routes |

### Phase D — Operator flows (P2)

| Area | Files |
|------|-------|
| HTTP operator sessions | `internal/httpserver/operator_http.go` |
| gRPC partial | `internal/grpcserver/machine_operator_grpc.go` (only if product requires gRPC session) |

### Phase E — E2E / CI hardening (P1)

| Area | Files |
|------|-------|
| Prod manifests | `tests/e2e/production/e2e-manifest*.yaml`, `suite-profiles.yaml` |
| Postman parity | `postman/production/generate_postman_from_manifest.py`, `scripts/ci/verify_production_postman_parity.sh` |
| Cleanup | `tests/e2e/production/lib/cleanup.sh` |

---

## 9. Android integration quick reference

```
[First boot]
  REST POST /v1/setup/activation-codes/claim  OR  gRPC ClaimActivation
  → store machine refresh token
  → gRPC RefreshMachineToken

[Every session]
  gRPC GetBootstrap / CheckForUpdates
  gRPC SyncSaleCatalog + MachineMediaService manifests
  MQTT subscribe: commands/dispatch (or enterprise commands)
  MQTT publish: state/heartbeat, presence, events/* as needed

[Cash sale]
  gRPC CreateOrder → ConfirmCashPayment → StartVend → ConfirmVendSuccess
  (machine lifecycle: online|offline)

[QR/card sale — BLOCKED until Phase A]
  gRPC CreateOrder → CreatePaymentSession → poll GetOrderStatus
  PSP webhook → AVF (server-side)

[Commands]
  MQTT ACK on commands/ack — NOT gRPC AckCommand

[Operator refill]
  HTTP operator-sessions/start (PIN)
  gRPC SubmitFillReport / SubmitStockAdjustment
  HTTP operator-sessions/end
```

---

## 10. Related audits

- Inventory counts: [`docs/audits/api-grpc-mqtt-full-inventory.md`](../audits/api-grpc-mqtt-full-inventory.md)
- Market readiness: [`docs/audits/market-readiness-final-report.md`](../audits/market-readiness-final-report.md) — **READY WITH RISKS** (aligns with payment placeholder finding)
- Transport boundary: [`docs/architecture/transport-boundary.md`](../architecture/transport-boundary.md)

---

*Audit produced without runtime code changes. Re-run `go test ./...` and production E2E manifests after any contract or payment implementation work.*
