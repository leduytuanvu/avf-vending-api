# Production Deploy Report — Machine Runtime Fleet

**Date:** 2026-07-04  
**Verdict:** **BLOCKED_BY_MERGE_AND_CI**

## Pre-deploy checklist

| Item | Status |
|------|--------|
| PR #409 merged to `develop` | **NOT DONE** — awaiting CI |
| `develop` merged to `main` | **NOT DONE** |
| pg_dump backup | **NOT RUN** |
| `/version` + migration version recorded | **NOT RUN** |
| Deploy workflow from `main` with `run_migration=true` | **NOT RUN** |

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
