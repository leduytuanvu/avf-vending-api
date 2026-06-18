# Production performance audit

**Date:** 2026-05-27  
**Scope:** Safe measurement and configuration review (no production writes in this audit).

## Measurements (pre-change baseline)

| Endpoint | http_code | total (s) | Notes |
|----------|-----------|-----------|-------|
| `GET /health/live` | 200 | ~0.36 | From operator network to api.ldtv.dev |
| `GET /health/ready` | 200 | ~0.31 | Same |

Admin list APIs require `ACCESS_TOKEN` — not run without operator credential.

## Configuration review (code + examples)

| Item | Finding |
|------|---------|
| `DATABASE_MAX_CONNS` | Examples on develop may still show low values; production code default when unset in `AppEnvProduction` is **40** on perf branch |
| Slow query | `DATABASE_SLOW_QUERY_LOG_MS` wired to pgx tracer |
| Redis catalog | `SALE_CATALOG_CACHE_TTL` default 45s when cache enabled |
| Metrics | `http_request_duration_seconds`, `avf_db_pool_*` on ops listener |
| Compose API CPU | app-node **0.75** on develop; perf branch proposes **1.50** |

## Safe improvements (this branch)

1. Postman/OpenAPI parity fix: `REST_EXPECTED=329` (no coverage reduction).
2. `scripts/production/smoke-market-readiness.sh` for read-only prod smoke.
3. Inventory JSON for ongoing audits.

## Not changed (requires perf branch merge or deploy)

- Pool defaults on live VPS
- API CPU compose bump
- Deprecation middleware

## Recommendations before high traffic

1. Run `measure-http-latency.sh` with admin token before/after deploy.
2. Watch `avf_db_pool_acquired_conns` vs max during peak admin usage.
3. Enable slow query log at 200ms in production env (not in git secrets).
4. Do not benchmark image upload as API latency (Cloudinary dominates).
