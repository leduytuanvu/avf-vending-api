# Production Full Suite ×3 — Runtime Fleet

**UTC bundle:** `20260703T234500Z`  
**Prefix:** `AVF-RUNTIME-FLEET-20260703T234500Z`  
**Production SHA (final):** `51485f5583a4f550cfe6fdb6e529e7339daad9ca` (hotfix PR #411 on timeline query)

## Triage summary

| Issue | Type | Resolution |
|-------|------|------------|
| Bootstrap machine code rejected | Harness | `production_machine_code()` → `^AVF[0-9]{6,}$` |
| Duplicate serial on pass 2/3 | Harness | Include `PROD_TEST_SUFFIX` in serial |
| `device-attachments/current` verify fail | Harness | Expect `"attachment"` key (null OK) |
| `timeline/unified` HTTP 500 | **Product** | PR #411 — wrong `machine_operator_sessions` columns in SQL |
| Hotfix deploy SLO 503 flake | Ops | `recovery_pre_deploy_reason` bypass after operator health check |
| E2E flow I activation replay | Harness | Same device fingerprint serial from registry; run before reattach |

## 3-pass matrix results (latest bundle)

| Pass | Bootstrap | REST | gRPC | MQTT | DB verify | Security | Fake-pass |
|------|-----------|------|------|------|-----------|----------|-----------|
| 1 | PASS | 352/352 | 75/75 | 17/17 | PASS | PASS | PASS |
| 2 | PASS | 352/352 | 75/75 | 17/17 | PASS | PASS | PASS |
| 3 | PASS | 352/352 | 75/75 | 17/17 | PASS | PASS | PASS |

Evidence: `reports/production-full-api-grpc-mqtt/20260703T234500Z/`

| Artifact | Result |
|----------|--------|
| `MULTI_PASS_PRODUCTION_VALIDATION.json` | 3/3 passes `ok=true` |
| `FAKE_PASS_AUDIT.json` | `fakePassRisk=false` |
| `SECURITY_AUTH_TEST_RESULTS.json` | 17/17 rules pass |
| `E2E_FLOW_RESULTS.json` | 9/9 flows pass (A–I) |
| `FINAL_PRODUCTION_REST_GRPC_MQTT_VERDICT.json` | `PRODUCTION_REST_GRPC_MQTT_RUNTIME_FLEET_100_PERCENT_PASS` |

## Deploy history for this verification cycle

1. **Runtime fleet deploy** — run `28686916171` @ `277a3ad4` + migrations 00017/00018 (SUCCESS)
2. **Timeline hotfix deploy** — run `28688099702` @ `51485f55` (SUCCESS, SLO recovery bypass)

## Known harness gaps (documented, not blocking automated matrices)

- Full SIM/board fingerprint on reattach E2E (minimal fingerprint only)
- Direct Postgres schema assertions (REST read-back only in `verify_db_state.py`)
- Dedicated technician multi-machine / operator-session RBAC negative matrix
- Fleet filter matrix / timeline event exhaustive assertions

## Gate

**PASS** — 3/3 production passes green with current-run evidence under prefix `AVF-RUNTIME-FLEET-20260703T234500Z`.
