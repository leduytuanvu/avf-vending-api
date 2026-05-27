# Project cleanup audit

**Date:** 2026-05-26  
**Branch:** `main`  
**SHA:** `0b76099e8c1fcffd70e84a58650f130c96b3aeb0`

## Baseline

| Metric | Value |
|--------|-------|
| Total repo size (working tree) | **333M** |
| `.git/` | ~200M |
| `.e2e-runs/` | ~84M (119 production run dirs) |
| `docs/` | ~11M |
| `postman/` | ~22M |
| `tests/` | ~1.6M |

## Largest directories (pruned scan)

- `.git/`
- `.e2e-runs/` (local E2E evidence; gitignored)
- `postman/` (canonical + generated full suites)
- `docs/` (swagger.json ~1.5M, verification reports)

## Largest files (>1M, excluding `.git/`)

| Size | Path |
|------|------|
| 4.2M | `postman/production-full-suite/avf-production-full.postman_collection.json` |
| 4.2M | `postman/generated/rest/AVF_REST_FULL.postman_collection.json` |
| 4.2M | `postman/suites/full-production-suite/*.postman_collection.json` |
| 1.5M | `docs/swagger/swagger.json` |
| 1.5M | `.tmp-swagger-prod.json` (untracked) |
| 1.2–1.5M | `.e2e-runs/production/*/postman/newman-report.json` (per-run, gitignored) |

## Untracked local artifacts (60 paths in `git status`)

- **`.tmp-*`**: deploy candidate dirs, PR bodies, commit msgs, suite logs, swagger dump, gitleaks scratch (~30+ paths on disk)
- **`docs/testing/production-e2e/`**: 44 untracked generated RESULTS / duplicate MANUAL/API_TRACE/POSTMAN for non-canonical runs
- **`postman/environments/AVF Production E2E.environment.yaml`**: local desktop export (may contain placeholders; not canonical CI JSON)

## Tracked production E2E docs (KEEP)

- `API_TRACE_20260525T192300Z-1196-5901.md`
- `MANUAL_RETEST_GUIDE_20260525T192300Z-1196-5901.md`
- `POSTMAN_IMPORT_PARITY_20260525T192300Z-1196-5901.md`
- `RESULTS_20260523T081121Z-5910-29639.md`, `RESULTS_20260523T082020Z-9286-30835.md` (historical, tracked)
- `README.md`, `RESULT_TEMPLATE.md`, `postman-import.md`, `rest-route-coverage.md`

## Generated / cache directories

| Path | Status |
|------|--------|
| `.e2e-runs/` | gitignored; local only |
| `.test-runs/` | gitignored |
| `coverage/`, `dist/`, `build/`, `tmp/`, `temp/` | gitignored / absent |
| `__pycache__/` | gitignored |
| `.cache/` | not present; added to gitignore |

## Classification summary

### KEEP_REQUIRED

- All `internal/`, `cmd/`, migrations, proto, Docker/compose, `.github/workflows`, deploy scripts
- `tests/e2e/production/` manifest, runner, generators
- `postman/production/` canonical E2E collection/environment
- Tracked production E2E docs listed above
- OpenAPI/swagger sources used by CI/docs

### SAFE_DELETE (this cleanup)

- Untracked `.tmp-*` files and directories
- Untracked duplicate `docs/testing/production-e2e/RESULTS_*.md` and superseded MANUAL/API_TRACE/POSTMAN copies
- `.e2e-runs/production/*` except canonical pass run `20260525T192300Z-1196-5901`
- `.e2e-runs/*.log`, local debug scripts in `.e2e-runs/` root

### MOVE_OR_GITIGNORE (applied in `.gitignore`)

- `.tmp-*`, `.tmp-*/`
- `postman/environments/*.yaml`
- `manual-import-parity-newman-report.json`, `*.postman_environment.local.json`
- `.cache/`

### REVIEW_REQUIRED (not deleted)

- Tracked older `RESULTS_20260523T*.md` in git
- `docs/reports/**` verification history
- Duplicate 4.2M Postman full-suite JSON trees (referenced by suite generators/CI)
- `postman/environments/` policy if team wants YAML tracked later

## Inventory files (local temp)

- `/tmp/large-files.txt` — large file listing (session)
- `/tmp/generated-dirs.txt` — empty (no stray `dist/`/`coverage/` outside gitignore)
- `/tmp/temp-files.txt` — empty (no `*.bak`/`*.orig` in tree)

## Reference checks

- No CI script requires untracked `.tmp-*` paths
- `generate_e2e_results_report.py` writes `RESULTS_<runId>.md` optionally; canonical evidence retained in tracked MANUAL/API_TRACE/POSTMAN for `20260525T192300Z-1196-5901`
- Newman reports live under `.e2e-runs/` (regenerable)
