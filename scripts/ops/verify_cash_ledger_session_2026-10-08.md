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

## Follow-up engineering (shipped 2026-10-08)

**Root cause:** `PushOfflineEvents` idempotency cached success while `ReportCashMovements` ingested **0** rows (proto payload without `deviceEventId` / replay skip on `machine_offline_events.processed`).

**Fixes:**

- API `05fb776c`: reject empty ingest (`invalid cash movements payload`); ops repair also sets `machine_offline_events` → `failed_retryable` for bill `cash_movement` keys without acceptance.
- App `d45355a9`: always remap `commerce.report_cash_movements` wire payload via `OutboxCanonicalMapper`.

**Prod actions triggered:**

- Deploy: workflow `production-self-hosted-build-deploy` run [37735008385](https://github.com/leduytuanvu/avf-vending-api/actions/runs/37735008385)
- Repair (apply): `production-repair-cash-idempotency-poison` run [37735012361](https://github.com/leduytuanvu/avf-vending-api/actions/runs/37735012361)

**On device after API deploy + app build with `d45355a9`:**

1. Dead-letter or fix outbox `CASH` seq **101** (`291a2dfd…:2E0204`) if still `invalid cash movements payload`.
2. Force sync / restart app; watch `outbox_push_accepted` and no `OUTBOX_REPLAY_STALLED_AT_HEAD`.
3. Re-run `production-verify-cash-ledger-session` for the test window (or a new field session).

Payout notes 2–3 still depend on unblocking CASH floor and sync.
