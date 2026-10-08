# Verify cash ledger — field session 2026-10-08 ~12:37 VN

Machine `01a0a7e5-3c68-7895-b526-bcb6504bccfb`, withdrawal `a1803384-dfb8-4d2d-89ec-7cef515eeb85`.

## Prod results (workflow [37733909888](https://github.com/leduytuanvu/avf-vending-api/actions/runs/37733909888))

| Check | Expected (plan) | Actual |
|-------|-----------------|--------|
| `bill_credit` 10k in window | ≥ 3 | **0** |
| Payout for withdrawal | 3 notes | **2** rows (note 1: `NOTE_COMMAND_ACCEPTED`, `NOTE_CONFIRMED`) |
| Admin `GET /v1/admin/cash/ledger` | matches SQL | **200**, 2 items, **0** `bill_credit` |
| Idempotency `291a2dfd…:2E0204` | stall or poison | **succeeded** but **no** `cash_acceptance_events` row |

**Verdict:** Session **does not meet** “3 nhận / 3 rút” on server or web admin.

## Web admin (self-check)

Deep link (same window as SQL/API):

`https://admin.ldtv.dev/cash/ledger?machineId=01a0a7e5-3c68-7895-b526-bcb6504bccfb&from=2026-10-08T05:30:00.000Z&to=2026-10-08T06:15:00.000Z`

Expect: payout journal lines only (~2), **no** `bill_credit` rows — consistent with API.

## Device outbox seq=101 (`2E0204` / boot `291a2dfd…`)

Log: `OUTBOX_SEQUENCED_NON_COMMERCE_PUSH_FAILED` → `invalid cash movements payload` (protojson cannot parse legacy snake_case / shape).

**On device (technician):**

1. Export `BusinessOutbox` row `type=CASH`, `sequence_no=101`, key `cash_movement:…:291a2dfd-bb5a-4df4-920a-93812ba45f20:2E0204`.
2. If payload uses `device_event_id` / snake_case → **dead-letter** after one canonical remap attempt, or fix payload per `OutboxCanonicalMapper` and retry sync.
3. Do **not** delete `machine_idempotency_keys` rows that already ingested acceptance; for keys **succeeded** with **no** acceptance row, run read-only section “Idempotency succeeded without acceptance” in [`verify_cash_ledger_session.sql`](verify_cash_ledger_session.sql), then optional poison DELETE per [`repair_cash_movement_idempotency_poison.sql`](repair_cash_movement_idempotency_poison.sql) section 5.

## Follow-up engineering

- Investigate why `outbox_push_accepted` / idempotency `succeeded` does not create `cash_acceptance_events` (empty `events[]` after protojson, wrong `kind`, or replay skipping ingest).
- Complete payout sync for notes 2–3 (`ImmediateSyncWorker` / unblock CASH floor).

Re-run verify: workflow `production-verify-cash-ledger-session` or `bash scripts/ops/verify_cash_ledger_session.sh` on prod runner.
