# Final Verdict — Machine Runtime Fleet

**Date:** 2026-07-04  
**Evidence bundle:** `reports/production-full-api-grpc-mqtt/20260703T234500Z/`  
**Prefix:** `AVF-RUNTIME-FLEET-20260703T234500Z`

## Verdict

**`PRODUCTION_REST_GRPC_MQTT_RUNTIME_FLEET_100_PERCENT_PASS`**

## Production state

| Item | Value |
|------|-------|
| Deployed SHA (live `/version`) | `51485f5583a4f550cfe6fdb6e529e7339daad9ca` |
| Runtime fleet base merge | `277a3ad4` (PR #410) + migrations 00017/00018 |
| Timeline hotfix | PR #411 → `51485f55` |
| Goose version | 18 |

## Automated gate summary

| Gate | Evidence |
|------|----------|
| Deploy + migrations | `09_PRODUCTION_DEPLOY_REPORT.md`, `08_PRODUCTION_BACKUP.md`, `10_POST_DEPLOY_HEALTH.md` |
| REST 100% (352 ops) | `REST_FINAL_COVERAGE.json` fail=0 |
| gRPC 100% (75 RPCs) | `GRPC_FINAL_COVERAGE.json` fail=0 |
| MQTT 100% (17 contracts) | `MQTT_FINAL_COVERAGE.json` fail=0 |
| DB/security/fake-pass | `DATABASE_STATE_VERIFICATION.json`, `SECURITY_AUTH_TEST_RESULTS.json`, `FAKE_PASS_AUDIT.json` |
| 3-pass | `MULTI_PASS_PRODUCTION_VALIDATION.json` — 3/3 ok |
| E2E flows | `E2E_FLOW_RESULTS.json` — 9/9 ok |

## Checklist notes (honest partial coverage)

Items covered by current-run matrices and E2E: deploy, migrations, REST/gRPC/MQTT surfaces, runtime session gRPC inline RPCs, claim/reattach/compromised MQTT, ops-overview read-back, security RBAC smoke, multi-pass stability.

Items with **partial** dedicated evidence (harness gaps documented in `11_PRODUCTION_FULL_SUITE_X3.md`): full fingerprint reattach field matrix, technician multi-machine negatives, direct DB column assertions, exhaustive timeline/fleet-filter matrices.

## Reports index

| Doc | Purpose |
|-----|---------|
| `06_PRE_DEPLOY_AUDIT.md` | Pre-deploy readonly gate |
| `07_DEPLOY_INPUTS.md` | Build/security/deploy inputs |
| `08_PRODUCTION_BACKUP.md` | Inline pg_dump evidence |
| `09_PRODUCTION_DEPLOY_REPORT.md` | Primary deploy run |
| `10_POST_DEPLOY_HEALTH.md` | Post-deploy probes |
| `11_PRODUCTION_FULL_SUITE_X3.md` | 3× suite + triage |
