# E2E — QR→cash interrupt (AVF000195)

Verify fix for abandoned server order shell when MoMo QR is open and wallet cash becomes sufficient.

## Preconditions

- Install latest `assembleTcnProductionRelease` APK on AVF000195.
- Machine online (`api.ldtv.dev` reachable).
- Deploy backend with `ConfirmVendSuccess` legacy-slot fallback (auto-provision from `machine_slot_configs`).
- Bootstrap legacy inventory for AVF000195 if `machine_slot_state` is empty:
  ```bash
  psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/bootstrap_avf000195_legacy_slot_state.sql
  psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/bootstrap_avf000195_legacy_slot_state.sql
  ```
- Reset outbox circuit breaker if needed: see `AVF000195_outbox_circuit_breaker_recovery.md`.
- Run repair SQL for historical orders `01a0f85d` / `01a0f8a2` if not done yet.

## Steps

1. Add **2 products** to cart (e.g. 10k + 5k = 15k total).
2. Checkout → payment dialog opens with **QR MoMo**.
3. Insert cash until wallet balance ≥ cart total (do not scan QR).
4. Wait for auto-vend to complete on machine.

## Expected logcat

- `WALLET_AUTO_VEND_MATERIALIZED clientOrderId=... serverOrderId=...`
- **No** `BACKGROUND_MATERIALIZE_CANCELLED` before `GRPC_CREATE_ORDER_FROM_QUOTE_RESULT`
- `GRPC_CREATE_CASH_CHECKOUT_RESULT paymentState=captured`
- `GRPC_START_VEND_RESULT vendState=in_progress` for **each** line
- `VEND_LINE_SUCCESS` for **each** line
- `GRPC_CONFIRM_VEND_RESULT` × **2** (required — **no** `GRPC_CONFIRM_VEND_ERROR ... no machine_slot_state`)
- **No** `GRPC_CONFIRM_VEND_ERROR` with `INTERNAL: postgres: no machine_slot_state`
- `VEND_DEFERRED_SUCCESS_FLUSH` × 2 when inline confirm was deferred during sequence
- `CASH_CHECKOUT_BACKEND_RESULT success=true` **or** `OFFLINE_SALE_REPLAY_ACCEPTED` within ~30s
- **No** sustained `OUTBOX_COMMERCE_PUSH_FAILED ... circuit breaker is open`
- **No** pattern of only `offline_vend_replay_enqueued` without subsequent `outbox_push_accepted`

## Expected web (admin)

Within ~30s of vend complete:

- Order shows **completed** (or paid→completed)
- Payment column shows **cash**
- Both line items **Đã rơi** (success)

## Regression smoke

- QR-only payment (no cash insert) still completes.
- Wallet sufficient at checkout (no QR) still auto-vends.
- True offline (airplane mode) still enqueues `OFFLINE_SALE`.
