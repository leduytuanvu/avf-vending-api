> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/cash-authority/`. System copy removed 2026-06-16.

# CASH_AUTHORITY diagnostics added

**Machine:** `019ec0d7-0a68-7bb0-ace0-f1d4dd3b0054`  
**Updated:** 2026-06-15 (CA2)

## Markers added

| Marker | Location | Purpose |
|--------|----------|---------|
| `CASH_AUTHORITY_DECISION` | ``SellReadinessGate.kt`` | Logs ambiguous lock, block reasons, prepayment vs live sell result |
| `CASH_READINESS_DIAG` | same | `cashReady` + cash-related blockers |
| `BILL_STATE_DIAG` | ``ICTBillDriver.kt`` poll path | connected, rx, recycler, credit map, ambiguous suppression |
| `PREPAYMENT_CHECKOUT_DECISION` | ``StorefrontReadinessObserver.kt`` | prepayment tier allowed + live sell ready |
| `CHECKOUT_GATE_DECISION` | ``StorefrontReducer.kt`` | cart sheet open gate |

## Tests added

| Test | File |
|------|------|
| Prepayment ready when only `CASH_AUTHORITY_AMBIGUOUS` | `SellReadinessEvaluatorTest` |
| Prepayment blocked when hardware not ready | `SellReadinessEvaluatorTest` |
| Prepayment ready when only `CASH_NOT_READY` | `SellReadinessEvaluatorTest` |
| Idle baseline overflow suppression rules | `ICTBillDriverRecordAmbiguousTest` |

## Harness

``FreshInstallE2eAutomator.waitForBillRuntimeStableBeforeCheckout`` — waits for `BILL_ACCEPTOR_READY` + sustained `PREPAYMENT_CHECKOUT_DECISION allowed=true`.
