# avf-vending-api

Go **1.25+** backend for the AVF vending platform: HTTP Admin/control plane (`cmd/api`), **public machine gRPC** (`avf.machine.v1`, Machine JWT, when **`MACHINE_GRPC_ENABLED=true`**), optional **internal** loopback gRPC reads (`avf.internal.v1`, service JWT), background processes, and shared wiring (PostgreSQL, Redis, OpenTelemetry).

**What runs today** (with env and migrations): HTTP `/v1` for admins/operators/webhooks; **native kiosk/runtime** on **`avf.machine.v1`** when machine gRPC is enabled (production requires **`MACHINE_GRPC_ENABLED=true`** explicitly); Postgres-backed commerce/device/fleet flows; legacy OpenAPI machine REST routes remain **`deprecated`** for migration windows only (`MACHINE_REST_LEGACY_ENABLED`). `cmd/worker` runs reliability ticks with **optional** NATS JetStream outbox publish when `NATS_URL` is set, **optional** ClickHouse analytics mirror when `ANALYTICS_*` is enabled (`ops/ANALYTICS_CLICKHOUSE.md`), `cmd/mqtt-ingest` MQTT→Postgres ingest, `cmd/reconciler` commerce reconciliation (list-only by default; **optional** close-the-loop actions when `RECONCILER_ACTIONS_ENABLED=true` with probe URL + NATS—see `internal/config/reconciler.go`). Optional **internal gRPC query services** (`INTERNAL_GRPC_ENABLED`) are loopback read/query only—not the vending app API. Optional **Temporal-backed workflow follow-up** via `cmd/temporal-worker`. **Artifacts** use S3 when `API_ARTIFACTS_ENABLED=true`. **Future scope:** broader internal gRPC mutation APIs and fully unified analytics/event planes—not missing native machine gRPC.

## Current architecture

The repository is currently implemented as a **Go modular monolith** with multiple binaries under `cmd/*`, shared `internal/app/*` business logic, and Postgres-backed persistence under `internal/modules/postgres`.

- **Implemented now:** `cmd/api` for HTTP control/admin/setup flows and optional **`avf.machine.v1`** + **`avf.internal.v1`** gRPC listeners; `cmd/mqtt-ingest` for MQTT-first device ingest; `cmd/worker` for reliability/outbox/telemetry background work; `cmd/reconciler` for commerce reconciliation.
- **Partially implemented:** MQTT runtime flows also depend on NATS JetStream in production for buffering and async processing; internal gRPC remains read/query-only on loopback; Temporal is implemented for selected long-running compensation/review flows behind feature flags; ClickHouse and object storage are wired for specific optional paths, not as universal platform dependencies.
- **Not implemented yet:** broad internal gRPC mutation/workflow APIs and a fully closed-loop analytics/event architecture across every bounded context.

The as-built freeze for this phase lives in [`docs/architecture/current-architecture.md`](docs/architecture/current-architecture.md). It separates **implemented**, **partial**, and **not implemented yet** items and calls out corrected docs drift.

The repo still follows a **strangler** posture for traffic cutover from any legacy system—see [docs/architecture/migration-strategy.md](docs/architecture/migration-strategy.md).

## Layout

| Path | Role |
|------|------|
| [`cmd/api`](cmd/api) | Public HTTP process |
| [`cmd/worker`](cmd/worker) | Reliability ticks (payments/commands/outbox); **NATS JetStream** outbox + **DLQ** (`AVF_INTERNAL_DLQ`) when `NATS_URL` is set; optional **ClickHouse** mirror (`ANALYTICS_*`); optional **`METRICS_ENABLED`** + `WORKER_METRICS_LISTEN` for Prometheus `/metrics` (see [`ops/METRICS.md`](ops/METRICS.md)) |
| [`cmd/mqtt-ingest`](cmd/mqtt-ingest) | MQTT subscriber → Postgres device ingest (broker via env; see `internal/platform/mqtt`) |
| [`cmd/reconciler`](cmd/reconciler) | Commerce reconciliation ticks; optional PSP probe + refund routing when `RECONCILER_ACTIONS_ENABLED=true` (validated at startup) |
| [`cmd/temporal-worker`](cmd/temporal-worker) | Temporal workflow worker for payment-to-vend, refund, command ACK, payment-timeout, vend-failure, and manual-review follow-up |
| [`cmd/cli`](cmd/cli) | Operational CLI (`-validate-config`, `-version`) |
| [`internal/config`](internal/config) | Environment-backed configuration with validation |
| [`internal/httpserver`](internal/httpserver) | Chi HTTP server: `/health/*`, optional `/metrics` |
| [`internal/bootstrap`](internal/bootstrap) | Process wiring (runtime clients, graceful shutdown) |
| [`internal/modules/postgres`](internal/modules/postgres) | Postgres + sqlc-backed OLTP (orders/payments/commands; not Temporal) |
| [`proto`](proto) | buf config — **`avf.machine.v1`** (public machine runtime), **`avf.internal.v1`** (loopback reads), legacy packages as applicable |
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

**End-to-end pipeline and deploy chain (branches, staging, production, triage):** [docs/runbooks/cicd-release.md](docs/runbooks/cicd-release.md). Enterprise audit: [CI_CD_FINAL_AUDIT.md](CI_CD_FINAL_AUDIT.md).

This repository's GitHub Actions live under `.github/workflows/` (high level):

- **`ci.yml` (`CI`)** — pull requests and pushes to `develop` and `main`, workflow contract and migration checks, and the same style of gates as `make ci-gates` (validates `deployments/docker/docker-compose.yml` among other steps). It also runs a **GitHub repository governance** read-only check (`scripts/ci/verify_github_governance.sh`); **Settings** for branches and the `production` environment must still be configured in the UI — see [docs/operations/github-governance.md](docs/operations/github-governance.md).
- **`security.yml` (`Security`)** — repository scans (ex.: govulncheck, **Secret**/**Config** jobs); **not** the same as **Security Release** (verdict/artifact) or a deploy trigger.
- **`build-push.yml` (`Build and Push Images`)** — runs after a successful **CI** `workflow_run` for eligible `develop`/`main` pushes; builds and pushes digest-pinned app and goose images and promotion artifacts. **Not** the default path for every open PR in isolation.

Local equivalents:

```powershell
make ci                 # fmt (gofmt check) + vet + go test + build — core go-ci make steps
make ci-gates           # fmt-check, vet, placeholders, wiring, migrations, api-contract-check (includes sqlc + swagger + postman + proto + machine gRPC docs; no unit tests)
make test-short         # go test -short (as in the Go CI Gates job)
make verify-workflows   # actionlint + workflow contract scripts (Workflow and Script Quality; needs actionlint on PATH)
make ci-full            # ci-gates + all tests (export TEST_DATABASE_URL for postgres integration tests)
```

Install **sqlc** for `make sqlc-check` / `make ci-gates`: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0` (pin should match the workflow).

**Swagger / OpenAPI** (embedded in the API process): annotations live in [`cmd/api/main.go`](cmd/api/main.go) (API metadata) and [`internal/httpserver/swagger_operations.go`](internal/httpserver/swagger_operations.go) (per-route docs). Regenerate committed artifacts with `make swagger` (uses Python 3 via `PY=python3` by default; on Windows use `make swagger PY=python` if needed). CI and `make swagger-check` fail when [`docs/swagger`](docs/swagger) is stale. With Swagger enabled (default in non-production; see `HTTP_SWAGGER_UI_ENABLED` in [`.env.example`](.env.example)), the server serves **Swagger UI** at `/swagger/index.html` and raw **OpenAPI 3.0** JSON at `/swagger/doc.json` (multi-environment **Servers** + shared error model).

**Reporting** (Bearer JWT + `reports.read`): read-only JSON under **`/v1/reports/*`** and organization-scoped Admin Web reports under **`/v1/admin/organizations/{organizationId}/reports/*`**. Every report requires **`from`** and **`to`** (RFC3339) with a maximum span of **366 days**; platform admins must pick the tenant explicitly. Supported CSV exports are synchronous (`format=csv` on selected Admin report routes, plus legacy `/v1/admin/reports/*/export.csv`) and audited.

## Local dependencies (Docker)

From the repository root:

```powershell
docker compose -f deployments/docker/docker-compose.yml up -d
```

PostgreSQL is created with database **`avf_vending`** (see [`deployments/docker/postgres-init/01-init.sql`](deployments/docker/postgres-init/01-init.sql)). Use a matching `DATABASE_URL` in `.env` (see [`.env.example`](.env.example)).

More detail: [`deployments/docker/README.md`](deployments/docker/README.md). Quick lifecycle targets: `make dev-up`, `make dev-down`, `make dev-migrate`, `make dev-test`, `make dev-reset-db` (see [docs/runbooks/local-dev.md](docs/runbooks/local-dev.md)).

**Windows (PowerShell):** helper scripts under [`scripts/local/`](scripts/local/) write Go test output to **`.test-runs/<timestamp>/`**, E2E harness output to **`.e2e-runs/run-*/`**, and default the API to **port 18080** so you do not collide with Apache on **8080**. See [docs/testing/local-testing-guide.md](docs/testing/local-testing-guide.md) — *Windows PowerShell local full test workflow*.

**Environments:** [docs/runbooks/environment-strategy.md](docs/runbooks/environment-strategy.md) (local, staging, production). Per-environment **example** files (placeholders / safe local defaults only): [`.env.local.example`](.env.local.example), [`.env.staging.example`](.env.staging.example), [`.env.production.example`](.env.production.example).

## Configuration

Copy [`.env.example`](.env.example) to `.env` for local development (or start from [`.env.local.example`](.env.local.example)). Validate:

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

- [Enterprise release process](docs/operations/release-process.md) and [operator checklists](docs/operations/production-release-checklist.md) (production is **manual-only**; see [CI/CD enterprise contract](docs/operations/ci-cd-enterprise-contract.md))
- [Enterprise target model](docs/architecture/enterprise-target-model.md) and [transport boundary](docs/architecture/transport-boundary.md) (credentials + per-transport ownership)
- [Current architecture (as built)](docs/architecture/current-architecture.md), [data flow overview](docs/architecture/data-flow.md), [deployment topology](docs/architecture/deployment-topology.md)
- [P0 / P1 / P2 implementation roadmap](docs/architecture/p0-p1-p2-implementation-roadmap.md)
- [Target architecture](docs/architecture/target-architecture.md)
- [Strangler / migration strategy](docs/architecture/migration-strategy.md)
- [Documentation index](docs/README.md)
- [Machine gRPC (`avf.machine.v1`)](docs/api/machine-grpc.md), [internal gRPC reads (`avf.internal.v1`)](docs/api/internal-grpc.md), [Admin REST](docs/api/admin-rest.md)
- [Local testing guide](docs/testing/local-testing-guide.md) · [LOCAL_TEST_GUIDE.md](LOCAL_TEST_GUIDE.md)
- [2-VPS production runbook](docs/runbooks/production-2-vps.md)
- [2-VPS cutover and rollback](docs/runbooks/production-cutover-rollback.md)
- [2-VPS backup, restore, and DR](docs/runbooks/production-backup-restore-dr.md)
- [2-VPS day-2 incidents](docs/runbooks/production-day-2-incidents.md)
- [Field pilot checklist](docs/operations/field-pilot-checklist.md)
- [Field smoke tests](docs/runbooks/field-smoke-tests.md)
- [Temporal workflow runbook](docs/runbooks/temporal-workflows.md)
- [Machine activation runbook](docs/runbooks/machine-activation.md)
- [Operations runbook](ops/RUNBOOK.md) — incidents, dashboards/alert ideas, SQL checks
- [Metrics / signals](ops/METRICS.md)

## Makefile

See `Makefile` for common targets such as `make ci`, `make ci-gates`, `make verify-workflows`, `make build`, and the `prod-*` helpers.

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
