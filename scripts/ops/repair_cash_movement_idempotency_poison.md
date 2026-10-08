# Cash movement idempotency poison (bill_credit not on web)

When the app logs `idempotency_payload_mismatch` for `type=CASH` keys `cash_movement:<machineId>:…`, the server ledger may block replay while `cash_acceptance_events` has no row.

1. Run read-only sections in [`repair_cash_movement_idempotency_poison.sql`](repair_cash_movement_idempotency_poison.sql).
2. After deploy of app/API fixes, delete only poisoned `machine_idempotency_keys` rows that have no matching acceptance (see SQL section 5).
3. New bill inserts use boot-scoped keys and v2 re-key on mismatch; reconcile checks forensic tables.

Machine reference: `01a0a7e5-3c68-7895-b526-bcb6504bccfb` (AVF000195 session).
