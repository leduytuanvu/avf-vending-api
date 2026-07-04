# Project cleanup result

**Date:** 2026-05-26  
**Branch:** `chore/project-cleanup-safe` (from `develop`)  
**Base SHA at cleanup:** `0b76099e8c1fcffd70e84a58650f130c96b3aeb0` (`main`)

## Size

| Metric | Before | After |
|--------|--------|-------|
| Working tree total | **333M** | **245M** |
| `.e2e-runs/` | ~84M | **4.3M** (kept `20260525T192300Z-1196-5901` only) |

**Approximate savings:** ~88M local disk (untracked/gitignored artifacts).

## Deleted (local only, 182 delete operations)

- All `.tmp-*` scratch (deploy candidate, PR bodies, logs, swagger dump, suite logs)
- **118** historical `.e2e-runs/production/<runId>/` directories (kept canonical pass run)
- **44** untracked `docs/testing/production-e2e/RESULTS_*.md` and duplicate MANUAL/API_TRACE/POSTMAN copies
- `.e2e-runs` root debug/log scripts

## Kept (required)

- Tracked production E2E docs for `20260525T192300Z-1196-5901` (MANUAL, API_TRACE, POSTMAN parity)
- Tracked historical `RESULTS_20260523T081121Z`, `RESULTS_20260523T082020Z`
- Full source, migrations, workflows, Postman generators/manifests, `postman/production/` canonical JSON
- Local evidence: `.e2e-runs/production/20260525T192300Z-1196-5901/`

## Gitignore updates

- `.tmp-*/`, `.tmp-*`, `.cache/`
- `postman/environments/*.yaml`, `*.postman_environment.local.json`
- `manual-import-parity-newman-report.json`

## REVIEW_REQUIRED (not removed)

- Tracked `docs/reports/**` verification archives
- Large `postman/suites/full-production-suite/` and `postman/generated/` collections (CI/generator referenced)
- Tracked older `RESULTS_20260523*.md` in git

## Repomix

- Added `repomix.config.json` with safe excludes (`.e2e-runs`, `.tmp-*`, coverage, Newman reports, binaries, etc.) — **removed 2026-07-04** (PR #416); repomix outputs remain gitignored
- Does **not** exclude source, migrations, OpenAPI, core docs, manifests, tests, workflows, deploy scripts

## Validation

| Check | Result |
|-------|--------|
| `git diff --check` | PASS |
| `go test ./...` | PASS |
| `verify_workflow_contracts.sh` | PASS |
| `verify_production_postman_parity.sh` | PASS |
| `verify_github_governance.sh` | PASS |
| `verify_governance_protection_window.sh` | PASS |
| `python -m unittest discover -s tests/e2e/production/scripts` | PASS (6 tests) |

## Risk

**Low** — no tracked source/workflow/deploy files deleted; only local untracked/gitignored artifacts and gitignore/Repomix config committed.

## Production deploy

**Not required** — no backend logic, API, or harness behavior changed.

## Final verdict

**PROJECT_CLEANUP_SAFE_PASS**
