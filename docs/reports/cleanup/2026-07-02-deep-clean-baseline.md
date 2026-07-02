# 2026-07-02 deep clean baseline

**Branch:** `cleanup/repo-deep-clean-docs-reports-scripts`  
**Parent commit (2026-07-02 repo cleanup):** `a35df0feaff8fa5bf16ab97fd34ade58b799ed6d`  
**Date:** 2026-07-02

## Repository state

| Check | Result |
|-------|--------|
| Prior pass committed | Yes — `chore(repo): restore Postman CI paths, remove deploy artifacts, archive reports` |
| Working tree | Clean (pre deep-clean changes) |
| Migrations | 15 files |
| Workflows | 20 files |

## Commands run

| Command | Result | Notes |
|---------|--------|-------|
| `go test ./... -short` | **PASS** | exit 0 |
| `go vet ./...` | **PASS** | exit 0 |
| `bash scripts/ci/verify_production_postman_parity.sh` | **PASS** | `POSTMAN_PARITY_CI_OK` |
| `bash scripts/ci/verify_migrations.sh` | **PASS** | 15 files, 0 findings |
| `bash scripts/ci/verify_workflow_contracts.sh` | **PASS** | workflow contract checks passed |
| `python tools/check_postman_artifacts.py` | **PASS** | OK |
| `python tools/check_markdown_links.py` | **FAIL (1)** | Broken relative link in `2026-07-02-repo-cleanup-final-report.md` → `.gitignore` (fix during deep clean) |
| `make postman-check` | **Not run** | `make` not on Git Bash PATH |
| `make api-contract-check` | **Deferred** | requires make + buf/sqlc |
| `make verify-enterprise-release` | **Deferred** | requires make + bash chain |

## Known pre-existing issues

- Markdown link checker: `docs/reports/cleanup/2026-07-02-repo-cleanup-final-report.md` uses `[../../.gitignore]` (should be `[../../../.gitignore]`).
- Postman generator `scripts/postman/generate_production_full_suite.py` still writes to legacy `postman/production-full-suite/` with old filenames (planned fix in this pass).
- Active `docs/reports/postman/`, `docs/reports/verification/*` reports, historical audits, and release evidence still under active paths (planned archive in this pass).

## Protected assets confirmed

- Migrations, workflows (`deploy-prod.yml`, `deploy-production.yml`), deployments, Postman CI paths (`postman/production/`, `postman/collections/`, `postman/environments/`).
- No production deploy or DB mutation performed.
