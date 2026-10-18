# Cash movement idempotency repair — machine `01a11014-4c22-7632-9a53-6d3f5a171663`

Field log **2026-10-15** showed repeated `idempotency_payload_mismatch` on `cash_movement:*` keys for this machine, then sync circuit breaker open.

## Steps

1. Copy [`repair_cash_movement_idempotency_poison.sql`](repair_cash_movement_idempotency_poison.sql) and set:

   ```text
   \set machine_id '01a11014-4c22-7632-9a53-6d3f5a171663'
   ```

2. Run sections **1–4** (read-only) against production with a read-only DB role. Confirm poisoned keys and missing `cash_acceptance_events` rows match the incident.

3. After app deploy (v2→v3 cash rekey + boot-scoped enqueue), run section **5** (apply) per runbook in [`repair_cash_movement_idempotency_poison.md`](repair_cash_movement_idempotency_poison.md).

4. On device: trigger outbox sync or wait for replay; verify `OUTBOX_PUSH_ACCEPTED` for cash movements and no sustained `Sync circuit breaker is open`.

**Note:** This environment has no production Postgres credentials; execute the SQL from an operator workstation with VPN/DB access.
