# Offline Replay (E2E Flow I) Failure Analysis

**UTC:** 20260706T034900Z (suite `20260706T034900Z`)  
**Classification:** **Test fixture bug** — not an activation-by-machineCode regression.

---

## Summary

E2E flow **I** failed with `activation_invalid` because it attempted to **re-claim an activation code already consumed in flow A**. One-time activation codes correctly reject reuse. Fix: create a **fresh** admin activation code before flow I claim.

---

## Failure evidence

| Field | Value |
|-------|-------|
| Flow | **I** — Offline replay idempotency |
| Error | `activation_invalid` on second claim |
| Root cause | Flow **A** (`bootstrap`) creates and **claims** code once; flow **I** reused `subst["activationCode"]` from registry |
| Production activation API | Expected behavior for consumed one-time code |

---

## Code path (before fix)

`tools/production_full_test/run_production_e2e_flows.py`:

- Flow A: `bootstrap()` → `admin_post(.../activation-codes)` → `claim_activation()` (consumes code)
- Flow I: `claim_activation(base_url, subst["activationCode"], ...)` → **fail**

---

## Fix (E2E tooling only)

Before flow I:

1. `admin_post(.../machines/{machine_id}/activation-codes, {...})` for a fresh code
2. `claim_activation()` with the new plaintext code
3. Pass when claim returns `machineToken` / `accessToken`

No activation feature or API contract change.

---

## Retest expectation

After fix merges, flow I should **pass** in `run_production_e2e_flows.py` when admin credentials and bootstrap machine are available.

---

## References

- [07_RETEST_AND_FIX_REPORT.md](07_RETEST_AND_FIX_REPORT.md) — flow I previously marked out of scope
- Bootstrap claim: `tools/production_full_test/bootstrap_test_data.py`
