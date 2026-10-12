# Latency VPS / pool ops checklist

Run after code deploys that target REST/gRPC/MQTT latency. See [production-latency-runbook](./production-latency-runbook.md).

## Before changing infra

- [ ] Baseline: `BASE_URL=https://api.ldtv.dev ./scripts/production/measure-http-latency.sh`
- [ ] `docker stats` on app-node (api, worker, mqtt-ingest, caddy)
- [ ] Ops metrics: `http_request_duration_seconds`, `grpc_request_duration_seconds`, `avf_db_pool_acquired_conns`
- [ ] Confirm `DATABASE_URL` uses Supabase **transaction pooler port 6543** (not session `:5432` pool_size=15)
- [ ] Sum of `API_DATABASE_MAX_CONNS` + worker + mqtt-ingest + reconciler < ~60–70% of pooler limit

## Safe tuning knobs (env on app-node)

| Knob | When to raise |
|------|----------------|
| API CPU limit (compose) | API container CPU sustained >70% with elevated p95 |
| `SALE_CATALOG_CACHE_TTL` | Keep 45s when Redis enabled; do not disable cache for latency |
| `API_DATABASE_MAX_CONNS` | Pool acquire wait high **and** pooler headroom verified |
| EMQX CPU/memory | MQTT publish/ingest CPU >70% or PUBACK p95 high |

## Do not

- Enable app-node B without pooler headroom (doubles connection budget)
- Raise HTTP/gRPC timeouts expecting faster responses (timeouts are ceilings only)
- Disable `critical_no_drop` MQTT ingest for speed

## After changes

- [ ] Re-run `measure-http-latency.sh` and compare TTFB p95
- [ ] Smoke gRPC catalog + heartbeat on a test machine
- [ ] Dispatch one safe remote command; confirm `queued` → `published` within seconds

## Emergency deploy fallback (GHCR / hosted runners blocked)

Use GitHub Actions **Production Self-Hosted Build Deploy** (`production-self-hosted-build-deploy.yml`) on app-node A:

- `confirm_deploy` = **`DEPLOY_PRODUCTION`**
- `run_migration` = **false** when this release has no new goose migrations (latency/harden-only)
- Then run the **After changes** checklist above

Canonical path remains **Deploy Production** (`deploy-prod.yml`) with Build + Security Release digests when the enterprise chain is available.
