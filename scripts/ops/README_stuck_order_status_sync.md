# Stuck order status sync (ops)

## Classify sync gap vs legitimate QR cancel (2026-10-15 `01a13f` prefix)

```bash
psql "$DATABASE_URL" -v machine_code='AVF000109' -v id_prefix='01a13f' \
  -f scripts/ops/classify_order_sync_status.sql
```

Columns `sync_class` and `target_status_hint` separate:

- `terminal_ok` — admin matches expected end state
- `likely_qr_abandon_or_orphan_created` — materialized, no capture (often `cancelled` after expire)
- `stale_vending_or_finalize_gap` / `rollup_stuck_or_missing_finalize` — machine finished but server non-terminal → check logcat + Pha B flush
- `unknown_if_machine_lost_session` — should become API `unknown` after reconciliation (crash, new checkout, TTL)

## Inspect AVF000109 / 01a132 cluster (2026-10-13)

```bash
psql "$DATABASE_URL" -f scripts/ops/inspect_avf000109_01a132_orders.sql
```

Do **not** run repair without hardware/logcat evidence.

## Repair paid/vending (evidence required)

```bash
psql "$DATABASE_URL" -v dry_run=1 -v order_id='<uuid>' \
  -f scripts/ops/repair_stuck_paid_vending_template.sql
```

Only when vend_sessions show `in_progress` but field confirms dispense completed/failed.

## Mark stale non-terminal as `unknown` (TTL)

```bash
psql "$DATABASE_URL" -v dry_run=1 -v stale_minutes=45 -v machine_code='AVF000109' \
  -f scripts/ops/mark_stale_non_terminal_orders_unknown.sql
```

Set `dry_run=0` only after reviewing candidates. Prefer machine `MarkOrderUnknown` when logcat shows normal checkout superseded or crash recovery.

## Monitor stale (no auto-complete)

- API: `scripts/ops/monitor_payment_reconciliation_stale_paid.sh [machine_code] [stale_hours]`
- DB: `scripts/ops/monitor_stale_paid_vending_orders.sql` with `stale_minutes` / `machine_code`

## Expire unpaid `created` / `quoted`

```bash
psql "$DATABASE_URL" -v dry_run=1 -v stale_minutes=30 -v machine_code='' \
  -f scripts/ops/expire_unpaid_created_orders.sql
```

Never use this script for `paid` or `vending`.

## Kiosk logcat (when adb available)

```bash
adb logcat -d | rg "POST_VEND_COMMERCE_FLUSH|OUTBOX_COMMERCE|VEND_CONFIRM_SKIPPED_TERMINAL|CommerceVendLineCloser"
```
