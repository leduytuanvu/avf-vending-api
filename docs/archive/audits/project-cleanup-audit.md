# Project cleanup audit

**Date:** 2026-05-28  
**Branch:** `chore/clean-project-nonessential-files`  
**Baseline commit:** `526b3032` (`develop`)

## Phase 0 — Baseline

| Command | Result |
|---------|--------|
| `git branch --show-current` | `chore/clean-project-nonessential-files` |
| `git log -1 --oneline` | `526b3032 fix: clamp production per-service DB pool defaults to DATABASE_MAX_CONNS (#319)` |
| `git status` (start) | clean after `git reset --hard origin/develop` |

### Size (before cleanup actions)

| Metric | Value |
|--------|-------|
| Working tree (excl. `.git`) | **~93.0 MB** |
| Tracked files only | **~34.5 MB** |
| `.git/` | **~197.5 MB** |
| Full directory total | **~290.5 MB** |

### Largest top-level directories (working tree, excl. `.git`)

| Path | Size |
|------|------|
| `.e2e-runs/` | 57.0 MB (local, gitignored) |
| `postman/` | 21.0 MB |
| `internal/` | 4.9 MB |
| `docs/` | 3.4 MB |

### Largest files (excl. `.git`, top 15)

| Size | Path | Notes |
|------|------|-------|
| 49.1 MB | `.e2e-runs/production/enterprise-postman-newman-report.json` | Local Newman output |
| 4.2 MB | `postman/production-full-suite/avf-production-full.postman_collection.json` | **KEEP** — CI Go audit test |
| 4.2 MB | `postman/suites/full-production-suite/AVF_FULL_100.postman_collection.json` | **KEEP** — regenerable, CI matrices |
| 4.1 MB | `postman/suites/full-production-suite/AVF_REST_365_FULL.postman_collection.json` | **KEEP** |
| 4.1 MB | `postman/generated/rest/AVF_REST_FULL.postman_collection.json` | **KEEP** — `scripts/postman/` generator |
| 1.5 MB | `postman/generated/API_INVENTORY_CANONICAL.json` | **KEEP** |
| 1.5 MB | `docs/swagger/swagger.json` | **KEEP** — OpenAPI source of truth |
| 369 KB | `docs/testing/production-e2e/API_TRACE_*.md` | **DELETE** — timestamped run report |
| 300 KB | `tools/build_openapi.py` | **KEEP** |
| 165 KB | `build/reports/api-grpc-mqtt-full-inventory.json` | **KEEP** — regenerable, referenced by audits |

## Generated / duplicate / local artifacts

| Category | Examples | Status |
|----------|----------|--------|
| Repomix output | `repomix-output.xml`, `.repomix/` | Not present; gitignored |
| E2E run dirs | `.e2e-runs/` | Local only — **delete from disk** |
| Deploy scratch | `.tmp-deploy-hotfix/`, `.tmp-deploy-candidate/` | Local only — **delete** |
| Python cache | `tools/__pycache__/` | **delete** |
| Timestamped E2E docs | `docs/testing/production-e2e/RESULTS_*.md`, `API_TRACE_*.md`, `POSTMAN_*_*.md` | Tracked — **git rm** (regenerable) |
| Postman audit MD | `postman/suites/full-production-suite/*_AUDIT*.md` | Tracked — **git rm** (regenerable; validator replaces) |
| MQTT coverage report | `docs/reports/test/mqtt-full-coverage.*` | Tracked — **git rm** (script-regenerable) |
| Superseded cleanup audits | `docs/audits/PROJECT_CLEANUP_*.md`, `DEEP_REPO_CLEANUP_*.md` | **Archive** → `docs/archive/cleanup/` |

## Phase 2 — Classification table

| Path | Size (approx.) | Type | Safe action | Reason | Risk | Validation |
|------|----------------|------|-------------|--------|------|------------|
| `.e2e-runs/**` | 57 MB | Local Newman/E2E | **Delete** | Gitignored run output | None | Not in `git ls-files` |
| `.tmp-deploy-*` | <50 KB | Local deploy scratch | **Delete** | Operator download dirs | None | Untracked |
| `tools/__pycache__/**` | ~200 KB | Python cache | **Delete** | Regenerated | None | — |
| `docs/testing/production-e2e/RESULTS_*.md` | ~100 KB | Timestamped report | **git rm** | `.gitignore` + template regen | Low | Keep `RESULT_TEMPLATE.md` |
| `docs/testing/production-e2e/API_TRACE_*.md` | 369 KB | Timestamped trace | **git rm** | Regenerable E2E output | Low | — |
| `docs/testing/production-e2e/POSTMAN_*_*.md` | varies | Timestamped audit | **git rm** | Superseded by CI validator | Low | — |
| `postman/suites/.../*_AUDIT*.md` | <500 KB | Generator audit MD | **git rm** | Listed in `.gitignore`; regen via validator | Low | `validate_generated_assets.py` |
| `docs/reports/test/mqtt-full-coverage.*` | <50 KB | Test report | **git rm** | `scripts/test/run-mqtt-full-coverage.sh` | Low | Optional regen |
| `docs/audits/PROJECT_CLEANUP_*.md` | small | Old audit | **Archive** | Historical traceability | None | README in archive |
| `postman/**/*.postman_collection.json` | ~20 MB | Generated Postman | **Keep** | CI / Go tests / generators | **High** if deleted | `postman_full_suite_audit_test.go`, parity CI |
| `docs/swagger/swagger.json` | 1.5 MB | OpenAPI | **Keep** | Contract + CI | **High** | `openapi_verify_release.py` |
| `migrations/**` | 134 KB | DB schema | **Keep** | Production SoT | **Critical** | `verify_migrations.sh` |
| `proto/**` | 957 KB | gRPC | **Keep** | API surface | **Critical** | `make proto-check` |
| `.github/workflows/**` | 499 KB | CI/CD | **Keep** | Required | **Critical** | workflow contracts |
| `build/reports/api-grpc-mqtt-full-inventory.json` | 165 KB | Inventory JSON | **Keep** | Referenced by market-readiness docs | Medium | `generate_market_readiness_inventory.py` |
| `vendor/` | n/a | Go vendor | **N/A** | Not committed (go modules) | — | — |

### Buckets summary

- **A (deleted):** `.e2e-runs/`, `.tmp-deploy-*`, `__pycache__`, 16 tracked regenerable reports (see Phase 3).
- **B (kept):** All application source, migrations, proto, OpenAPI, Postman generators + collections, workflows, deployments, canonical docs.
- **C (archived):** Five superseded cleanup audit documents under `docs/archive/cleanup/`.
- **D (needs human review):** None for this pass; large Postman JSON kept intentionally.

## Unsafe to delete (explicit)

- `cmd/`, `internal/`, `pkg/`, `api/`, `tests/`, `testdata/`
- `migrations/`, `db/schema/`, `proto/`
- `docs/swagger/`, `tools/build_openapi.py`, `tools/openapi_verify_release.py`
- `postman/production/` (CI Postman parity)
- `postman/suites/full-production-suite/generate_full_postman_suite.py`, `validate_generated_assets.py`
- `postman/production-full-suite/` (Go audit + product flow)
- `.github/`, `deployments/`, `Makefile`, `scripts/ci/`

## Phase 3–5 actions (this branch)

See [`project-cleanup-final-report.md`](project-cleanup-final-report.md) for executed deletions, `.gitignore` updates, and validation results.
