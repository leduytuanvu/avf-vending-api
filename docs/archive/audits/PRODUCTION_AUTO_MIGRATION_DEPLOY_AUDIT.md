# Production Auto-Migration Deploy Audit

**Date:** 2026-05-20  
**Scope:** Read-only audit before implementing automatic production DB migration on deploy  
**Branch baseline:** `main` (2-VPS app-node topology)

---

## 1. Current production deployment flow

Production deploy is **manual** via GitHub Actions **Deploy Production** (`.github/workflows/deploy-prod.yml`, `workflow_dispatch` on `main`, `environment: production`).

**High-level sequence today:**

1. Validate inputs (security verdict, staging evidence, image digests, topology)
2. Resolve digest-pinned `APP_IMAGE_REF` and `GOOSE_IMAGE_REF` from build artifacts
3. SSH tar-sync to each app VPS: `deployments/prod/app-node`, `shared`, **`migrations/`** from workflow checkout
4. Optional data-node release
5. **App node A:** `release_app_cluster.sh` → remote `release_app_node.sh`
6. **App node B:** same without migration
7. Final public smoke + deployment manifest artifact

**On-VPS app-node release order** (`release_app_node.sh`):

1. Validate prerequisites, write new image refs to `.env.app-node`
2. **Drain traffic** (stop Caddy)
3. Pull images
4. **If `RUN_MIGRATION=1`:** `verify_database_environment.sh` → `compose --profile migration run migrate` (goose up)
5. Recreate api/worker/reconciler/mqtt-ingest (+ optional temporal-worker)
6. Healthcheck app containers
7. Start Caddy, healthcheck edge

---

## 2. Workflow performing production deployment

| Workflow | Role |
|----------|------|
| `.github/workflows/deploy-prod.yml` | **Canonical** production deploy/rollback |
| `.github/workflows/deploy-production.yml` | Pointer only → use `deploy-prod.yml` |
| `.github/workflows/rollback-prod.yml` | Rollback preflight (no SSH deploy) |

**Concurrency:** `group: production-deployment`, `cancel-in-progress: false` (already serializes deploys).

---

## 3. Scripts used by production deployment

| Script | Purpose |
|--------|---------|
| `deployments/prod/shared/scripts/release_app_cluster.sh` | Rolling SSH to app nodes |
| `deployments/prod/app-node/scripts/release_app_node.sh` | Single-node pull/migrate/restart |
| `deployments/prod/shared/scripts/lib_release.sh` | Shared helpers, SSH wrapper |
| `deployments/prod/shared/scripts/healthcheck_app_node.sh` | Container readiness |
| `deployments/prod/scripts/smoke_prod.sh` | Public HTTP smoke (via GHA env) |
| `scripts/verify_database_environment.sh` | APP_ENV vs DATABASE_URL guard |
| `deployments/prod/shared/scripts/backup_managed_postgres.sh` | Client-side pg_dump (not wired into app-node migrate path) |

Legacy single-host path: `deployments/prod/scripts/release.sh` (backup → migrate → deploy) — **not** the 2-VPS GHA path.

---

## 4. How `APP_IMAGE_REF` is written to server

1. GHA job `resolve-image-refs` outputs digest-pinned refs from build artifacts  
2. `deploy-prod` passes `APP_IMAGE_REF` / `GOOSE_IMAGE_REF` to `release_app_cluster.sh`  
3. Remote `release_app_node.sh` calls `set_env_value` on `.env.app-node` before pull/migrate  

Example: `APP_IMAGE_REF=ghcr.io/leduytuanvu/avf-vending-api@sha256:...`

---

## 5. Where `DATABASE_URL` is sourced

- **On each app VPS** in `.env.app-node` (managed PostgreSQL / Supabase) — **not** injected by GitHub Actions during migrate
- Migrate service reads via Docker Compose `env_file`
- Guard: `scripts/db/verify_database_environment.sh` (masks password in logs)

---

## 6. Whether production image includes migrations

**No.** `deployments/prod/Dockerfile` builds Go binaries only; no `COPY migrations/`.

Migrations today come from:

1. Tar-sync of repo `migrations/` to VPS during GHA deploy, and  
2. Compose bind mount: `../../../migrations:/migrations:ro` on goose container

**Risk:** VPS disk copy can drift from deployed image commit; manual deploys may run stale SQL.

---

## 7. Existing migration runner

| Runner | Mechanism |
|--------|-----------|
| Compose `migrate` service | Separate `GOOSE_IMAGE_REF` container + host-mounted `/migrations` |
| `make migrate-up` | `go run github.com/pressly/goose/v3/cmd/goose@v3.27.0` |
| Legacy `release.sh` | Inline backup + compose migrate profile |

**No `cmd/migrate` binary** in app image today.

**Migration files:** `migrations/00001`–`00004` (includes `00003_seed_dev.sql` — dev seed; production DBs should already have goose version ≥ 3 applied).

---

## 8. Risks in current flow

| Risk | Severity | Detail |
|------|----------|--------|
| **Migration opt-in** | High | `run_migration` defaults to `false`; most deploys skip schema updates |
| **Drain before migrate** | High | Caddy stopped before migrate; failure leaves edge down while apps may still run |
| **No inline backup on app-node path** | High | Expects separate `backup_evidence_id` artifact when migration enabled |
| **Migrations not in image** | Medium | Tar-sync + bind mount ≠ same artifact as running app digest |
| **Dual entrypoints** | Medium | Legacy `docker-compose.prod.yml` vs app-node compose |
| **Dev seed migration** | Medium | `00003_seed_dev.sql` in same folder as production DDL |
| **No advisory lock** | Low | Mitigated by GHA concurrency; not enforced on manual shell deploy |

---

## 9. Exact files needing changes

| File | Change |
|------|--------|
| `deployments/prod/Dockerfile` | Build `cmd/migrate`, `COPY migrations /app/migrations` |
| `cmd/migrate/main.go` | New goose wrapper: validate/status/version/up |
| `go.mod` / `go.sum` | Add `github.com/pressly/goose/v3` |
| `deployments/prod/app-node/docker-compose.app-node.yml` | Migrate service uses `APP_IMAGE_REF` + `/app/migrate` |
| `scripts/deploy/production-migrate.sh` | **New** — backup, verify, advisory lock, migrate |
| `scripts/deploy/validate_migration_image.sh` | **New** — post-build image checks |
| `deployments/prod/app-node/scripts/release_app_node.sh` | Migrate before drain; call production-migrate.sh |
| `.github/workflows/deploy-prod.yml` | Default auto-migration on deploy; optional backup evidence |
| `docs/deployment/PRODUCTION_AUTO_MIGRATION.md` | **New** operator doc |
| `scripts/ci/verify_workflow_contracts.sh` | Update if backup_evidence gate changes |

**Intentionally unchanged:** `migrations/*.sql` content, `internal/**` business logic, goose down/reset paths.

---

## 10. Proposed implementation plan

### Phase 1 — Image-embedded migrations

- Add `cmd/migrate` reading `/app/migrations` and `DATABASE_URL`
- Embed `migrations/` in production Docker image at build time
- `validate` subcommand for CI and pre-migrate checks

### Phase 2 — `production-migrate.sh`

- Resolve/mask `DATABASE_URL`
- `pg_dump` + `pg_restore -l` via `postgres:17-alpine`
- PostgreSQL advisory lock for single-writer
- Status → up → status via `/app/migrate`
- Logs under `/opt/avf-vending-api/deployments/prod/logs/migrations/`

### Phase 3 — Deploy wiring

- Reorder `release_app_node.sh`: pull → **migrate (while Caddy still serving old apps if possible)** → drain → recreate apps
- GHA deploy mode: always `RUN_MIGRATION_ON_FIRST_NODE=1`
- Rollback mode: `RUN_MIGRATION=0`
- Keep `production-deployment` concurrency

### Phase 4 — Validation & docs

- `validate_migration_image.sh` after docker build
- Operator runbook `docs/deployment/PRODUCTION_AUTO_MIGRATION.md`
- `go test ./...` unchanged for business packages; add migrate cmd tests if needed

---

## Acceptance mapping

| Requirement | Implementation |
|-------------|----------------|
| Migrations from same commit/image | `COPY migrations` in Dockerfile |
| Auto migration before success | GHA deploy always runs migrate on node A |
| Migration failure fails workflow | `set -e` scripts + non-zero migrate exit |
| Concurrency | Existing `production-deployment` group |
| Backup before migrate | `production-migrate.sh` pg_dump |
| Secret masking | Existing verify script + log redaction |
| Idempotent up | goose up no-op when current |
| No reset/drop | No down/reset in deploy path |
| Health/smoke gates | Unchanged post-restart checks |
