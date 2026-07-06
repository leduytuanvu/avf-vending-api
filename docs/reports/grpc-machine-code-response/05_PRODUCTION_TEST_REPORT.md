# 05 — Production test report

**UTC:** `20260706T013826Z` (full suite) + smoke `20260706T014725Z`  
**Production SHA:** `2cc5569e1beebbff218848ab6ac42da952a489e5` (verified via `/version` in security suite)

## Full suite (3 passes)

| Surface | Per pass | 3-pass total | fail |
|---------|----------|--------------|------|
| REST | 363 / 363 | 1089 | 0 |
| gRPC | 75 / 75 | 225 | 0 |
| MQTT | 17 / 17 | 51 | 0 |

Source: `reports/production-full-api-grpc-mqtt/20260706T013826Z/FINAL_PRODUCTION_REST_GRPC_MQTT_VERDICT.json`

- E2E flows: 9 / 9 ok
- Security auth: fail=0
- DB verification: fail=0
- Fake-pass audit: risk=false
- Verdict: `PRODUCTION_REST_GRPC_MQTT_100_PERCENT_PASS`

## gRPC machine_code smoke

Source: `docs/reports/grpc-machine-code-response/evidence/grpc_machine_code_smoke.json`

| Step | Result |
|------|--------|
| ClaimActivation `machine_code` | pass |
| RefreshMachineToken `machine_code` | pass |
| GetBootstrap `machine.machine_code` | pass |
| MQTT username == UUID | pass |

**pass_count:** 5, **fail_count:** 0

## Machine-code activation smoke

`run_machine_code_activation_prod.py` — all 13 steps pass (admin paths + REST claim).

## Deploy (redeploy)

| Item | Value |
|------|-------|
| Promote PR | #435 develop → main |
| Build run | `28762078596` |
| Security Release | `28762213877` |
| Deploy Production | `28762275156` (success, SLO pass) |
| Migrations | skipped (`run_migration=false`) |
| Staging gate | bypass documented in deploy summary |
