# Production Auto-Migration Gate Audit

Generated: 2026-05-20

## Contract (required)

1. Resolve **digest-pinned** production images (`app_image_ref`, `goose_image_ref`)
2. **Backup database** before migration
3. Run migrations from **embedded `/app/migrations`** via `/app/migrate` (same app image or dedicated goose image)
4. On migration failure → deployment **must not** proceed; old app stays running
5. On success → deploy services → health checks → smoke tests → mark success
6. Logs show migration version before/after
7. Secrets from GitHub Actions / server env — **never printed**

## Workflow: `.github/workflows/deploy-prod.yml`

| Control | Implementation | Status |
|---------|----------------|--------|
| Manual-only trigger | `on.workflow_dispatch` only; no push/schedule | **PASS** |
| Digest-pinned images | Inputs `app_image_ref`, `goose_image_ref` `@sha256:…` | **PASS** |
| `run_migration` input | Default `true`; documented inline pg_dump backup | **PASS** |
| Migration on first node only | `RUN_MIGRATION_ON_FIRST_NODE: '1'` on app-node A; `'0'` on B | **PASS** |
| Smoke after ready | `APP_NODE_RUN_SMOKE_AFTER_READY: '1'` in deploy mode | **PASS** |
| No auto DB rollback | Manifest: `migration_rollback_policy: never_automatic` | **PASS** |
| Asset sync includes migrations | `tar … migrations` to app nodes | **PASS** |

## Scripts

| Script | Role | Status |
|--------|------|--------|
| `scripts/deploy/production-migrate.sh` | pg_dump backup + goose up; masks `DATABASE_URL` | **Present** |
| `scripts/deploy/validate_migration_image.sh` | Asserts `/app/migrations` + `/app/migrate validate` | **Present** |
| `deployments/prod/app-node/scripts/release_app_node.sh` | Invokes migrate before drain/rollout | **Wired** |
| `cmd/migrate/main.go` | `validate`, `status`, `version`, `up` | **Present** |
| `deployments/prod/Dockerfile` | Embeds migrations in image | **Present** |

## Migration runner env

- `COMPOSE_FILE` → `docker-compose.app-node.yml`
- `COMPOSE_ENV_FILE` → `.env.app-node` (DATABASE_URL source)
- `DRY_VALIDATE=1` / `--validate-only` for image-only validation

## Static validation run (local)

```bash
bash scripts/ci/verify_migrations.sh          # PASS
MIGRATIONS_DIR=migrations go run ./cmd/migrate validate  # PASS — 5 files
docker compose -f deployments/docker/docker-compose.yml config --quiet  # PASS
```

**Not run locally (requires production secrets / images):**

- `production-migrate.sh` against real Supabase
- `validate_migration_image.sh` against GHCR digest
- Full `deploy-prod.yml` workflow dispatch

## Workflow YAML validation

```bash
# Requires actionlint on PATH
VERIFY_WITH_WORKFLOWS=1 bash scripts/local/verify-full-system.sh
```

**Local status:** SKIPPED — `actionlint` not installed on verification host. CI job `ci-workflows` runs this gate.

Contract checker: `scripts/ci/verify_workflow_contracts.sh` + `tools/verify_github_workflow_cicd_contract.py`

## Secrets handling

- `production-migrate.sh` uses `mask_database_url()` — passwords redacted in logs
- Workflow uses `${{ secrets.* }}` — not echoed in run steps reviewed

## Gaps / risks

| Risk | Severity | Note |
|------|----------|------|
| actionlint not run locally | Low | CI covers workflow contracts |
| Image embed not validated without GHCR pull | Low | `validate_migration_image.sh` exists for pre-deploy |
| Auto-rollback does not reverse DB | Info | By design — document in runbooks |

## Verdict

**Production migration gate: PASS (static audit)** — workflow contract, scripts, and embedded migrate runner align with required deployment sequence. Live production proof requires operator workflow dispatch with real secrets.
