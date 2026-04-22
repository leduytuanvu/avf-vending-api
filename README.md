# avf-vending-api

Go **1.25+** backend for the AVF vending platform: HTTP API (`cmd/api`), optional internal gRPC query services, background processes, and shared wiring (PostgreSQL, Redis, OpenTelemetry).

**What runs today** (with env and migrations): HTTP `/v1`, Postgres-backed commerce/device/fleet flows, `cmd/worker` reliability ticks with **optional** NATS JetStream outbox publish when `NATS_URL` is set, **optional** ClickHouse analytics mirror when `ANALYTICS_*` is enabled (`ops/ANALYTICS_CLICKHOUSE.md`), `cmd/mqtt-ingest` MQTT→Postgres ingest, `cmd/reconciler` commerce reconciliation (list-only by default; **optional** close-the-loop actions when `RECONCILER_ACTIONS_ENABLED=true` with probe URL + NATS—see `internal/config/reconciler.go`), optional **internal gRPC query services** for machine, telemetry, and commerce reads behind Bearer JWT auth, and optional **Temporal-backed workflow follow-up** via `cmd/temporal-worker`. **Artifacts** use S3 when `API_ARTIFACTS_ENABLED=true`. **Not in this repo yet**: device/runtime traffic over gRPC and broad command/workflow mutation RPCs beyond the implemented compensation/review flows.

## Current architecture

The repository is currently implemented as a **Go modular monolith** with multiple binaries under `cmd/*`, shared `internal/app/*` business logic, and Postgres-backed persistence under `internal/modules/postgres`.

- **Implemented now:** `cmd/api` for HTTP control/admin/setup flows, `cmd/mqtt-ingest` for MQTT-first device ingest, `cmd/worker` for reliability/outbox/telemetry background work, and `cmd/reconciler` for commerce reconciliation.
- **Partially implemented:** MQTT runtime flows also depend on NATS JetStream in production for buffering and async processing; gRPC is internal-only and currently limited to query/read services rather than a broad service mesh API; Temporal is implemented for selected long-running compensation/review flows behind feature flags; ClickHouse and object storage are wired for specific optional paths, not as universal platform dependencies.
- **Not implemented yet:** broad internal gRPC mutation/workflow APIs and a fully closed-loop analytics/event architecture across every device/runtime flow.

The as-built freeze for this phase lives in [`docs/architecture/current-architecture.md`](docs/architecture/current-architecture.md). It separates **implemented**, **partial**, and **not implemented yet** items and calls out corrected docs drift.

The repo still follows a **strangler** posture for traffic cutover from any legacy system—see [docs/architecture/migration-strategy.md](docs/architecture/migration-strategy.md).

## Layout

| Path | Role |
|------|------|
| [`cmd/api`](cmd/api) | Public HTTP process |
| [`cmd/worker`](cmd/worker) | Reliability ticks (payments/commands/outbox); **NATS JetStream** outbox + **DLQ** (`AVF_INTERNAL_DLQ`) when `NATS_URL` is set; optional **ClickHouse** mirror (`ANALYTICS_*`); optional **`METRICS_ENABLED`** + `WORKER_METRICS_LISTEN` for Prometheus `/metrics` (see [`ops/METRICS.md`](ops/METRICS.md)) |
| [`cmd/mqtt-ingest`](cmd/mqtt-ingest) | MQTT subscriber → Postgres device ingest (broker via env; see `internal/platform/mqtt`) |
| [`cmd/reconciler`](cmd/reconciler) | Commerce reconciliation ticks; optional PSP probe + refund routing when `RECONCILER_ACTIONS_ENABLED=true` (validated at startup) |
| [`cmd/temporal-worker`](cmd/temporal-worker) | Temporal workflow worker for payment-timeout, vend-failure, refund, and manual-review follow-up |
| [`cmd/cli`](cmd/cli) | Operational CLI (`-validate-config`, `-version`) |
| [`internal/config`](internal/config) | Environment-backed configuration with validation |
| [`internal/httpserver`](internal/httpserver) | Chi HTTP server: `/health/*`, optional `/metrics` |
| [`internal/bootstrap`](internal/bootstrap) | Process wiring (runtime clients, graceful shutdown) |
| [`internal/modules/postgres`](internal/modules/postgres) | Postgres + sqlc-backed OLTP (orders/payments/commands; not Temporal) |
| [`proto`](proto) | buf config + internal gRPC protobuf contracts (`avf/v1`) |
| [`migrations`](migrations) | goose SQL migrations |
| [`deployments/docker`](deployments/docker) | Local dependency stack (Compose) |
| [`deployments/prod/app-node`](deployments/prod/app-node) | New 2-VPS stateless production app-node stack |
| [`deployments/prod/data-node`](deployments/prod/data-node) | New 2-VPS fallback broker/data-node stack |

## Prerequisites

- **Go** 1.25+ on `PATH`
- **Docker** (optional, for local dependencies)
- **sqlc** (optional, only if you regenerate [`internal/gen/db`](internal/gen/db))
- **bash**, **git**, and **ripgrep (`rg`)** on `PATH` if you run `make check-placeholders` / `make ci-gates` (Git Bash or WSL on Windows is fine)

## CI and local gates

This repository's GitHub Actions CI foundation lives under `.github/workflows/`:

- `ci.yml` runs on pull requests and pushes to `main`, runs the same gate set as `make ci-gates`, and validates `deployments/docker/docker-compose.yml`
- `security.yml` runs dependency review on pull requests and `govulncheck` on the repo
- `build-push.yml` builds and pushes the production app and goose images

Local equivalents:

```powershell
make ci          # gates + unit tests (-short)
make ci-full     # gates + all tests (export TEST_DATABASE_URL for postgres integration tests)
make ci-gates    # formatting, vet, sqlc drift, swagger drift, placeholder/wiring/migration scripts (no tests)
```

Install **sqlc** for `make sqlc-check` / `make ci-gates`: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0` (pin should match the workflow).

**Swagger / OpenAPI** (embedded in the API process): annotations live in [`cmd/api/main.go`](cmd/api/main.go) (API metadata) and [`internal/httpserver/swagger_operations.go`](internal/httpserver/swagger_operations.go) (per-route docs). Regenerate committed artifacts with `make swagger` (uses Python 3 via `PY=python3` by default; on Windows use `make swagger PY=python` if needed). CI and `make swagger-check` fail when [`docs/swagger`](docs/swagger) is stale. With Swagger enabled (default in non-production; see `HTTP_SWAGGER_UI_ENABLED` in [`.env.example`](.env.example)), the server serves **Swagger UI** at `/swagger/index.html` and raw **OpenAPI 3.0** JSON at `/swagger/doc.json` (multi-environment **Servers** + shared error model).

**Reporting** (`platform_admin` or `org_admin`, Bearer JWT): read-only JSON under **`/v1/reports/*`** (sales, payments, fleet health, inventory exceptions). Every report requires **`from`** and **`to`** (RFC3339) with a maximum span of **366 days**; **`organization_id`** is required when the caller is `platform_admin`, same as other tenant-picked admin routes. There is no CSV export or async job in this API surface—clients consume aggregates directly.

## Local dependencies (Docker)

From the repository root:

```powershell
docker compose -f deployments/docker/docker-compose.yml up -d
```

PostgreSQL is created with database **`avf_vending`** (see [`deployments/docker/postgres-init/01-init.sql`](deployments/docker/postgres-init/01-init.sql)). Use a matching `DATABASE_URL` in `.env` (see [`.env.example`](.env.example)).

More detail: [`deployments/docker/README.md`](deployments/docker/README.md).

## Configuration

Copy [`.env.example`](.env.example) to `.env` for local development. Validate:

```powershell
go run ./cmd/cli -validate-config
```

## Database migrations

With `DATABASE_URL` set (PowerShell example):

```powershell
$env:DATABASE_URL = "postgres://localhost:5432/avf_vending?sslmode=disable"
make migrate-up
```

Use your local database credentials in `DATABASE_URL` as needed. On Unix shells, `make migrate-up` passes `${DATABASE_URL}` to goose; ensure the variable is exported first.

## Build and test

```powershell
make tidy
make build
make test
```

Integration-style tests under [`internal/modules/postgres`](internal/modules/postgres) require **`TEST_DATABASE_URL`** and skip when unset or when `-short` is passed.

## Documentation

- [Target architecture](docs/architecture/target-architecture.md)
- [Current architecture (as built)](docs/architecture/current-architecture.md)
- [Strangler / migration strategy](docs/architecture/migration-strategy.md)
- [Documentation index](docs/README.md)
- [Internal gRPC contract](docs/api/internal-grpc.md)
- [2-VPS production runbook](docs/runbooks/production-2-vps.md)
- [2-VPS cutover and rollback](docs/runbooks/production-cutover-rollback.md)
- [2-VPS backup, restore, and DR](docs/runbooks/production-backup-restore-dr.md)
- [2-VPS day-2 incidents](docs/runbooks/production-day-2-incidents.md)
- [Operations runbook](ops/RUNBOOK.md) — incidents, dashboards/alert ideas, SQL checks
- [Metrics / signals](ops/METRICS.md)

## Makefile

See `Makefile` for common targets such as `make ci-gates`, `make ci`, `make build`, and the `prod-*` helpers.

## Lean production (telemetry hardening checks)

Before merging or promoting a release that touches production telemetry wiring, from the repository root (with a filled `deployments/prod/.env.production`, usually copied from `.env.production.example`):

```powershell
docker compose --env-file deployments/prod/.env.production -f deployments/prod/docker-compose.prod.yml config
bash deployments/prod/scripts/validate_prod_telemetry.sh
go build ./...
```

On the VPS after `docker compose up`, run `bash deployments/prod/scripts/healthcheck_prod.sh` (or `make prod-smoke` / `make prod-smoke-full` from `deployments/prod`). Operator runbooks: [docs/runbooks/telemetry-production-rollout.md](docs/runbooks/telemetry-production-rollout.md), [docs/runbooks/telemetry-jetstream-resilience.md](docs/runbooks/telemetry-jetstream-resilience.md).

For the new 2-VPS enterprise production layout, keep the existing single-VPS path above for rollback and use the new deployment assets under `deployments/prod/app-node`, `deployments/prod/data-node`, and `deployments/prod/shared`. Validate them with:

```powershell
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml config
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml --profile temporal config
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml --profile migration config
docker compose --env-file deployments/prod/data-node/.env.data-node.example -f deployments/prod/data-node/docker-compose.data-node.yml config
```

For the current production snapshot, keep runtime values environment-driven only: `APP_BASE_URL=https://api.ldtv.dev`, `MQTT_BROKER_URL` pointing at `mqtt.ldtv.dev`, `DATABASE_URL` supplied by Supabase, and `OBJECT_STORAGE_BUCKET=avf-vending-prod-assets`. No real secrets belong in the repo, and Grafana is intentionally out of scope for this pass.
