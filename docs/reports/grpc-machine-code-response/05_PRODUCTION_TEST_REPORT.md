# 05 — Production test report

**UTC:** `20260706T005447Z`  
**Production SHA:** `98169070b234d2940d8aab767d2dd25e52a85d11` (verified via `/version` in security suite)

## Full suite (3 passes)

| Surface | Per pass | 3-pass total | fail |
|---------|----------|--------------|------|
| REST | 363 / 363 | 1089 | 0 |
| gRPC | 75 / 75 | 225 | 0 |
| MQTT | 17 / 17 | 51 | 0 |

Source: `reports/production-full-api-grpc-mqtt/20260706T005447Z/FINAL_PRODUCTION_REST_GRPC_MQTT_VERDICT.json`

- E2E flows: 9 / 9 ok
- Security auth: fail=0
- DB verification: fail=0
- Fake-pass audit: risk=false

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

`run_machine_code_activation_prod.py` — all 12 steps pass (admin paths + REST claim).

## Deploy

| Item | Value |
|------|-------|
| Build run | `28760605207` |
| Security Release | `28760798022` |
| Deploy Production | `28760850147` (success) |
| Migrations | skipped (`run_migration=false`) |
