> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/cash-authority/`. System copy removed 2026-06-16.

# CASH_AUTHORITY — full rerun report

**Machine:** `019ec0d7-0a68-7bb0-ace0-f1d4dd3b0054`  
**Device:** `PFT9UY4Y59`  
**Updated:** 2026-06-15 (CA4)

## Code verification (local)

| Check | Result |
|-------|--------|
| `:app:testTcnProductionReleaseUnitTest` | **PASS** (244 tests) |
| `:hardware:hardware-bill:test` | **PASS** (includes `ICTBillDriverRecordAmbiguousTest`) |
| `:app:assembleTcnProductionRelease` | **PASS** |
| `:app:assembleTcnProductionReleaseAndroidTest` | **PASS** |

## Device focused reruns (post-fix)

| Run | Artifact | Outcome | Failure point |
|-----|----------|---------|---------------|
| 1 | [`094321Z`](fresh-install-storefront-checkout-focused-20260615T092129Z/) | **INSTRUMENTATION_FAILED** | `SERVICE_READY after ConfigApply` — MachineSetup wizard timeout (~16 min) |
| 2 | [`094321Z`](fresh-install-storefront-checkout-focused-20260615T094321Z/) | **INSTRUMENTATION_FAILED** | `Port bindings persisted` — commissioning step before F6/F7 |

Neither rerun reached F6/F7 checkout. Failures are **commissioning/wizard automation**, not `CASH_AUTHORITY_AMBIGUOUS`.

## Full orchestrator

Not rerun in this session — blocked on same fresh-install commissioning instability after consecutive failed focused attempts.

## Prior authoritative proof (unchanged)

| Run | Verdict |
|-----|---------|
| [`072500Z`](fresh-install-storefront-checkout-focused-20260615T072500Z/) | **CHECKOUT_PREPAYMENT_READY** (pre-fix baseline) |
| [`073255Z`](fresh-install-e2e-20260615T073255Z/) | **FULL_RUN_PROOF_PENDING_CASH_AUTHORITY_AMBIGUOUS** (root cause classified + fix applied) |

## Expected post-fix behavior (when F5 completes)

1. Idle BILL poll: `BILL_STATE_DIAG ... suppressed=durable_lock_idle_baseline` — no `CASH_AUTHORITY_AMBIGUOUS` from benign overflow
2. `PREPAYMENT_CHECKOUT_DECISION allowed=true` even if `liveSellReady=false` due to cash authority
3. Full orchestrator F7: `CHECKOUT_OPENED` without checkout blocked by prepayment tier

## Verdict caps

| Scope | Status |
|-------|--------|
| Root cause + fix | **Complete** (CA1–CA3) |
| Focused device proof (post-fix) | **PENDING** — commissioning harness blocked |
| Full orchestrator | **FULL_RUN_PROOF_PENDING_CASH_AUTHORITY_AMBIGUOUS** until clean F5→F7 rerun |
| F8 | **BLOCKED_OPERATOR_TOKEN_REQUIRED** |

## Not claimed

- Post-fix device F6/F7 PASS
- Full orchestrator end-to-end PASS
- `PAYMENT_READY`, `SALE_READY`, `MARKET_READY`

## Next

Retry focused + full orchestrator when commissioning wizard completes reliably on clean install (same as prior successful `072500Z` conditions).
