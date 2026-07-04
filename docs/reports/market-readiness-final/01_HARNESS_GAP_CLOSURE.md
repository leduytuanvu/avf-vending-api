# Market Readiness — Harness Gap Closure

**UTC:** 20260704T001500Z

## Before / After Gap Table

| Gap (runtime-fleet) | Before | After (market readiness harness) |
|---------------------|--------|----------------------------------|
| Full SIM/board fingerprint reattach | Minimal serial-only E2E | `run_reattach_fingerprint_matrix.py` — camelCase + snake_case, REST read-back |
| Direct Postgres assertions | REST-only `verify_db_state.py` | `verify_db_direct.py` — attachments, snapshot, machine code via `PROD_DATABASE_URL` |
| Technician multi-machine RBAC | Skipped / Go contract only | `bootstrap_market_rbac.py` + `run_technician_rbac_matrix.py` — 3 machines, negatives |
| Fleet filters + timeline | Single ops-overview smoke | `run_fleet_timeline_matrix.py` — filter matrix + unified timeline metadata |
| Security skip-as-pass | Missing role tokens → PASS | `MARKET_READINESS_STRICT=1` → SKIPPED = FAIL |
| REST Chi-only fleet routes | Partial | Extended `CHI_ONLY_ROUTES` + strict `expected_ok_status()` |
| Orchestrator / verdict | `run_production_full_suite.py` | `run_market_readiness_suite.py` + `write_market_readiness_verdict.py` (14 gates) |
| E2E / chaos | Flows A–I only | `run_market_e2e_flows.py` (1–9), `run_chaos_edge_tests.py` (14 cases) |
| Fake-pass audit | REST/gRPC/MQTT bundle | Extended for market bundle artifacts + security skip check |

## New modules

| Module | Purpose |
|--------|---------|
| `tools/market_readiness/_common.py` | Prefix, bundle dir, fingerprint builder, backup gate |
| `tools/market_readiness/run_reattach_fingerprint_matrix.py` | 3× fingerprint reattach |
| `tools/market_readiness/verify_db_direct.py` | 3× SQL verification |
| `tools/market_readiness/bootstrap_market_rbac.py` | Technician + machines A/B/C |
| `tools/market_readiness/run_technician_rbac_matrix.py` | 3× RBAC negatives |
| `tools/market_readiness/run_fleet_timeline_matrix.py` | 3× fleet + timeline |
| `tools/market_readiness/run_market_e2e_flows.py` | 3× flows 1–9 |
| `tools/market_readiness/run_chaos_edge_tests.py` | 3× edge cases |
| `tools/market_readiness/run_market_readiness_suite.py` | Master runner |
| `tools/market_readiness/write_market_readiness_verdict.py` | 14-gate verdict |

## Timeline event types asserted (actual API strings)

From `db/queries/machine_ops_timeline.sql`:

- `device.attachment.attached`
- `device.attachment.replaced` / `device.attachment.revoked` / `device.attachment.compromised`
- `runtime.app_session.started` / `runtime.app_session.ended`
- Operator / lifecycle events as returned by `GET .../timeline/unified`

## Run command

```bash
python tools/market_readiness/run_market_readiness_suite.py \
  --prefix AVF-MARKET-READY-{UTC} \
  --base-url https://api.ldtv.dev \
  --rest-passes 3 --grpc-passes 3 --mqtt-passes 3 \
  --e2e-passes 3 --chaos-passes 3
```

Session env: `PROD_TEST_ADMIN_EMAIL`, `PROD_TEST_ADMIN_PASSWORD`, `PROD_DATABASE_URL`, optional technician env overrides.
