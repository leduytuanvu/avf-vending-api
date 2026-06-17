> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/cash-authority/`. System copy removed 2026-06-16.

# CASH_AUTHORITY_AMBIGUOUS — fix report

**Machine:** `019ec0d7-0a68-7bb0-ace0-f1d4dd3b0054`  
**Device:** `PFT9UY4Y59`  
**Classification:** `CA_PREPAYMENT_CHECKOUT_TOO_STRICTLY_BLOCKED_BY_LIVE_CASH_AUTHORITY`  
**Updated:** 2026-06-15 (CA3)

## Fix applied (production-safe, no bypass)

### 3A — Idle first-poll record buffer

``ICTBillDriver.kt``:

- When poll flags `record_buffer_full_possible_overflow` with **no active cash session** and **no new credit records**, log `BILL_STATE_DIAG` + warning but **do not** publish durable `bill_record_ambiguous` event.
- Active session, new records, misalignment, odd-length buffer, register jump — **unchanged** (still durable-lock).

### 3B — Prepayment vs live cash readiness split

``SellReadiness.kt``:

- Added `isPrepaymentStorefrontReady` — allows storefront when blockers are **only** `CASH_AUTHORITY_AMBIGUOUS` and/or `CASH_NOT_READY`.
- Live sale / BILL enable / payment confirm still gated on `isSellReady`.

``StorefrontReadinessObserver.kt``:

- Maps `isSalesBlocked = !isPrepaymentStorefrontReady` for browse/cart/checkout sheet.
- `StorefrontCheckoutHandler.liveSellReadyBlockMessage()` unchanged — blocks confirm/payment without live sell ready.

### Harness

``FreshInstallE2eAutomator``: `waitForBillRuntimeStableBeforeCheckout()` before checkout flow.

## Not done (by design)

- No fake BILL readiness or payment events
- No silent clear of `isAmbiguousLocked` during active session
- No vend/BILL enable in F7 prepayment path

## Verification (pending CA4 device rerun)

```powershell
cd avf-vending-app
.\gradlew.bat :app:testTcnProductionReleaseUnitTest
.\gradlew.bat :hardware:hardware-bill:test
.\gradlew.bat :app:assembleTcnProductionRelease
.\gradlew.bat :app:assembleTcnProductionReleaseAndroidTest
```

**Unit tests:** PASS (244 app + hardware-bill tests, 2026-06-15).

**Device reruns:** Blocked on commissioning wizard (see [`CASH_AUTHORITY_FULL_RERUN_REPORT.md`](CASH_AUTHORITY_FULL_RERUN_REPORT.md)).

## Expected post-fix markers

```
BILL_STATE_DIAG ... ambiguous=false reason=record_buffer_full_possible_overflow
PREPAYMENT_CHECKOUT_DECISION allowed=true liveSellReady=false
CHECKOUT_OPENED slot=A1
```

Live confirm checkout still blocked until `isSellReady=true` and operator tokens for F8.
