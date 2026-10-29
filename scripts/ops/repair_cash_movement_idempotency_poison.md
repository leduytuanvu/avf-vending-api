# Cash movement idempotency poison (bill_credit not on web)

When the app logs `idempotency_payload_mismatch` for `type=CASH` keys `cash_movement:<machineId>:…`, the server ledger may block replay while `cash_acceptance_events` has no row.

Typical 14:00 pattern: ICT `escrow_held` and `bill_credit` both used the same suffix (`B20204` / raw record hex). Typical 17:04 pattern: several `bill_credit` rows reused `BC0103` / `C00103` / `C20103` / `BE0103` and boot-scoped rekey went to v2 without a unique `client_event_id`, so `OUTBOX_STREAM_STALLED … commerceBlocked=true` and nạp/rút never appear on admin `/cash/ledger`. Payout `NOTE_CONFIRMED` sat behind that floor.

1. Run read-only sections in [`repair_cash_movement_idempotency_poison.sql`](repair_cash_movement_idempotency_poison.sql). Section 5 lists every `cash_movement` key without `cash_acceptance_events`. Section 6 lists payout keys without `cash_payout_events`.
2. **Deploy the APK that scopes cash keys by movement kind, uses `enqueueIfAbsent`, skips v2 rekey (always v3 + nonce on `client_event_id`), and heals 23505** **before** apply mode. If the old APK is still minting colliding `…:BC0103:bill_credit` rows, apply will re-poison.
3. After that APK is on the machine, run apply mode: deletes poisoned `machine_idempotency_keys` (tokens `920204`, `940105`, `BC0103`, `C00103`, `C20103`, `BE0103`) plus payout keys with no `cash_payout_events` row, and sets matching `machine_offline_events` to `failed_retryable` so PushOfflineEvents re-dispatches.
4. New bill inserts use boot-scoped, kind-scoped keys and boot-scoped `client_event_id`; mismatch recovery rekeys straight to v3. Reconcile checks forensic tables.

Machine reference: `01a0a7e5-3c68-7895-b526-bcb6504bccfb` (AVF000195 session).
