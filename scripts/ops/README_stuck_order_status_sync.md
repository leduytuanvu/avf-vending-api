# Stuck order status sync (ops)

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
