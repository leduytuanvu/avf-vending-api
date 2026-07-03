# Production Backup — Inline pg_dump (Runtime Fleet Deploy)

**UTC:** 20260703T230500Z  
**Deploy run:** https://github.com/leduytuanvu/avf-vending-api/actions/runs/28686916171

## Mechanism

Inline backup executed during app-node deploy with `run_migration=true` per `scripts/deploy/production-migrate.sh` (documented in `docs/deployment/PRODUCTION_AUTO_MIGRATION.md`).

## Evidence (app-node deploy log)

From `production-deploy-evidence` artifact → `app-node-0-72.62.244.94-deploy.log`:

| Field | Value |
|-------|-------|
| Backup path | `/opt/avf-vending-api/deployments/prod/logs/migrations/backup-20260703T230257Z.dump` |
| Goose version before | `16` |
| Goose version after | `18` |
| Migrations applied | `00017_machine_runtime_fleet.sql`, `00018_machine_runtime_fleet_fixes.sql` |
| Migration gate | `closed` |
| Outcome | `production-migrate: PASS` |

Excerpt:

```
production-migrate: goose version before: 16
...
2026/07/03 23:04:12 OK   00017_machine_runtime_fleet.sql (479.43ms)
2026/07/03 23:04:12 OK   00018_machine_runtime_fleet_fixes.sql (300.14ms)
2026/07/03 23:04:12 goose: successfully migrated database to version: 18
...
production-migrate: PASS backup=/opt/avf-vending-api/deployments/prod/logs/migrations/backup-20260703T230257Z.dump version_before=16 version_after=18 migration_gate=closed
```

## Supplementary backup workflow

`production-backup-evidence.yml` was **not** triggered (`backup_evidence_id` empty in manifest). Inline backup is the primary required path for this deploy.

## Gate

**PASS** — inline `pg_dump` completed before goose up; migrations 00017/00018 applied successfully.
