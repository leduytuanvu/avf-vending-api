# Clean-slate reset runbook

TRUNCATE all application data while preserving schema and `goose_db_version`. Does **not** run `bootstrap-admin`.

## Prerequisites

- Quiesce writers: `bash scripts/db/quiesce_writers.sh --environment <env>`
- Verified Postgres backup for staging/production
- Fleet factory reset on deployed machines before restarting API (see `avf-vending-app` `RequestReCommissioningUseCase`)

## Audit (read-only)

See `docs/runbooks/database-audit.md`.

```bash
bash scripts/ops/run-clean-slate-audit.sh --environment development --component all
bash scripts/ops/run-clean-slate-audit.sh --environment staging --component all
bash scripts/ops/run-clean-slate-audit.sh --environment production --component all
```

## Wipe

See `docs/runbooks/database-wipe.md`. Default is **dry-run**; pass `--execute` for mutation.

```bash
# Dry-run (default)
bash scripts/ops/run-environment-data-wipe.sh --environment development

CONFIRM_DEV_DATA_WIPE=AVF-WIPE-DEV-DATA \
  bash scripts/ops/run-environment-data-wipe.sh --environment development --execute --phase all

CONFIRM_STAGING_DATA_WIPE=AVF-WIPE-STAGING-DATA BACKUP_PATH=... \
  bash scripts/ops/run-environment-data-wipe.sh --environment staging --execute --phase all

CONFIRM_PRODUCTION_DATA_WIPE=I_UNDERSTAND_THIS_WIPES_PRODUCTION \
  bash scripts/ops/run-environment-data-wipe.sh --environment production --execute \
    --allow-production --confirmation "WIPE PRODUCTION <database_name>" --phase all
```

## Component-only (production)

```bash
CONFIRM_PRODUCTION_DATA_WIPE=I_UNDERSTAND_THIS_WIPES_PRODUCTION make prod-redis-flush
CONFIRM_PRODUCTION_DATA_WIPE=I_UNDERSTAND_THIS_WIPES_PRODUCTION make prod-nats-purge   # data-node NATS_URL
CONFIRM_PRODUCTION_DATA_WIPE=I_UNDERSTAND_THIS_WIPES_PRODUCTION make prod-emqx-purge   # data-node EMQX API
```

Production EMQX/NATS phases run on **data-node** when `EMQX_MANAGEMENT_URL` / `NATS_URL` point to local services.

## Staging discovery (no local credentials)

```bash
make staging-discover
STAGING_HOST=... STAGING_SSH_USER=root make staging-discover-remote
```

## Evidence

Artifacts under `.db-destroy-evidence/` — never commit secrets.

## Process hygiene (audit / wipe)

- Let `run-table-data-audit.sh` and `run-environment-data-wipe.sh` run to completion; do not background them and disconnect.
- After any audit or wipe, confirm no orphaned host processes remain:

```bash
pgrep -af 'db-destroy-evidence|verify_database_environment\.sh' || echo none
uptime
```

If orphaned `.db-destroy-evidence/.bin/python3` processes appear, follow `docs/runbooks/db-audit-shim-cpu-incident.md` before starting another audit.

## After clean-slate PASS

Run `bootstrap-admin` in a **separate** step only after independent audit and observation window pass.
