# Database audit toolchain (read-only)

Tracked ops scripts for **read-only** inspection of PostgreSQL row counts, Redis key inventory, EMQX/NATS state, and related components. These scripts **never** mutate production data.

## Prerequisites

- Repository checkout at a known `main` commit (verify `scripts/ops/TOOLCHAIN_SHA` on the host after sync).
- `scripts/ops/lib/python3_shim.sh` must be used for any `python3` child processes (see `docs/runbooks/db-audit-shim-cpu-incident.md`).
- `DATABASE_URL` or environment-specific `.env` files as documented per script.

## Entry points

| Script | Purpose |
|--------|---------|
| `scripts/ops/run-table-data-audit.sh` | Public-schema row-count audit for one environment |
| `scripts/ops/run-clean-slate-audit.sh` | Multi-component read-only audit (postgres, redis, emqx, nats, media, topology) |
| `scripts/ops/verify-goose-migration.sh` | Goose migration state report |
| `scripts/ops/staging-discover.sh` | Staging DNS/SSH discovery metadata |
| `scripts/ops/emqx-audit.sh` | EMQX machine users / retained messages (read-only) |
| `scripts/ops/nats-audit-streams.sh` | NATS JetStream stream inventory (read-only) |

## Examples

```bash
# Development (local docker postgres on :15432)
bash scripts/ops/run-table-data-audit.sh --environment development

# Staging / production — requires correct env files on the host
bash scripts/ops/run-clean-slate-audit.sh --environment production --component all
bash scripts/ops/run-clean-slate-audit.sh --environment staging --component postgres,redis
```

## Production guidance

- Run **off-peak** — full `COUNT(*)` over all public tables can load the database.
- Use read-only audits before any destructive wipe rehearsal.
- Evidence is written under `.db-destroy-evidence/` (gitignored). URLs are redacted; do not commit evidence files.

## Host sync

The default production deploy workflow does **not** copy the full `scripts/ops/` tree. After merge, use the **Sync ops toolchain** workflow (`sync-ops-toolchain.yml`) or verify `TOOLCHAIN_SHA` on the app node:

```bash
cat /opt/avf-vending-api/scripts/ops/TOOLCHAIN_SHA
```

## CI verification

```bash
bash scripts/ci/check_db_audit_readonly.sh
bash scripts/ops/tests/db_audit_readonly.test.sh
```

Clean-clone gate (optional, on operator workstation):

```bash
bash scripts/ci/check_db_audit_clean_clone.sh
```

## Related

- Destructive wipe: `docs/runbooks/database-wipe.md` (separate PR2 toolchain)
- Shim incident: `docs/runbooks/db-audit-shim-cpu-incident.md`
