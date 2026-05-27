# Production resource sizing

Tune only after observing `docker stats`, pool metrics (`avf_db_pool_*`), and `measure-http-latency.sh` baselines.

## Fleet scale guidance (starting points)

| Machines | API `DATABASE_MAX_CONNS` / `API_DATABASE_MAX_CONNS` | Worker / mqtt-ingest max | API CPU (compose limit) |
|----------|-----------------------------------------------------|--------------------------|-------------------------|
| ~100 | 40 / 25 | 8 / 8 | 1.50 |
| ~500 | 40–60 / 25–35 | 8–12 | 1.50–2.00 |
| ~1000 | 60+ / 35+ (verify pooler limit) | 12+ | 2.00 |

Global `DATABASE_MAX_CONNS` must be ≥ sum of per-process effective max × app nodes.

## Compose defaults (app-node)

- **api:** 1.50 CPU, 768M RAM (`deployments/prod/app-node/docker-compose.app-node.yml`)
- **worker:** 0.50 CPU, 512M
- **mqtt-ingest:** 0.35 CPU, 384M

## When to change

- API CPU sustained >70% with elevated `http_request_duration_seconds` → raise API CPU or horizontal scale.
- `avf_db_pool_acquired_conns` near max with slow admin APIs → raise pool caps or optimize queries.
- mqtt-ingest dispatch failures / lag → raise ingest CPU or `TELEMETRY_GLOBAL_MAX_INFLIGHT` only with evidence.

See [production-latency-runbook](./production-latency-runbook.md).
