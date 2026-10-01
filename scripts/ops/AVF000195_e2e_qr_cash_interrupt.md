# E2E — QR→cash interrupt (AVF000195)

Verify fix for abandoned server order shell when MoMo QR is open and wallet cash becomes sufficient.

## Preconditions

- Install latest `assembleTcnProductionRelease` APK on AVF000195.
- Machine online (`api.ldtv.dev` reachable).
- Run repair SQL for historical order `01a0f85d` if not done yet.

## Steps

1. Add **2 products** to cart (e.g. 10k + 5k = 15k total).
2. Checkout → payment dialog opens with **QR MoMo**.
3. Insert cash until wallet balance ≥ cart total (do not scan QR).
4. Wait for auto-vend to complete on machine.

## Expected logcat

- `WALLET_AUTO_VEND_MATERIALIZED clientOrderId=... serverOrderId=...`
- **No** `BACKGROUND_MATERIALIZE_CANCELLED` before `GRPC_CREATE_ORDER_FROM_QUOTE_RESULT`
- `CASH_CHECKOUT_BACKEND_RESULT success=true` **or** `OFFLINE_SALE_REPLAY_ACCEPTED` within ~30s
- **No** sustained `OUTBOX_COMMERCE_PUSH_FAILED ... circuit breaker is open`

## Expected web (admin)

Within ~30s of vend complete:

- Order shows **completed** (or paid→completed)
- Payment column shows **cash**
- Both line items **Đã rơi** (success)

## Regression smoke

- QR-only payment (no cash insert) still completes.
- Wallet sufficient at checkout (no QR) still auto-vends.
- True offline (airplane mode) still enqueues `OFFLINE_SALE`.
