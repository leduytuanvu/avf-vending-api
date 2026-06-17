# Production latency runbook

Measure before and after deploys. Do not use multipart image upload as a general API latency benchmark (Cloudinary/object storage dominates).

## Baseline (pre-deploy)

```bash
BASE_URL=https://api.ldtv.dev ./scripts/production/measure-http-latency.sh
BASE_URL=https://api.ldtv.dev ACCESS_TOKEN=<admin JWT> ./scripts/production/measure-http-latency.sh
```

On the app node (optional):

```bash
APP_NODE_DIR=/path/to/deployments/prod/app-node PUBLIC_URL=https://api.ldtv.dev \
  ./scripts/production/measure-production-stack.sh
```

Artifacts: `.production-latency-runs/<timestamp>/` (gitignored).

## Interpreting curl timings

| Field | Meaning |
|-------|---------|
| `dns` | DNS lookup |
| `connect` | TCP connect to origin |
| `tls` | TLS handshake (HTTPS) |
| `ttfb` | Time to first byte (server + DB/Redis) |
| `total` | End-to-end |

**Health fast, admin lists slow** → Postgres pool saturation, slow queries, missing indexes, or Redis cache off. Check `DATABASE_*` pool env, `DATABASE_SLOW_QUERY_LOG_MS`, and `avf_db_pool_*` metrics on the ops listener.

**All endpoints slow** → VPS CPU/RAM, Caddy, network, or TLS termination. Check `docker stats`, API CPU limit in `deployments/prod/app-node/docker-compose.app-node.yml`.

**High `dns`/`connect` only on public URL** → DNS/CDN/edge; compare localhost timings from `measure-production-stack.sh`.

## Prometheus metrics (ops listener)

Scrape `HTTP_OPS_ADDR` (or worker/mqtt-ingest `*_METRICS_LISTEN`) when `METRICS_ENABLED=true`. Public `/metrics` stays disabled in production unless explicitly allowed.

| Metric | Use |
|--------|-----|
| `http_request_duration_seconds` | p95 by `method`, `route` (Chi template), `status` |
| `http_requests_in_flight` | Concurrency pressure |
| `avf_db_pool_acquired_conns` / `idle` / `total` / `max` | Pool saturation |
| `avf_db_pool_acquire_count_total` | Acquire churn |
| `avf_db_pool_acquire_duration_seconds_total` | Wait time aggregate |
| `grpc_request_duration_seconds` | Machine RPC latency |
| `avf_mqtt_ingest_dispatch_total` | Ingest errors/backpressure |

Do not add high-cardinality labels (raw path, accountId, machineId, tokens).

## Redis / catalog cache

- `REDIS_CACHE_ENABLED=true`, `SALE_CATALOG_CACHE_TTL=45s` in production examples when Redis is configured.
- Catalog cache is machine-scoped with revision/epoch keys; short TTL if invalidation path is uncertain.
- Never cache auth/session revocation decisions in catalog TTL paths.

## Related

- [Production resource sizing](./production-resource-sizing.md)
- [Production performance deploy checklist](./production-performance-deploy-checklist.md)
- [Baseline audit](../archive/audits/production-latency-and-surface-audit.md) (archived)
