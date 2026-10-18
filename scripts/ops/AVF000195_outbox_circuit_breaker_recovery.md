# AVF000195 — Outbox circuit breaker recovery

When logcat shows `OUTBOX_COMMERCE_PUSH_FAILED ... Sync circuit breaker is open`, commerce replay
(`offline-sale`, `offline-vend-*`) is blocked until the breaker closes.

After deploying API fix for `vend_hardware_evidence.raw` jsonb bind (22P02 → `vend_close_persistence_failed`)
and the app vend idempotency recovery build:

1. Inspect head-of-line outbox row (`seq=131` or current floor): note `event_type`, `idempotency_key`, `last_error`.
2. If `last_error` is `vend_close_persistence_failed` or `idempotency_payload_mismatch` on `VEND`, allow one sync
   cycle so `OUTBOX_VEND_CLOSE_FAILURE_REKEYED` / `OUTBOX_VEND_MISMATCH_REKEYED` can advance the queue.
3. If head is poisoned `CASH` idempotency, follow
   [`repair_cash_movement_idempotency_poison_01a11014.md`](repair_cash_movement_idempotency_poison_01a11014.md).

## Checklist

1. Confirm network to `api.ldtv.dev` is stable from the machine.
2. Reboot the vending app (or full device reboot) after connectivity is healthy.
3. Monitor logcat for:
   - `OFFLINE_SALE_REPLAY_ACCEPTED`
   - `outbox_push_accepted`
   - `GRPC_CONFIRM_VEND_RESULT`
4. If tasks remain in dead-letter, use technician diagnostics / local outbox view to replay
   pending commerce rows for keys:
   - `offline-sale:c1ce683f-e91c-4bb4-aacd-ee9083001dce`
   - `offline-vend-success:c1ce683f-*`
5. After installing the fixed APK, prefer inline `CASH_CHECKOUT_BACKEND_RESULT` over outbox for
   QR→cash interrupt orders.

## ADB log filter (optional)

```bash
adb logcat -s BusinessOutbox WalletAutoVend StorefrontVend CommerceGrpc WalletAutoVendResolver
```
