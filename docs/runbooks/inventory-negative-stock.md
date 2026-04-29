# Inventory negative stock (oversell prevention)

Use when **`AVFInventoryNegativeStockAttemptsSpike`** fires or refunds cite phantom inventory.

## What the alert means

**Metric:** `inventory_negative_stock_attempts_total` (canonical, API process).

**Threshold:** Production rule uses sustained rate > **0.02/s** over **15m** (tune per fleet size).

The counter increments when a mutation would drive **`machine_slot_state.current_quantity` negative**; Postgres layer rejects the write and records this metric.

## How to query

```promql
sum(rate(inventory_negative_stock_attempts_total{job="avf_api_metrics"}[15m]))
```

**Logs:** Inventory gRPC / offline replay; search **`machine_id`**, slot index, idempotency keys.

**DB:** `machine_slot_state`, recent `inventory_events`, vend outcomes for the slot.

## Immediate mitigation

1. Pause automated fills/adjustments for the affected machine if telemetry looks inconsistent.
2. Compare **planogram** vs **live slots** — wrong product mapping causes apparent oversell.
3. Check for **duplicate vend success** paths (MQTT + gRPC retry without idempotency).

## Safe manual recovery

- Correct quantity via audited operator fill / adjustment after field confirms physical count.
- Resolve any open **`inventory_reconciliation`** cases before clearing alerts.

## Escalation

- Widespread spike after release: treat as regression — roll back or feature-flag.
- Suspected fraud: security + retail ops.
