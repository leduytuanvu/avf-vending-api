# Production Deploy Report — Machine Runtime Fleet

**Date:** 2026-07-04  
**Verdict:** **BLOCKED_BY_MANUAL_DEPLOY_INPUTS**

## Pre-deploy checklist

| Item | Status |
|------|--------|
| PR #409 merged to `develop` | **DONE** @ `8991f526` |
| `develop` merged to `main` | **DONE** @ `277a3ad4` (PR #410) |
| Branch parity `develop..main` | **EMPTY DIFF** |
| pg_dump backup | **NOT RUN** (operator step) |
| Deploy workflow `deploy-prod.yml` | **NOT RUN** — requires manual `workflow_dispatch` with build_run_id, security_release_run_id, digest-pinned images, `DEPLOY_PRODUCTION` confirmation |

## Intended deploy

- Workflow: `.github/workflows/deploy-prod.yml`
- Migrations: `00017_machine_runtime_fleet.sql` + `00018_machine_runtime_fleet_fixes.sql`
- Env: `MACHINE_ONLINE_THRESHOLD_SECONDS=60`, `MACHINE_STALE_THRESHOLD_SECONDS=300`

## Post-deploy verification (pending)

- `/health/live`, `/health/ready`, `/version` SHA parity
- gRPC + MQTT reachability
- Migration version includes 00018

## Blocker

Production deploy cannot proceed until PR #409 merges to `develop`, `develop→main` parity is confirmed, and CI is green.
