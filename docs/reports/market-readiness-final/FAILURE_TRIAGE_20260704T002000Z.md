# Market Readiness — Failure Triage

**UTC:** 20260704T002000Z  
**Status:** Harness implemented; full production matrix **not executed** in this session.

## Blocker

| Item | Expected | Actual | Root cause |
|------|----------|--------|------------|
| Production suite run | 3× REST/gRPC/MQTT + gap matrices green | Not run | `PROD_TEST_ADMIN_EMAIL` / `PROD_TEST_ADMIN_PASSWORD` not in session env; `.env` has only local `DATABASE_URL` |
| Direct DB verification | `verify_db_direct.py` 3× pass | Not run | `PROD_DATABASE_URL` not in session env |

## Harness fixes applied (pre-run)

| Change | Type | Files |
|--------|------|-------|
| Market readiness package | Harness | `tools/market_readiness/*` |
| Strict REST / security | Harness | `_common.py`, `run_rest_full_production.py`, `security_auth_tests.py`, `fake_pass_audit.py` |
| Chi-only fleet routes | Harness | `CHI_ONLY_ROUTES` expansion |

## Product code changes

**None** — no production failures observed; deploy not required for harness-only work.

## Next operator steps

1. Export session env (never commit):
   - `PROD_TEST_ADMIN_EMAIL` / `PROD_TEST_ADMIN_PASSWORD`
   - `PROD_DATABASE_URL`
   - Optional `PROD_TEST_TECHNICIAN_EMAIL` / `PROD_TEST_TECHNICIAN_PASSWORD`
   - `GRPC_HOST=machine-api.ldtv.dev:443`, `MQTT_BROKER=mqtt.ldtv.dev:8883`
2. `pip install psycopg[binary]`
3. Run:
   ```bash
   python tools/market_readiness/run_market_readiness_suite.py \
     --prefix AVF-MARKET-READY-$(date -u +%Y%m%dT%H%M%SZ) \
     --base-url https://api.ldtv.dev
   ```
4. On failure: triage here, fix harness or product, merge `develop`/`main`, redeploy if product fix, **restart suite from pass 1**.

## develop/main parity (pre-merge)

PR #411 timeline hotfix on `main` only — merge to `develop` before parity gate can pass.
