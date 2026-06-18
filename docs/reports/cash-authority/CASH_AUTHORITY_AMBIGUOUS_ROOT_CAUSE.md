> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/cash-authority/`. System copy removed 2026-06-16.

# CASH_AUTHORITY_AMBIGUOUS — root cause

**Machine:** `019ec0d7-0a68-7bb0-ace0-f1d4dd3b0054`  
**Device:** `PFT9UY4Y59`  
**Updated:** 2026-06-15 (CA1)

## Primary classification

**`CA_PREPAYMENT_CHECKOUT_TOO_STRICTLY_BLOCKED_BY_LIVE_CASH_AUTHORITY`**

**Contributing delta:** **`CA_FOCUSED_RUN_RACE_OPENED_CHECKOUT_BEFORE_BILL_CONNECT`**

Not classified: backend mapping conflict, recycler_count_zero alone, stale payment authority metadata, or UNKNOWN.

---

## Emitter (exact code path)

| Layer | File | Function / rule |
|-------|------|-----------------|
| Verdict | ``SellReadinessEvaluator.kt`` L141–148 | Rule: `if (s.billRecyclerAmbiguousLocked)` → `SellBlockReason.CASH_AUTHORITY_AMBIGUOUS` |
| Snapshot input | ``SellReadinessDependencySourceImpl.kt`` L325 | `billRecyclerAmbiguousLocked = recyclerState.isAmbiguousLocked` |
| Durable lock | ``HardwareEventObserver.kt`` L56–57 | `bill_record_ambiguous` → `recyclerStateRepository.applyBillAmbiguous()` |
| Persistence | ``RecyclerStateDataStore.kt`` L126–131 | Sets `AMBIGUOUS_LOCKED=true` |
| Trigger | ``ICTBillDriver.kt`` L1037–1047 | First poll after READY publishes `bill_record_ambiguous` when tracker flags ambiguous |
| Tracker | ``BillPollRecordTracker.kt`` L120–122 | `current.size >= maxRecords` → `record_buffer_full_possible_overflow` |

Storefront gate: ``StorefrontContract.purchaseInteractionEnabled`` requires `!isSalesBlocked`, which mirrors full sell readiness today.

---

## Decision inputs (full run pid 4916)

| Input | Value |
|-------|-------|
| Backend `payment_authority` | `backend` (``merged-layout.json``) |
| App `cashTopology` | `direct_bill` (`BOOTSTRAP_METADATA_SUMMARY`) |
| `billRecyclerAmbiguousLocked` | **true** (after `applyBillAmbiguous`) |
| `recyclerState.isAmbiguousLocked` | **true** |
| BILL connected before ambiguous | **yes** — `BILL_DRIVER_OPEN_SUCCESS` 15:36:10.988 |
| Recycler count (hardware) | **0** — `NOT_READY_BILL_RECYCLER_CHANGE_10000 reason=recycler_count_zero` (change-payout marker only) |
| BILL RX proven | **yes** — identity + poll success before ambiguous |
| Recycler available query | **yes** — `BILL_RECYCLER_READY` with count=0 |
| Credit map | **ready** — `BILL_TYPE_CREDIT` types 1–6 logged |
| Ambiguous reason | `record_buffer_full_possible_overflow` from first post-READY poll |

---

## Timeline — full fail (`073255Z`)

| Time (local log) | Event |
|------------------|-------|
| 15:35:59 | `SERVICE_READY_HARDWARE_CONNECT reason=service_ready` |
| 15:36:00 | Sustained `salesBlocked=false`, sellable=10 |
| 15:36:10.988 | `BILL_DRIVER_OPEN_SUCCESS port=/dev/ttyS1` (runtime reconnect) |
| 15:36:12.416 | `BILL_ACCEPTOR_READY`, `BILL_RECYCLER_READY`, `NOT_READY_BILL_RECYCLER_CHANGE_10000 reason=recycler_count_zero` |
| 15:36:12.551 | **`BILL_RECORD_AMBIGUOUS reason=record_buffer_full_possible_overflow`** |
| 15:36:16.299 | **`CASH_AUTHORITY_AMBIGUOUS`** — `salesBlocked=true` |
| 15:36:20.288 | `ADD_TO_CART_CLICKED` — checkout sheet not opened |

Instrumentation failure: `AssertionError: Checkout sheet visible` (``FreshInstallFullPipelineInstrumentedTest``).

---

## Timeline — focused pass (`072500Z`, pid 602)

| Time | Event |
|------|-------|
| 15:28:15 | `STOREFRONT_RENDERED salesBlocked=false`, sellable=10 |
| 15:28:26.003 | `BILL_DRIVER_OPEN_SUCCESS` — identity read **started** |
| 15:28:27.112 | Serial EOF during bill-type credit fetch — init **interrupted** |
| 15:28:34.502 | `ADD_TO_CART_CLICKED` |
| 15:28:37.180 | **`CHECKOUT_OPENED`** — cash-only, no vend/BILL enable |
| 15:28:39 | `CHECKOUT_CANCELLED_OR_BACK_TO_STOREFRONT` |

On pid 602: **no** `BILL_ACCEPTOR_READY` / **no** `BILL_RECORD_AMBIGUOUS` before checkout. Ambiguous latched later on restarted process (pid 2540) after test completed.

---

## Why focused passed but full failed

Same machine, same metadata, same APK generation:

1. **Full run** allowed BILL startup reconciliation + first poll to complete after SERVICE_READY reconnect (~2s after open).
2. First poll saw a full 6-record status buffer (idle escrow zeros) → tracker flagged overflow → durable ambiguous lock.
3. **`waitForSalesUnblocked`** passed **before** ambiguous latched (~15:36:00), then BILL reconnect completed and blocked sales before cart (~15:36:16).
4. **Focused run** checkout raced ahead of completed BILL init on the same PID — ambiguous lock never applied before `CHECKOUT_OPENED`.

This is a **timing/consistency** gap between harness pass criteria and post-reconnect BILL safety latch, not a backend metadata regression.

---

## Is ambiguity a true production safety blocker?

| Concern | Assessment |
|---------|------------|
| Live cash acceptance / BILL enable | **Yes** — ambiguous record buffer during/after cash-moving operations must block until operator clears with evidence |
| Idle post-reconnect full buffer (no new credit, no active session) | **Over-blocked** — logging `NOT_READY_BILL_RECYCLER_CHANGE_10000` and internal tracker warning is appropriate; **durable lock blocking entire storefront/prepayment cart is too strict** |
| Prepayment checkout (F7: open sheet, cash-only UI, no BILL enable, no vend) | **Should remain allowed** while live cash authority unresolved |

**Verdict:** Not a reason to bypass active-session ambiguous signals. Fix by (a) not durable-locking on benign idle first-poll overflow, and (b) splitting prepayment storefront readiness from live cash sale readiness.

---

## recycler_count_zero note

`NOT_READY_BILL_RECYCLER_CHANGE_10000 reason=recycler_count_zero` appears in **both** runs after BILL connect. It does **not** set `isAmbiguousLocked`. It indicates empty recycler change capacity — relevant for live change payout, not the `CASH_AUTHORITY_AMBIGUOUS` blocker in this incident.

---

## Artifacts

| Artifact | Path |
|----------|------|
| Full run logcat | ``fresh-install-e2e-20260615T073255Z/logcat-instrument-com-avf-vending-FreshInstallFullPipelineInstrumentedTest.log`` |
| Focused logcat | ``fresh-install-storefront-checkout-focused-20260615T072500Z/logcat-instrument-com-avf-vending-FreshInstallStorefrontCheckoutInstrumentedTest.log`` |
| Focused checkout verdict | ``checkout-verdict.json`` |
| App local state (focused) | ``app-local-state.json`` |

---

## Hard gate for CA3

Classification is **not** `CA_UNKNOWN_NEEDS_MORE_LOGGING`. Proceed to diagnostics + fix.
