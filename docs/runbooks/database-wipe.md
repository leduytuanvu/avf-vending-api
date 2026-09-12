# Database wipe toolchain (DESTRUCTIVE)

> **DESTRUCTIVE** — This runbook describes data wipe operations that **delete business data** across PostgreSQL, Redis, media, EMQX, and NATS. Default invocation is **dry-run**. Production execution requires multiple explicit gates.

## Safety contract

| Gate | Requirement |
|------|-------------|
| Default | Dry-run only (plan + counts, no mutation) |
| Execute | `--execute` + environment confirm token |
| Production | `--allow-production` + `--confirmation "WIPE PRODUCTION <database_name>"` |
| Backup | Staging/production require backup via `verify_backup_gate.sh` (dev may `--skip-backup`) |
| DROP DATABASE | **Prohibited** on production via default CLI; use scoped TRUNCATE wipe or DBA break-glass (`destroy_database_target.sh --break-glass-production-drop`) |

## Entry point

```bash
# Dry-run (default) — safe to run for planning
bash scripts/ops/run-environment-data-wipe.sh --environment staging

# Execute on development
CONFIRM_DEV_DATA_WIPE=AVF-WIPE-DEV-DATA \
  bash scripts/ops/run-environment-data-wipe.sh --environment development --execute --phase all

# Production execute (operator approval required)
CONFIRM_PRODUCTION_DATA_WIPE=I_UNDERSTAND_THIS_WIPES_PRODUCTION \
  bash scripts/ops/run-environment-data-wipe.sh --environment production --execute \
    --allow-production --confirmation "WIPE PRODUCTION avf_vending_prod" --phase all
```

Confirmation phrase must match the **parsed database name** from `verify_database_environment.sh`.

## Phases

`preflight` → `postgres` → `redis` → `media` → `emqx` → `nats` → `temporal` → `verify` → `all`

Phases are **not atomic** across systems. Partial failure emits evidence and exits non-zero.

## Redis strategy

- **Shared instance:** prefix-scoped `SCAN` + `UNLINK` (default, `REDIS_KEY_PREFIX`)
- **Dedicated instance:** `REDIS_DEDICATED_INSTANCE=1` allows `FLUSHDB`

## Evidence

Append-only JSONL under `.db-destroy-evidence/` (gitignored). Never commit evidence files.

## DROP DATABASE (staging/dev only)

```bash
bash scripts/db/destroy_database_target.sh --target-id TARGET-DB-003 --dry-run
```

Production targets (`TARGET-DB-005/006/007`) require `--break-glass-production-drop` and are **not** invoked by `run-environment-data-wipe.sh`.

## Related

- Read-only audit: `docs/runbooks/database-audit.md`
- Legacy purge (deprecated): `scripts/ops/run-production-purge.sh` — migrate to this toolchain
- Host sync: `.github/workflows/sync-ops-toolchain.yml` with `sync_mode: full`
