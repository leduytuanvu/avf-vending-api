# Numeric Activation Code — Production Test Report

**UTC:** 20260706T103058Z (full suite) + 20260706T103900Z (activation smoke re-run)  
**Production SHA:** `999b9e93e4acc63dacb2c8087bc0a8ea47316a00`

## Summary

| Surface | Total | Passed | Failed | Skipped | Blocked | Not run |
|---------|-------|--------|--------|---------|---------|---------|
| REST | 363 | 363 | 0 | 0 | 0 | 0 |
| gRPC | 75 | 75 | 0 | 0 | 0 | 0 |
| MQTT | 17 | 17 | 0 | 0 | 0 | 0 |
| Numeric activation smoke | 16 | 16 | 0 | 0 | 0 | 0 |
| gRPC machine_code smoke | 5 | 5 | 0 | 0 | 0 | 0 |
| Security/fake-pass audit | — | pass | 0 | — | — | — |
| E2E flows | 9 | 9 | 0 | 0 | 0 | 0 |

Multi-pass: **3/3** REST/gRPC/MQTT passes with zero failures.

## Numeric activation smoke (required)

Runner: `tools/production_full_test/run_machine_code_activation_prod.py` (exit **0** after runner fix)

- Admin create returns 6-digit `activationCode` (len=6, string type)
- Claim with created code succeeds; `machineCode` remains `AVF...`; `machineId` UUID
- `deviceAttachmentId`, machine token, MQTT credentials present on claim
- Reject AVF-style → `activation_invalid` (HTTP 400)
- Reject 5-digit → `activation_invalid`
- Reject 7-digit → `activation_invalid`

Evidence: `docs/reports/machine-code-activation-production/evidence/activation_smoke_results.json`

## Full suite evidence

`reports/production-full-api-grpc-mqtt/20260706T103058Z/`

- `REST_FINAL_COVERAGE.json` — 363/363
- `GRPC_FINAL_COVERAGE.json` — 75/75
- `MQTT_FINAL_COVERAGE.json` — 17/17
- `FAKE_PASS_AUDIT.json` — no fake-pass risk
- `E2E_FLOW_RESULTS.json` — 9/9 flows OK

## Runner note

First full-suite orchestrator exit was **1** due to test-runner bugs (not API failures):

1. `ACTIVATION_CODE_RE` → `MACHINE_CODE_RE` rename incomplete in one guard
2. Negative claim assertions expected top-level `code` but API returns `error.code`

Both fixed in `run_machine_code_activation_prod.py`; activation smoke re-run **exit 0**.

## Version check

`/version` `git_sha` = `999b9e93e4acc63dacb2c8087bc0a8ea47316a00` (matches deployed commit).
