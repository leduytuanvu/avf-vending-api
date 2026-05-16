# PR #217 — CI fix report

Date: 2026-05-16  
Branch: `test/openapi-json-body-shape-proof` (PR **#217**)

## 1. Root cause summary

| Failure | Cause |
|---------|--------|
| **Migration Safety** | Goose `Up` for `00073_single_company_scope_consolidation.sql` contained `DROP TABLE`, `DROP COLUMN` (including inside `EXECUTE format(...)` strings). `tools/verify_migrations.py` regex-matches those substrings in **`Up`** bodies; `DEPLOY_TARGET=ci` always fails with any destructive hit. |
| **Go CI Gates / sqlc** | `Makefile` pinned `SQLC_VERSION := v1.29.0` while committed `internal/gen/db/` was generated with **sqlc v1.31.1**. `make sqlc-check` regenerated headers with v1.29.0 → `git diff --exit-code -- internal/gen/db/` failed in CI. |

## 2. Migration safety fix

- **`migrations/00073_single_company_scope_consolidation.sql`**: `Up` now contains **only** additive `CREATE UNIQUE INDEX IF NOT EXISTS` statements (same indexes as before). No `DROP TABLE`, `DROP COLUMN`, `TRUNCATE`, or unsafe `DROP INDEX` in `Up`.
- **Manual destructive path** (not scanned by goose verifier):
  - `docs/runbooks/manual-db-cleanup/single_company_scope_destructive_cleanup.sql` — former PL/pgSQL teardown block (operator-run).
  - `docs/runbooks/manual-db-cleanup/README.md` — backup, maintenance window, approval, rollback, verification.
- **`docs/runbooks/migration-safety.md`**: cross-link to manual cleanup directory.

**CI bypass:** None. No `ALLOW_*` env vars added to workflows.

## 3. sqlc version drift fix

- **`Makefile`**: `SQLC_VERSION := v1.31.1` (matches committed codegen headers in `internal/gen/db/`).
- **Docs**: `README.md`, `docs/runbooks/final-production-readiness-signoff.md` updated to reference **v1.31.1**.
- **CI workflow**: `.github/workflows/ci.yml` already invokes `make api-contract-check`; no separate sqlc pin required.

## 4. Files changed (this fix)

| Path | Change |
|------|--------|
| `migrations/00073_single_company_scope_consolidation.sql` | Non-destructive `Up`; comments point to manual cleanup |
| `docs/runbooks/manual-db-cleanup/README.md` | New runbook |
| `docs/runbooks/manual-db-cleanup/single_company_scope_destructive_cleanup.sql` | New manual SQL |
| `docs/runbooks/migration-safety.md` | Related-docs link |
| `Makefile` | `SQLC_VERSION` → v1.31.1 |
| `README.md` | sqlc install pin |
| `docs/runbooks/final-production-readiness-signoff.md` | sqlc command pin |
| `reports/ci/pr217-ci-fix-analysis.md` | Detailed analysis |
| `reports/ci/pr217-ci-fix-report.md` | This file |

## 5. Commands run and results

### Migration safety (local)

```bash
DEPLOY_TARGET=ci bash scripts/ci/verify_migrations.sh --report ci-reports/migration-safety-report.json
```

**Result:** **PASS** — `destructive=0`, `blocked=false`, `exit_code=0`  
*(Report path is gitignored `ci-reports/`.)*

### `make check-placeholders check-wiring api-contract-check`

**Result:** **Not run** on this Windows agent — `make` and `rg` were not available in the default shell/PATH (`make: command not found`; `check_production_placeholders.sh` requires `rg`).  

**Equivalent sqlc drift check (ran):**

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
git diff --exit-code -- internal/gen/db/
```

**Result:** **PASS** (no diff).

### Go tests

```bash
go vet ./...
go test ./...
```

**Result:** **PASS** both.

### Forbidden org/tenant grep

```bash
git grep -n -I -i -E 'organization|organizations|...|requiretenantscope' -- '*.go' '*.sql' || true
```

**Result:** **PASS** (no matches; exit code 1 = no hits).

### Optional E2E

Not executed in this session (infra/time); no fabricated PASS.

## 6. Manual destructive DB cleanup path

Operators must run **`docs/runbooks/manual-db-cleanup/single_company_scope_destructive_cleanup.sql`** under the controls in **`docs/runbooks/manual-db-cleanup/README.md`** when legacy `scope_id` / `companies` DDL must be removed from long-lived databases.

## 7. Confirmation — CI not bypassed

- No changes to `tools/verify_migrations.py` destructive rules.
- No workflow env vars enabling destructive migrations in CI.
- No obfuscation of DDL (dynamic SQL was **removed** from goose `Up`, not added).

## 8. Follow-up for maintainers

- Install **GNU make** and **ripgrep (`rg`)** locally (or use Linux CI / devcontainer) to mirror `make check-placeholders check-wiring api-contract-check` exactly before merge if desired.
