# Local development (Docker)

## Start dependencies

From the repository root:

```bash
make dev-up
# or: docker compose -f deployments/docker/docker-compose.yml up -d
```

Postgres is listening on `localhost:5432` (default user/password in compose: `postgres` / `postgres`, database `avf_vending`). Optional profiles: `broker` (EMQX, MinIO), `observability`, etc. — see `deployments/docker/docker-compose.yml` and `deployments/docker/README.md`.

## Environment

Copy [`.env.local.example`](../../.env.local.example) to `.env` in the repository root (or load manually). A typical `DATABASE_URL`:

`postgres://postgres:postgres@localhost:5432/avf_vending?sslmode=disable`

Set `TEST_DATABASE_URL` the same for integration tests if needed.

## Redis-backed runtime features

Local development can run without Redis unless you explicitly enable Redis-backed features. When `REDIS_ENABLED=true` and `REDIS_ADDR` / `REDIS_URL` is configured, the API uses Redis for optional runtime helpers such as sale-catalog cache, refresh-session cache, login lockout counters, HTTP abuse limits, owner-token locks, access-token revocation checks, and machine gRPC hot-RPC rate limits.

Useful local toggles:

- `REDIS_CACHE_ENABLED=true` (or legacy `CACHE_ENABLED=true`) enables the Redis-backed sale-catalog/media-manifest projection cache.
- `REDIS_RATE_LIMIT_ENABLED=true` enables distributed HTTP rate-limit counters; local development falls back to memory when Redis is absent and `READINESS_STRICT=false`.
- `REDIS_SESSION_CACHE_ENABLED=true` caches refresh-session metadata by token hash only; PostgreSQL remains authoritative and revocation invalidates the cache.
- `REDIS_LOGIN_LOCKOUT_ENABLED=true` enables fast login-failure counters with the same lockout TTL as the PostgreSQL account lockout path.
- `REDIS_LOCKS_ENABLED=true` enables the Redis owner-token lock adapter for critical multi-worker sections.
- `REDIS_KEY_PREFIX=avf` scopes all new Redis keys for shared Redis deployments.
- `AUTH_ACCESS_JTI_REVOCATION_ENABLED=true` enables Redis JWT/JTI revocation checks; keep `AUTH_REVOCATION_REDIS_FAIL_OPEN=true` only for local troubleshooting.
- `RATE_LIMIT_GRPC_MACHINE_HOT_PER_MIN=900` controls machine gRPC hot-method limits. Without Redis, local gRPC falls back to an in-memory limiter; staging/production should configure managed Redis for distributed limits.

## Migrations

```bash
make dev-migrate
```

Runs `scripts/verify_database_environment.sh` with `APP_ENV=development`, then Goose `up` against the local DSN.

## Tests

```bash
make dev-test
# or: go test ./...
```

## Run services

- API: `make run-api` (uses `go run ./cmd/api`; load env first).
- Worker / reconciler / mqtt-ingest: `make run-worker`, and see `cmd/reconciler`, `cmd/mqtt-ingest` in the Makefile `build` list.

**Swagger (local):** With `HTTP_SWAGGER_UI_ENABLED=true`, UI is at `http://localhost:8080/swagger/index.html` (or your `HTTP_ADDR`) and OpenAPI JSON at `/swagger/doc.json`.

Quick Swagger/OpenAPI check:

Git Bash:

```bash
curl -fsS http://localhost:8080/swagger/doc.json >/tmp/avf-openapi.json
python tools/build_openapi.py
python tools/openapi_verify_release.py
```

PowerShell:

```powershell
Invoke-WebRequest -UseBasicParsing http://localhost:8080/swagger/doc.json -OutFile .\avf-openapi.json
python tools/build_openapi.py
python tools/openapi_verify_release.py
```

Local mutating smoke, after migrations and seed data:

```bash
bash scripts/smoke/local_field_smoke.sh --base-url http://localhost:8080 --evidence-json smoke-reports/local-field-smoke.json
```

## Reset local database only (destructive)

```bash
make dev-reset-db
```

Then re-run `make dev-migrate` when Postgres is ready.

## Validate config

```bash
go run ./cmd/cli -validate-config
```

## Health and metrics (local)

- **Liveness:** `GET /health/live` — process up; no dependency checks.
- **Readiness:** `GET /health/ready` — probes configured dependencies when `READINESS_STRICT=true` (see [production-readiness.md](./production-readiness.md)). Response body is only `ok` or `not ready` (no internal detail).
- **Metrics:** With `METRICS_ENABLED=true`, the API exposes Prometheus metrics on the main HTTP listener in non-production by default; in production, prefer scraping `HTTP_OPS_ADDR/metrics` unless you intentionally enable public metrics with `METRICS_EXPOSE_ON_PUBLIC_HTTP` and `METRICS_SCRAPE_TOKEN`.
- **Correlation:** HTTP uses chi `RequestID` and OpenTelemetry trace context; logs include `request_id` and `trace_id` when present (see `internal/observability`).

## Related

- [environment-strategy.md](./environment-strategy.md) — how local fits with staging and production.
- [production-readiness.md](./production-readiness.md) — staging/production gates and operability checklist.
- [docs/api/](../../api/) — product/API docs.
