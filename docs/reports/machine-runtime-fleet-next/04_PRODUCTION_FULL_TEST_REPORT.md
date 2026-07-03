# Production Full Test Report — Machine Runtime Fleet

**Date:** 2026-07-04  
**Prefix (required):** `AVF-RUNTIME-FLEET-{UTC}`  
**Verdict:** **BLOCKED_BY_PRODUCTION_DEPLOY_AND_TEST**

## Harness updates (local, committed)

- `_common.runtime_fleet_prefix()` → `AVF-RUNTIME-FLEET-{UTC}_{suffix}`
- `run_production_full_suite.py --prefix` sets `PRODUCTION_TEST_PREFIX` / `PRODUCTION_SUITE=runtime_fleet`
- `run_grpc_full_production.py` inline `MachineRuntimeSessionService` RPC matrix (no deleted GRPC_INVENTORY dependency)
- `verify_db_state.py` adds ops-overview, device-attachments, fleet ops-overview checks

## 3× production suite

**NOT EXECUTED** — blocked by:

1. Feature not deployed to production (00017+00018)
2. Admin production credentials not available in this agent session for live bootstrap

## Command (when unblocked)

```bash
python tools/production_full_test/run_production_full_suite.py --passes 3 --prefix AVF-RUNTIME-FLEET-<UTC>
```

## Expected matrices (when run)

| Surface | Coverage target |
|---------|-----------------|
| REST | app-sessions, device-attachments, ops-overview, activate/deactivate, reattach |
| gRPC | Start/Heartbeat/Get/End runtime session |
| MQTT | heartbeat touch + negatives |
| DB | one current attachment/session, snapshot fields |
| Security | RBAC on admin runtime routes |

## Fake-pass audit

No production run artifacts — **no pass claim**.
