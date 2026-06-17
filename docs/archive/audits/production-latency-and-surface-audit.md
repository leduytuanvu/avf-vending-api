# Production latency and API surface audit (baseline)

**Date:** 2026-05-23  
**Branch baseline:** `develop` (pre-change)  
**Scope:** Observability, DB/Redis/runtime tuning, REST/gRPC/MQTT canonical vs legacy — no route/proto/topic removal.

## Git / policy

- Default integration branch: `develop`; production release from `main` via existing GH workflows.
- `scripts/production/` did not exist before this initiative (deploy scripts under `scripts/deploy/`, `deployments/prod/scripts/`).

## Suspected latency sources

| Area | Finding | Evidence |
|------|---------|----------|
| DB pool | Production examples use **low** `DATABASE_MAX_CONNS` (3–10) vs fleet growth; risk of queueing under admin list load | `deployments/prod/.env.production.example`, `loadPostgresConfig` production default 10 |
| Slow queries | Wired via `DATABASE_SLOW_QUERY_LOG_MS` → pgx tracer (`postgres_slow_query`); often **0/disabled** in examples | `internal/platform/db/slow_query_tracer.go` |
| Redis | Sale-catalog cache optional; production examples may omit `REDIS_*` feature toggles | `internal/config/redis_runtime.go` |
| API CPU | App-node compose limits API to **0.75 CPU / 768M** | `deployments/prod/app-node/docker-compose.app-node.yml` |
| Legacy REST | Machine runtime HTTP disabled in prod by default; aliases still mounted for admin | `TransportBoundary.MachineRESTLegacyEnabled` |
| External media | Product image upload latency dominated by Cloudinary/object storage — not API-only benchmark | runbook note |
| MQTT ingest | Backpressure / global in-flight caps can delay dispatch under burst | `TELEMETRY_*` env in prod example |

## Current env defaults (code)

| Setting | Development/test | Staging | Production (when `DATABASE_URL` set, env unset) |
|---------|------------------|---------|--------------------------------------------------|
| `DATABASE_MAX_CONNS` | 3 | 5 | **10** (code); examples often 3–10 |
| `DATABASE_MIN_CONNS` | 0 | 0 | 0 |
| `DATABASE_MAX_CONN_IDLE_TIME` | — | 5m if URL set | 5m |
| `DATABASE_MAX_CONN_LIFETIME` | — | 30m if URL set | 30m |
| `DATABASE_SLOW_QUERY_LOG_MS` | 0 | 0 | 0 |
| Per-process max | Optional `API_DATABASE_MAX_CONNS`, etc. | Same | Examples: API=4, worker=3, mqtt-ingest=2 |

`DATABASE_HEALTH_CHECK_PERIOD` — **not implemented** (pgxpool manages health internally; document only).

## DB pool (runtime)

- Effective max: `PostgresConfig.MaxConnsForProcess(processName)` (`internal/config/config.go`).
- Pool created in `internal/platform/db/postgres.go`; metrics registered via `internal/observability/dbpoolmetrics`.
- Slow-query tracer attached when threshold > 0.

## CPU / memory limits (compose)

**Primary:** `deployments/prod/app-node/docker-compose.app-node.yml`

| Service | CPU limit | Memory limit |
|---------|-----------|--------------|
| api | 0.75 | 768M |
| worker | 0.50 | 512M |
| mqtt-ingest | 0.35 | 384M |
| reconciler | (see file) | (see file) |

**Legacy single-host:** `deployments/prod/docker-compose.prod.yml` — lower limits; rollback path only.

## Redis / cache status

- Connection: `REDIS_URL` / `REDIS_ADDR`, `REDIS_ENABLED`.
- Features (`redis_runtime.go`): `REDIS_CACHE_ENABLED`, `REDIS_RATE_LIMIT_ENABLED`, `REDIS_SESSION_CACHE_ENABLED`, `SALE_CATALOG_CACHE_TTL` (default 45s).
- Production: Redis required unless `PRODUCTION_ALLOW_MISSING_REDIS=true`.
- Catalog cache keys scoped by machine/catalog revision (see sale catalog service); **do not** cache auth decisions.

## Metrics / HTTP ops

- `METRICS_ENABLED` on public HTTP; production **discourages** public `/metrics` unless `METRICS_EXPOSE_ON_PUBLIC_HTTP` + token + `PRODUCTION_PUBLIC_METRICS_ENDPOINT_ALLOWED`.
- `HTTP_OPS_ADDR` mirrors health/version/metrics on private listener (`internal/httpserver/server.go`, `internal/observability/ops_http.go`).
- Existing: `http_request_duration_seconds`, gRPC histograms, `avf_db_pool_*` gauges, mqtt-ingest counters.

## REST duplicated / alias surfaces

| Domain | Canonical | Legacy / alias | Notes |
|--------|-----------|----------------|-------|
| Admin users | `/v1/admin/auth/users/*` (`accountId`) | `/v1/admin/users/*` (`userId`) | Same handlers (`auth_admin_http.go`) |
| User roles | `PUT /.../roles` | `POST`/`PATCH /.../roles` | Same handler |
| Media list/get | `/v1/admin/media/assets/*` | `/v1/admin/media`, `/v1/admin/media/{id}` | Aliases |
| Media upload | `/v1/admin/media/uploads/init`, assets POST | `/v1/admin/media/uploads`, complete on legacy paths | |
| Product image | `/v1/admin/products/{id}/media` | `/v1/admin/products/{id}/image` | Bind vs media link |
| Machine runtime | gRPC + MQTT enterprise | `/v1/setup`, `/v1/commerce`, `/v1/device`, sale-catalog, shadow, telemetry REST | Gated off in prod unless legacy flag |
| Command dispatch | Admin `POST /v1/admin/machines/{id}/commands` + MQTT enterprise `.../commands` | `POST /v1/machines/{id}/commands/dispatch`, legacy MQTT `commands/dispatch` | REST dispatch **not** behind legacy guard |

## gRPC surfaces

**Machine (`avf.machine.v1`):** Activation, Token, Auth (facade), Bootstrap, Catalog, Media, Inventory, Telemetry, Operator, Commerce, Sale, OfflineSync, Command — registered in `internal/grpcserver/machine_grpc_services.go`.

**Internal (`avf.internal.v1`):** Seven query services on loopback `INTERNAL_GRPC` listener.

**Duplicate protos not registered:** `proto/avf/v1/internal_queries.proto` (package `avf.v1`) — compatibility stubs only.

## MQTT surfaces

- Layout: `MQTT_TOPIC_LAYOUT=legacy|enterprise` (default legacy).
- Enterprise: `{prefix}/machines/{machineId}/...`; legacy: `{prefix}/{machineId}/...`.
- Commands: enterprise `.../commands`; legacy `.../commands/dispatch` or `commands/down`.
- Enterprise subscribes to generic `{prefix}/machines/+/events` — requires `event_type` in payload (`router.go`).
- Typed channels: `events/vend`, `events/cash`, `events/inventory` — supported on both layouts.

## Canonical vs legacy proposal

1. **Document** canonical paths in `docs/api/canonical-api-surface.md` and `docs/api/grpc-canonical-surface.md`.
2. **Deprecation headers** on known legacy REST aliases (`Deprecation: true`, `Link: rel=successor-version`).
3. **Postman:** group legacy flows under `Legacy and Compatibility`.
4. **No removal** until usage logs show zero and release notes announce sunset.
5. **MQTT:** prefer enterprise layout in production examples; keep legacy subscriptions.

## Uncertainties (do not guess)

- Live production `.env` values on VPS (not in git) may differ from examples — tune only after `measure-http-latency.sh` + `docker stats`.
- Supabase pooler session limit vs sum(process max × nodes) must be validated operator-side.
- Whether `MQTT_TOPIC_LAYOUT=enterprise` is already set in production broker ACLs — confirm on deploy host.

## Next steps (implementation branch)

See `perf/production-latency-observability-and-surface-cleanup`: scripts, env example updates, metrics, docs, Postman regen, local gates, deploy checklist.
