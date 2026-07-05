# Retest and Fix Report

Date: 2026-07-06

## Summary

No code fixes required for machine-code activation after production testing. REST/gRPC/MQTT full suites and activation smoke passed with zero failures.

---

## Issues encountered during verification

### 1. Activation smoke login token shape

| Field | Value |
|-------|-------|
| Surface | deployment / smoke script |
| Root cause | Login response uses `tokens.accessToken` nested field |
| Fix | PR #425 — parse nested tokens |
| Retest | **pass** |

### 2. Bootstrap machine codes vs activation `{6}$` regex

| Field | Value |
|-------|-------|
| Surface | E2E / activation smoke |
| Root cause | `production_machine_code()` generates `AVF` + >6 digits; activation admin resolver accepts only `^AVF[0-9]{6}$` |
| Fix | Smoke script creates isolated machine with 6-digit `AVF######` code |
| Retest | **pass** |

### 3. E2E flow I — offline replay idempotency (pre-existing)

| Field | Value |
|-------|-------|
| Surface | E2E |
| Status | **not retested / out of scope** for machine-code activation |
| Notes | Does not block REST/gRPC/MQTT surface verification or activation-by-machineCode |

---

## Retest results

| Suite | Before | After |
|-------|--------|-------|
| Activation smoke | blocked (login/registry) | **12/12 pass** |
| REST full production | 363/363 pass | unchanged |
| gRPC full production | 75/75 pass | unchanged |
| MQTT full production | 17/17 pass | unchanged |

No redeploy required for smoke script fixes (tooling only). API deploy for `MachineIdentityRef` via PR #426 pending at time of this report.
