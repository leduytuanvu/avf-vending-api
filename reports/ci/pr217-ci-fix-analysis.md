# PR #217 — CI failure analysis

Branch theme: **feat: consolidate single-company scope after organization removal**

Date: 2026-05-16

## 1. Migration Safety Check failure

### Why CI blocks migration `00073`

`DEPLOY_TARGET=ci` runs `scripts/ci/verify_migrations.sh` → `tools/verify_migrations.py`.

Policy (**always** for CI):

- Only goose **`Up`** bodies under `migrations/*.sql` are scanned for destructive patterns.
- Any match sets `blocked=true` and exit code **1**. There is **no** approval bypass for CI (see `docs/runbooks/migration-safety.md`).

### Destructive statements present (before fix)

The prior `00073_single_company_scope_consolidation.sql` **Up** section contained a large `DO $$ ... $$` PL/pgSQL block that included:

- **`DROP TABLE`** — `DROP TABLE IF EXISTS companies CASCADE`
- **`DROP COLUMN`** — dynamic `ALTER TABLE ... DROP COLUMN IF EXISTS scope_id` / `company_target_id` via `EXECUTE format(...)`

The verifier uses **regex on the raw Up text** (after stripping `--` and `/* */` comments). Strings inside `EXECUTE format('... DROP COLUMN ...')` still contain the substrings `DROP COLUMN` and `DROP TABLE`, so they **match** even though execution is dynamic.

Additional destructive-class patterns in that block included **`DROP CONSTRAINT`** (not individually listed in the policy table but **not** the reported CI failure) and **`DROP INDEX IF EXISTS`** (allowed because `IF EXISTS` is present).

### How migration safety expects destructive changes

Per `docs/runbooks/migration-safety.md`:

1. Prefer **additive** `Up` migrations (`CREATE`, `IF NOT EXISTS`, etc.).
2. Use phased rollout for breaking DDL; avoid destructive `Up` in CI-gated paths.
3. Staging/production may allow destructive migrations only with explicit **non-CI** env vars — **not** used for PR validation.

**Correct approach for this repo:** move destructive DDL out of `migrations/` automatic goose `Up` into an **operator-run manual SQL** path that **goose does not execute** and `verify_migrations.py` does **not** scan.

## 2. Go CI Gates / sqlc drift failure

### Where sqlc version is pinned

- **Primary pin:** `Makefile` variables `SQLC_VERSION` and `SQLC_GEN := $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)`.
- **CI:** `.github/workflows/ci.yml` job **Go CI Gates** runs `make api-contract-check`, which depends on `sqlc-check` → same `SQLC_GEN` as local/Makefile. There was **no** separate workflow pin; drift came from **Makefile `SQLC_VERSION` lagging** committed codegen.

### Why generated files drifted in CI

Committed files under `internal/gen/db/` include headers:

```text
//   sqlc v1.31.1
```

While `SQLC_VERSION` was **v1.29.0**, `make sqlc-check` in CI regenerated with **v1.29.0**, changing headers (and any incidental codegen diff) so `git diff --exit-code -- internal/gen/db/` failed.

### Preferred fix

Align **`SQLC_VERSION` with the committed generator version (`v1.31.1`)** so CI regeneration is a no-op diff.

## 3. Workflow references

| Gate | Workflow / entrypoint |
|------|------------------------|
| Migration Safety | `ci.yml` → `migration-safety` job → `DEPLOY_TARGET=ci bash scripts/ci/verify_migrations.sh --report ...` |
| sqlc / contract | `ci.yml` → `go-ci` job → `make check-placeholders check-wiring api-contract-check` |

## 4. Fix strategy summary (implemented separately)

1. **Migrations:** Strip destructive `Up` from `00073`; keep additive indexes only; relocate teardown SQL to `docs/runbooks/manual-db-cleanup/` + README procedures.
2. **sqlc:** Set `SQLC_VERSION := v1.31.1` in `Makefile`; refresh docs/README pins.

No changes to `verify_migrations.py` policy, no CI approval env vars, no hiding DDL in comments/strings to evade grep.
