# Manual database cleanup (outside goose)

Scripts in this directory are **not** run by `goose up`, **not** scanned by `scripts/ci/verify_migrations.sh`, and **must not** be wired into automatic deploy migrations.

Use them only during a planned maintenance window after explicit operator approval.

## `single_company_scope_destructive_cleanup.sql`

**Purpose:** Remove legacy multi-entity scope columns (`scope_id`, `company_target_id`), dependent constraints and indexes, normalize promotion/feature target rows away from the retired `company` target kind, and drop the `companies` table — matching the historical shape that was previously embedded in goose migration `00073` before CI migration-safety policy required splitting destructive work out of automatic `Up` migrations.

### Prerequisites

1. **Backup:** Full logical or volume backup of the Postgres instance; record backup id and restore drill notes.
2. **Maintenance window:** No conflicting schema migrations or deploys; brief API read-only or downtime if required.
3. **Approval:** Engineering + DBA sign-off in your change process.
4. **Application version:** Deploy must already rely on the single-company runtime (queries and services that no longer read `scope_id` / `companies`).

### Execution

1. Open a `psql` session (or trusted SQL runner) against the **target** database.
2. Review the script contents end-to-end.
3. Run in a transaction first if your process requires:

   ```sql
   BEGIN;
   \i docs/runbooks/manual-db-cleanup/single_company_scope_destructive_cleanup.sql
   -- Verify queries below; then COMMIT or ROLLBACK
   ```

4. On success, commit.

### Verification (examples)

- Confirm `companies` no longer exists: `\dt companies` → should not list the table.
- Confirm no `scope_id` on expected tables: query `information_schema.columns` for `column_name = 'scope_id'` in `public`.
- Smoke API: admin fleet list, commerce checkout path, telemetry reconcile — per your release checklist.

### Rollback

There is **no** automated `Down` for this script. Rollback is **restore from backup** or forward-fix with a new migration designed by engineers.

### Related

- [Migration safety](../migration-safety.md) — destructive patterns in goose `Up` are blocked in CI.
- Goose migration `00073_single_company_scope_consolidation.sql` — applies only non-destructive uniqueness indexes.
- Goose migration `00075_audit_events_relax_company_scope.sql` — drops legacy `audit_events` scope indexes and makes `scope_id` / `organization_id` nullable when those columns still exist, so enterprise audit INSERTs (auth login, etc.) no longer hit NOT NULL violations. After operators confirm API traffic and backups, optional manual follow-up: `ALTER TABLE audit_events DROP COLUMN IF EXISTS scope_id` and `DROP COLUMN IF EXISTS organization_id` (only if both code and SQL no longer reference them — canonical schema has neither column).
