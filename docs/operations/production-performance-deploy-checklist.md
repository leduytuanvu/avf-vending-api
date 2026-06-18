# Production performance deploy checklist

## Pre-deploy

- [ ] `BASE_URL=https://api.ldtv.dev ./scripts/production/measure-http-latency.sh` (baseline)
- [ ] Confirm Supabase/pooler session limit vs `DATABASE_MAX_CONNS` × processes × nodes
- [ ] Redis region/latency acceptable; `REDIS_CACHE_ENABLED` / `SALE_CATALOG_CACHE_TTL` reviewed
- [ ] `docker stats` on app node (API/worker/mqtt-ingest)
- [ ] `git diff` contains **no** `.env.production`, secrets, or tokens
- [ ] `go test ./...`, OpenAPI verify, Postman validators, compose config validate — all pass

## Deploy

- [ ] Use existing GitHub production workflow (no gate bypass)
- [ ] Migrations only if `verify_migrations` passed in CI
- [ ] Record workflow run ID in final report

## Post-deploy

- [ ] `curl -fsS https://api.ldtv.dev/health/live`
- [ ] `curl -fsS https://api.ldtv.dev/health/ready`
- [ ] `curl -fsS https://api.ldtv.dev/version`
- [ ] `measure-http-latency.sh` with fresh admin token (products, machines, categories)
- [ ] Login + `/v1/auth/me` smoke
- [ ] gRPC safe unary (bootstrap/catalog) if test machine token available
- [ ] MQTT broker connect + safe ACK topic (test machine only)
- [ ] Postman read-only / happy-case subset (no destructive suite unless gated)

## Rollback

- [ ] Redeploy last-known-good image digest via existing rollback workflow
- [ ] Roll back immediately if health/ready fail, 5xx spike, or severe p95 regression

Record results in `docs/archive/audits/production-latency-and-surface-final-report.md` (archived template).
