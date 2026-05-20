# Production troubleshooting

Quick reference for production deploy and verification issues. Full phased evidence: [`../reports/production-deploy/PRODUCTION_DEPLOY_FAILURE_ANALYSIS.md`](../reports/production-deploy/PRODUCTION_DEPLOY_FAILURE_ANALYSIS.md).

## Migration script Permission denied

**Symptom:** Deploy fails at `release_app_node` → `production-migrate.sh` with `verify_database_environment.sh: Permission denied`.

**Root cause:** Nested `.sh` invoked via `exec` without execute bit after tar sync (`100644` in git).

**Fix (merged to main):** `run_script()` helper in `deployments/prod/shared/scripts/lib_release.sh`; post-tar `chmod +x` in `deploy-prod.yml`; migration scripts call `run_script` instead of bare `exec`.

**Verify:** `bash scripts/ci/validate-production-deploy.sh` — nested `.sh` scan and `run_script` presence.

## Deploy workflow green but `/version` git_sha stale

**Symptom:** `/health/live` and `/health/ready` return 200; `/version` shows old `git_sha`.

**Likely cause:** Runtime `APP_GIT_SHA` in server `.env.app-node` overrides link-time embed (`internal/config/config.go`).

**Action:** Align or unset `APP_GIT_SHA` on next env sync — not a migration regression if deploy logs show successful image pull and goose apply.

## Migration gate failures

See [migration-safety.md](../runbooks/migration-safety.md), [backup-evidence-for-production-migrations.md](../runbooks/backup-evidence-for-production-migrations.md), and [deploy-failure.md](../runbooks/deploy-failure.md).

## Retry deploy inputs

Phase 7 workflow inputs checklist: [`../reports/production-deploy/PRODUCTION_DEPLOY_RETRY_INPUTS.md`](../reports/production-deploy/PRODUCTION_DEPLOY_RETRY_INPUTS.md).
