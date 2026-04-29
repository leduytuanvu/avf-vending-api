# Machine gRPC API (`avf.machine.v1`)

> **Release / TPM:** The **normative production integration contract** (Admin REST vs machine gRPC vs MQTT vs payments vs media) is **[`production-final-contract.md`](../architecture/production-final-contract.md)**. Field QA uses **[`field-test-cases.md`](../testing/field-test-cases.md)**. Android handoff: [`kiosk-app-implementation-checklist.md`](kiosk-app-implementation-checklist.md).

Native machine runtime contracts live under `proto/avf/machine/v1`. The **entry import path** for the full public surface is [`machine_runtime.proto`](../../proto/avf/machine/v1/machine_runtime.proto) (documentation aggregator). This package is **not** `avf.internal.v1` — internal query gRPC is service-to-service only and must not be used as the public vending machine app API.

## Production endpoint

Vending apps should connect to **`grpcs://machine-api.<your-domain>:443`** (TLS on port 443). Set **`GRPC_PUBLIC_BASE_URL`** to that URI on the API. **`APP_ENV=production`** requires TLS either on-process (**`GRPC_TLS_ENABLED`**) or at a reverse proxy (**`GRPC_BEHIND_TLS_PROXY`**) — plaintext **without** TLS termination is rejected at config validation. Operational detail: [`../runbooks/grpc-production.md`](../runbooks/grpc-production.md).

## RPC classification (summary)

| Kind | Examples |
| ---- | -------- |
| **Read-only** | Catalog/media/bootstrap reads, `GetOrder` / `GetOrderStatus`, inventory snapshots and planogram, `GetEventStatus`, `GetSyncCursor`, `GetPendingCommands` / `GetAssignedUpdate` (polling reads only). |
| **Retryable mutation (ledger + idempotency key)** | Commerce checkout and vend lifecycle; sale-service aliases; inventory deltas, snapshots, adjustments, fill/restock/ack; telemetry check-in / batch / critical / `ReconcileEvents`; offline `PushOfflineEvents`; bootstrap `CheckIn` + config ack; catalog/media/inventory sync acks; command `ReportUpdateStatus` / `ReportDiagnosticBundleResult`; operator `SubmitFillReport` / `SubmitStockAdjustment` and **`HeartbeatOperatorSession`**. |
| **Non-retryable / N/A on this surface** | Auth token exchange (`ClaimActivation`, `RefreshMachineToken`) — use new client requests, not the mutation ledger. |
| **Streaming / long-lived** | None in `avf.machine.v1` public surface today; telemetry uses unary batch RPCs. |
| **Unimplemented on gRPC (by design)** | `AckCommand` / `RejectCommand` (MQTT + ledger); `OpenOperatorSession` / `CloseOperatorSession` / operator `Login` / `Logout` (HTTP operator identity). **No idempotency ledger** — do not call for primary flows. |

When **`GRPC_REQUIRE_IDEMPOTENCY`** is true, the process must wire a non-nil ledger: idempotent mutations return **`Internal`** if the ledger is missing (fail closed). Stale **`in_progress`** rows may be reclaimed after idle time so a client can retry; if a handler committed domain state but crashed before `succeeded`, **domain-level** idempotency (orders, payments, inventory) remains the backstop against duplicate side effects.

## Services

| Service | Role |
| ------- | ---- |
| `MachineAuthService` | **`ClaimActivation`**, **`RefreshMachineToken`** (also **`ActivateMachine`** compatibility alias). |
| `MachineActivationService` | Legacy registration for **`ClaimActivation`** — same handler surface as auth service. |
| `MachineTokenService` | Legacy registration for **`RefreshMachineToken`**. |
| `MachineBootstrapService` | **`GetBootstrap`**, **`CheckForUpdates`**, **`CheckIn`** (delegates to the telemetry **`CheckIn`** path — persists **`machine_check_ins`** when configured), **`AckConfigVersion`** (persists ack fields on **`machine_current_snapshot`** when provided). |
| `MachineCatalogService` | **`GetSaleCatalog`** / **`SyncSaleCatalog`** (aliases of **`GetCatalogSnapshot`**), **`GetCatalogSnapshot`**, **`GetCatalogDelta`**, **`AckCatalogVersion`**, **`GetMediaManifest`**. |
| `MachineMediaService` | **`GetMediaManifest`**, **`GetMediaDelta`**, **`AckMediaVersion`**. |
| `MachineCommerceService` | Kiosk commerce: **`CreateOrder`**, **`CreatePaymentSession`** / **`AttachPaymentResult`**, **`ConfirmCashPayment`** / **`CreateCashCheckout`**, vend lifecycle (**`StartVend`**, **`ConfirmVendSuccess`**, **`ReportVendSuccess`**, **`ReportVendFailure`**), **`CancelOrder`**, reads (**`GetOrder`**, **`GetOrderStatus`**). |
| `MachineSaleService` | Native runtime names: **`CreateSale`**, **`AttachPayment`**, **`ConfirmCashReceived`**, **`StartVend`**, **`CompleteVend`**, **`FailVend`**, **`CancelSale`** — each delegates to the same commerce orchestrator as `MachineCommerceService` / REST. |
| `MachineInventoryService` | **`PushInventoryDelta`**, **`GetInventorySnapshot`**, **`AckInventorySync`**, **`SubmitFillResult`** / **`SubmitFillReport`**, **`SubmitInventoryAdjustment`** / **`SubmitStockAdjustment`**, **`SubmitRestock`**, **`SubmitStockSnapshot`**, etc. |
| `MachineTelemetryService` | **`PushTelemetryBatch`**, **`PushCriticalEvent`**, **`ReconcileEvents`**, **`GetEventStatus`** (read status; **`ReconcileEvents`** participates in mutation idempotency). |
| `MachineOfflineSyncService` | **`PushOfflineEvents`**, **`GetSyncCursor`** — persists offline replay rows and dispatches supported event types. |
| `MachineCommandService` | OTA/diagnostics (**`GetAssignedUpdate`**, **`ReportUpdateStatus`**, **`ReportDiagnosticBundleResult`**). **`GetPendingCommands`**, **`AckCommand`**, **`RejectCommand`** are proto-**deprecated** — they return structured **`Unimplemented`** stating MQTT TLS + ledger delivery (**do not use as primary polling**). |
| `MachineOperatorService` | **`SubmitFillReport`**, **`SubmitStockAdjustment`** delegate to inventory. **`OpenOperatorSession`**, **`CloseOperatorSession`** return **`Unimplemented`** (human identity uses HTTP operator routes). **`HeartbeatOperatorSession`** is implemented when operator service is configured. |

## Authentication

Use metadata: `authorization: Bearer <token>`.

**Without Machine JWT** (interceptor allows handler):

- `MachineAuthService.ActivateMachine`
- `MachineAuthService.ClaimActivation`
- `MachineAuthService.RefreshMachineToken` (opaque refresh in body — still **no** Machine JWT)
- **`MachineActivationService.ClaimActivation`** (legacy registration — same body as auth service)
- **`MachineTokenService.RefreshMachineToken`** (legacy registration — same body as auth service)

**Requires Machine JWT** — all other `avf.machine.v1` RPCs.

Machine JWT validation runs in the unary interceptor before handlers: signature, issuer, audience, `token_use=machine_access`, role, time bounds, optional Redis JTI revocation, optional client certificate match, and Postgres credential/version gates.

Postgres gates enforce **`machines.credential_version`** plus operational **`machines.status`**. Rows in **`suspended`**, **`compromised`**, **`retired`**, **`decommissioned`**, or **`maintenance`** (and related lifecycle states not in the allow-list) fail closed so kiosk authentication cannot succeed until admins resume/repair the machine through fleet APIs. Pre-service states such as **`draft`**, **`provisioned`**, **`provisioning`**, **`online`**, and **`offline`** are accepted by the credential gate when not combined with a blocking condition above.

Access tokens include **`credential_version`** (alias **`token_version`** for backward compatibility), **`session_id`** when issued after activation or refresh, **`jti`**, `iss`, `aud`, `organization_id`, `machine_id`, and role **`machine`**. When **`session_id`** is present, the credential checker validates the **`machine_sessions`** row (active, unexpired, matching credential and machine state). Tokens without **`session_id`** use the legacy gate until clients roll forward.

Admin JWTs must not be accepted as Machine JWTs (`token_use`, `typ`, audience, and role checks).

Request `machine_id` / `organization_id` fields (including nested `MachineRequestMeta`) must match the token or the call returns **`PermissionDenied`**.

## Backend-owned card / QR payment sessions (`CreatePaymentSession`)

`MachineCommerceService.CreatePaymentSession` creates the **`payments`** row and PSP session **only on the server**. The kiosk sends **`order_id`**, **`amount_minor`**, **`currency`**, optional **`provider`** (validated against `COMMERCE_PAYMENT_PROVIDER` when set), and **`IdempotencyContext`**. The server loads the order, checks totals, resolves the **payment provider registry** adapter, calls **`CreatePaymentSession`** on that adapter, persists **`payment_attempts`** with the adapter’s **`provider_reference`** / display JSON, and returns **`qr_payload_or_url`** for UI (HTTPS URL or opaque payload from the PSP — never from untrusted client fields). Fields such as **`outbox_payload_json`**, provider references, payment URLs, or QR material from the client are **ignored** for trust boundaries. Capture still happens via the **REST** webhook path (HMAC, idempotent); see [`payment.md`](payment.md) and [`payment-webhook-security.md`](payment-webhook-security.md).

## Common envelopes

- **`MachineRequestMeta`**: `organization_id`, `machine_id`, `request_id`, `idempotency_key`, `occurred_at`, `client_sequence`, `offline_sequence`, `app_version`, `config_version`, `catalog_version`, `client_event_id`.
- **`IdempotencyContext`** (mutations): `idempotency_key`, `client_event_id`, `client_created_at`, optional `operator_session_id`.
- **`MachineResponseMeta`**: `server_time`, `trace_id`, `request_id`, `status`, **`retryable`**, **`error_code`** (set where applicable).

## Idempotency ledger

Mutations listed in `internal/grpcserver/machine_replay_ledger.go` (`isMachineIdempotentMutation`) **require** a stable idempotency scope: most carry `idempotency_key` via `MachineRequestMeta` / `IdempotencyContext`. **`MachineTelemetryService.ReconcileEvents`** derives a deterministic synthetic key from a sorted fingerprint of `idempotency_keys` (duplicate ordering is harmless). That list includes commerce vend flows, inventory deltas, telemetry ingest including **`ReconcileEvents`**, offline replay, **`MachineBootstrapService.CheckIn`**, **`AckConfigVersion`**, **`AckCatalogVersion`**, **`AckInventorySync`**, **`AckMediaVersion`**, **`MachineOperatorService.HeartbeatOperatorSession`**, and command reporting RPCs. Spec-native RPC aliases (**`CreateCashCheckout`**, **`AttachPaymentResult`**, **`SubmitFillReport`**, **`SubmitStockAdjustment`**) share the **same PostgreSQL idempotency operation column** as their primary methods (**`ConfirmCashPayment`**, **`CreatePaymentSession`**, **`SubmitFillResult`**, **`SubmitInventoryAdjustment`**) so retries cannot double-apply across names. The unary interceptor persists rows in PostgreSQL **`machine_idempotency_keys`**: fingerprints use deterministic protobuf serialization after stripping unstable **`MachineRequestMeta.request_id`** fields (**`HashMutationRequest`**); raw `StableProtoHash` hashes the message as-is. Rows use lifecycle **`in_progress` → `succeeded`** (replayable **completed** response snapshot), **`failed`** when handlers return definitive errors, and **`FailedPrecondition` / `idempotency_payload_mismatch`** when the payload hash differs for the same logical key. Stale **`in_progress`** rows can be reclaimed after a bounded idle window so crash/network loss does not deadlock the ledger.

Successful **ledger replays** force outer response `replay=true` where the envelope carries that flag (first completion is stored with `replay=false`), so clients can distinguish network retries from first execution without relying on unstable nested fields alone.

## Offline replay

Queue durable events with strictly increasing `offline_sequence` per machine. **`MachineOfflineSyncService.PushOfflineEvents`** persists **`machine_offline_events`**, advances **`machine_sync_cursors`**, and dispatches supported `event_type` values through the same commerce/inventory/telemetry handlers as online RPCs (no parallel business logic in the sync service).

Duplicate **`client_event_id`** (on event `meta`) is idempotent: replays return **`REPLAYED`** without re-running side effects. If `offline_sequence` skips ahead of `last_sequence + 1`, the RPC fails with **`Aborted`** and a message like `offline sequence out of order: expected N got M` so clients can rewind or refill gaps before retrying.

## Migrating the Android vending app from HTTP to gRPC

1. **Enable native gRPC** in the client using `avf.machine.v1` services with **Machine JWT** (`authorization` metadata). Use the same issuer/audience model as documented above — admin User JWTs must not be sent on machine RPCs.
2. **Replace legacy machine REST** calls (`/v1/setup/...`, `/v1/machines/{id}/sale-catalog`, telemetry GETs, `/v1/device/machines/...`, `/v1/commerce/...`, operator session HTTP) with the matching gRPC RPCs. Those routes are registered only when **`ENABLE_LEGACY_MACHINE_HTTP=true`** (alias: `MACHINE_REST_LEGACY_ENABLED` if `ENABLE_*` unset). Default **on** in development/test, **off** in production unless explicitly enabled with **`MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION=true`**.
3. **Keep MQTT** for backend→device command delivery; command pull over HTTP (`POST .../commands/poll`) is a degraded-mode legacy bridge — primary delivery remains MQTT TLS + ledger.
4. **Production operators** must set `MACHINE_GRPC_ENABLED=true` and keep **`ENABLE_LEGACY_MACHINE_HTTP=false`** (or unset in production for the same default) unless a deliberate migration window uses `MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION=true`.

See also [`../architecture/transport-boundary.md`](../architecture/transport-boundary.md) (Runtime enforcement).

## Runtime sale catalog fingerprint (`catalog_version`)

- **`CatalogSnapshot.catalog_version`** is **`salecatalog.RuntimeSaleCatalogFingerprint`** (prefix **`runtime_sale_catalog_v4`**): it composes assortment + pricebook + planogram lineage + promotion placeholder (`prm:none`) + **`MediaFingerprint`** + **`InventorySnapshotFingerprint`** + shadow **`config_version`** + **currency** + snapshot flags **`include_unavailable`** / **`include_images`**. Treat it as the mobile **`catalog_fingerprint`** / ETag analogue.
- **`CatalogSnapshot.generated_at`** is snapshot UTC time; unary responses also include **`MachineResponseMeta.server_time`** for wall-clock skew checks.
- **`GetCatalogDelta`** builds with **`include_unavailable=true`** and **`include_images=true`** — store/compare **`basis_catalog_version`** only against fingerprints produced under the same projection semantics (or issue unconditional **`GetCatalogSnapshot`**).
- **`CheckForUpdates`** still exposes **separate** `ServerCatalogFingerprint`, `ServerPricingFingerprint`, … for lightweight polling; **`RuntimeSaleCatalogFingerprint`** is the **single authoritative** string covering all of those concerns for sale UI caches.
- **`GetMediaManifest`** attaches the same **`catalog_version`** composite plus a dedicated **`media_fingerprint`** (media-only **`MediaFingerprint`**). Oversized manifests return **`ResourceExhausted`** when entry count exceeds **`CAPACITY_MAX_MEDIA_MANIFEST_ENTRIES`** (validated range **[64,100000]**).

## Bootstrap / config acknowledgement

- **`GetBootstrapResponse`** includes **`published_planogram_version_id`** and **`published_planogram_version_no`** when enterprise planogram versioning has published at least once (**`machines.published_planogram_version_id`** → **`machine_planogram_versions`**).
- **`PlanogramFingerprint`** used by **`CheckForUpdates`** prefixes a **`pv:`** segment when a published planogram version exists so publish/rollback bumps **`planogram_changed`** independently of slot hashing.
- **`AckConfigVersion`** persists **`acknowledged_config_version`** to **`machine_current_snapshot.last_acknowledged_config_revision`** and optional **`acknowledged_planogram_version_id`** to **`last_acknowledged_planogram_version_id`** (best-effort; requires an existing snapshot row for the machine).

## Catalog snapshot media (`ProductMediaRef`)

**`MachineCatalogService.GetCatalogSnapshot`** / **`SyncSaleCatalog`** attach **`PrimaryMedia`** on each line item with **`ProductMediaRef`**:

| Field | Meaning |
| ----- | ------- |
| **`thumb_url`**, **`display_url`** | HTTPS URLs for kiosk fetch (from **`product_media`** projection when ingested via object storage; external bindings remain supported). |
| **`checksum_sha256`** | Integrity tag for local disk cache (merged projection hash / asset digest in **`salecatalog`**). |
| **`content_type`** | MIME type when known (**`MimeType`** from **`product_media`** projection). |
| **`media_version`** | Monotonic-ish version when bytes change (see **`product_media.media_version`** / **`product_images.media_version`**). |
| **`width`**, **`height`** | Pixel dimensions when known. |
| **`etag`**, **`updated_at`**, **`size_bytes`**, **`object_version`** | Optional HTTP-style freshness / object-store metadata. |
| **`deleted`** | Primarily for delta projections — defaults **false** on catalog snapshots. |
| **`expires_at`** | Optional CDN/cache TTL hint when wired end-to-end. |

Treat **`checksum_sha256`** + **`media_version`** as the stable cache key pair for offline image reuse across app restarts.

## Media manifest / delta (offline image cache)

- **`MachineMediaService.GetMediaManifest`** and **`GetMediaDelta`** project the same sale-catalog rows as HTTP **`/v1/machines/{id}/sale-catalog`** (via **`salecatalog.BuildSnapshot`**), including the **`include_unavailable`** toggle.
- Set **`GetMediaDeltaRequest.include_unavailable`** to the **same** value as **`GetMediaManifestRequest.include_unavailable`** when comparing **`basis_media_fingerprint`** to **`media_fingerprint`**; otherwise the server will treat the basis as stale.
- **`media_fingerprint`** hashes each projected line (product id, SKU, and the active image identity: media asset id + **`media_version`** + **`content_hash`**, or a **`deleted`** sentinel when the slot has no image).
- Manifest entries use **`ProductMediaRef`**: URLs point at **WebP** variants when the admin pipeline completed processing; **`checksum_sha256`** / **`media_version`** align with the sale catalog. **`deleted`** is **true** for tombstone rows (purge any cached image state for that **`product_id`**).
- When S3-compatible storage is wired, the API may **re-presign** HTTPS URLs per response and populate **`expires_at`** on **`ProductMediaRef`**.

## MQTT vs gRPC

Backend→machine **realtime commands** use **MQTT TLS** and the Postgres command ledger. **`MachineCommandService`** **`GetPendingCommands`** / **`AckCommand`** / **`RejectCommand`** remain deprecated stubs returning **`Unimplemented`** with explicit MQTT guidance — they must not be used as production polling substitutes.

## Smoke expectations

- Missing/invalid Machine JWT on protected RPCs → **`Unauthenticated`**
- Valid JWT, scope mismatch → **`PermissionDenied`**
- Contract gap → **`Unimplemented`** with structured details (never silent success)
