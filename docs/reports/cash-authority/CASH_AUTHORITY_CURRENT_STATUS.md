> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/cash-authority/`. System copy removed 2026-06-16.

# CASH_AUTHORITY — current status

**Machine:** `019ec0d7-0a68-7bb0-ace0-f1d4dd3b0054`  
**Device:** `PFT9UY4Y59`  
**Updated:** 2026-06-15 (CA0)

## Executive verdict

| Scope | Verdict |
|-------|---------|
| Focused F6/F7 | **`STOREFRONT_READY`** + **`CHECKOUT_PREPAYMENT_READY`** — ``072500Z`` |
| Full orchestrator | **`FULL_RUN_PROOF_PENDING_CASH_AUTHORITY_AMBIGUOUS`** — ``073255Z`` |
| F8 live sale | **`BLOCKED_OPERATOR_TOKEN_REQUIRED`** |

## Proven gates (do not re-litigate)

- Sticky STARTUP_FATAL fix = **DEVICE_VERIFIED** (`044038Z`)
- F5 / CONFIG_READY = **PASS** (`044128Z`, `072500Z`)
- `ServiceReadyHardwareConnectObserver` = **PASS** (`072500Z`)
- TCN reconnect after SERVICE_READY = **PASS** (`TCN_DRIVER_OPEN_SUCCESS` on `072500Z`)
- Focused checkout = cash-only, no vend, no BILL enable (`checkout-verdict.json` on `072500Z`)

## Blocker summary

After SERVICE_READY, runtime BILL reconnect completes startup reconciliation and first poll. On full orchestrator session (`073255Z`), first poll raises `BILL_RECORD_AMBIGUOUS reason=record_buffer_full_possible_overflow`, which durable-locks cash authority → `CASH_AUTHORITY_AMBIGUOUS` → storefront `salesBlocked=true` before checkout.

Focused run (`072500Z`) opened checkout **before** BILL init/poll completed on the same process (serial interrupt during identity read), so ambiguous lock did not latch in time.

Primary classification: **`CA_PREPAYMENT_CHECKOUT_TOO_STRICTLY_BLOCKED_BY_LIVE_CASH_AUTHORITY`**  
Contributing delta: **`CA_FOCUSED_RUN_RACE_OPENED_CHECKOUT_BEFORE_BILL_CONNECT`**

See [`CASH_AUTHORITY_AMBIGUOUS_ROOT_CAUSE.md`](CASH_AUTHORITY_AMBIGUOUS_ROOT_CAUSE.md).

## Backend metadata (both runs)

From `merged-layout.json`:

- `payment_authority`: **backend**
- `cash_topology`: **direct_bill**
- `bill_protocol`: **ict_bc_v1**
- `board_protocol`: **tcn**
- BILL port: `/dev/ttyS1`, TCN port: `/dev/ttyS4`

No backend metadata drift between focused pass and full fail.

## Allowed verdict caps

- `STOREFRONT_READY`
- `CHECKOUT_PREPAYMENT_READY`
- `FULL_RUN_PROOF_PENDING_CASH_AUTHORITY_AMBIGUOUS`
- `CASH_AUTHORITY_AMBIGUOUS_BLOCKS_PRODUCTION_CHECKOUT` (only if true safety blocker after fix attempt)
- `BLOCKED_OPERATOR_TOKEN_REQUIRED`

## Not claimed

- `PAYMENT_READY`, `SALE_READY`, `MARKET_READY`

## Key artifacts

| Run | Path | Outcome |
|-----|------|---------|
| Focused PASS | ``fresh-install-storefront-checkout-focused-20260615T072500Z/`` | F6/F7 prepayment |
| Full FAIL | ``fresh-install-e2e-20260615T073255Z/`` | Checkout blocked by `CASH_AUTHORITY_AMBIGUOUS` |

## Single source of truth

``POST_STICKY_FATAL_CURRENT_BLOCKERS.md``
