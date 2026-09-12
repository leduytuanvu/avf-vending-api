# Ops toolchain retirement checklist

After PR1 (audit) and PR2 (wipe) are merged and verified on hosts:

## 1. Host sync verification

```bash
# On app-node after sync-ops-toolchain workflow
cat /opt/avf-vending-api/scripts/ops/TOOLCHAIN_SHA
# Must match merged main commit SHA
```

## 2. Staging rehearsal

1. `bash scripts/ops/run-clean-slate-audit.sh --environment staging --component all` (read-only)
2. `bash scripts/ops/run-environment-data-wipe.sh --environment staging` (dry-run)
3. With backup: execute wipe on staging with confirm tokens
4. Re-run audit to verify empty state

## 3. Production

1. Dry-run only first: `run-environment-data-wipe.sh --environment production`
2. Execute only with operator approval and backup evidence
3. Remove dangerous `.db-destroy-evidence/.bin/python3` from any host if present

## 4. Retire local copies

- Diff local untracked scripts vs tracked `scripts/ops/` at merged SHA
- Archive obsolete copies under `.local-archive/` (gitignored) or delete
- Do not scp working-tree files to production — use CI-built tar only

## 5. Deprecate legacy purge scripts

After wipe parity sign-off:

- `scripts/ops/run-production-purge.sh`
- `scripts/ops/run-production-purge-docker.sh`
- `scripts/ops/production-purge-*.sql` (thin-wrap or remove in follow-up PR)

Point operators to `docs/runbooks/database-wipe.md`.

## 6. Update incident runbook

Remove stale "DO NOT RUN" warnings from `db-audit-shim-cpu-incident.md` once host `TOOLCHAIN_SHA` is verified (already updated to reference tracked toolchain).
