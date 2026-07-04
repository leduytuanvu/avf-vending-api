# Project cleanup — final report

**Date:** 2026-05-28  
**Branch:** `chore/clean-project-nonessential-files`  
**Base:** `526b3032` (`develop`)

## Summary

Nonessential local and tracked artifacts were removed or archived. **No application runtime logic, routes, migrations, workflows, or deployment files were changed.** Postman suite was regenerated to refresh `manifest.json` and matrices; coverage counts unchanged (**REST 329**, **gRPC 86**, **MQTT 28**).

## Size

| Metric | Before | After |
|--------|--------|-------|
| Working tree (excl. `.git`) | ~93 MB | **~34 MB** |
| Tracked files | ~34.5 MB | **~34.0 MB** (net −~0.5 MB tracked docs removed) |
| Local `.e2e-runs/` | ~57 MB | **0** (deleted) |

## Repomix (historical — config removed 2026-07-04)

At the time of this report, the repo used `repomix.config.json` and a generation guide under `docs/operations/`. Both were **removed** in PR #416; local `repomix-output*.xml` remains gitignored via [`.gitignore`](../../../.gitignore). Ad-hoc packs may still be generated with `npx repomix@latest --include "..."` if needed for external review.

| Metric (2026-05-28 snapshot) | Value |
|--------|-------|
| Command (historical) | `npx repomix@latest --config repomix.config.json` |
| Files packed | 1,734 |
| Tokens | ~5.27M |
| Output size | `repomix-output.xml` ~17.0 MB (gitignored) |
| Excluded | Large `.postman_collection.json`, E2E dirs, `build/reports/`, run logs |

## Deleted (Bucket A)

### Local only (not in git)

- `.e2e-runs/` (entire tree, ~57 MB Newman/E2E evidence)
- `.tmp-deploy-hotfix/`, `.tmp-deploy-candidate/`
- `tools/__pycache__/`

### Tracked — `git rm`

**Production E2E timestamped reports** (`docs/testing/production-e2e/`):

- `API_TRACE_20260525T192300Z-1196-5901.md`
- `RESULTS_20260523T081121Z-5910-29639.md`
- `RESULTS_20260523T082020Z-9286-30835.md`
- `MANUAL_RETEST_GUIDE_20260525T192300Z-1196-5901.md`
- `POSTMAN_ENTERPRISE_AUDIT_20260525T192300Z-1196-5901.md`
- `POSTMAN_ENTERPRISE_IMPORT_PARITY_20260525T192300Z-1196-5901.md`
- `POSTMAN_ENTERPRISE_REQUEST_RESPONSE_TRACE_20260525T192300Z-1196-5901.md`
- `POSTMAN_IMPORT_PARITY_20260525T192300Z-1196-5901.md`

**Regenerable test reports:**

- `docs/reports/test/mqtt-full-coverage.json`
- `docs/reports/test/mqtt-full-coverage.md`

**Postman audit markdown** (regenerated locally by `generate_full_postman_suite.py`; remain **gitignored**, not re-committed):

- `REST_COVERAGE_AUDIT.md`, `POSTMAN_IMPORT_VALIDATION_REPORT.md`
- `EMPTY_BODY_AUDIT_REPORT_VI.md`, `EMPTY_URL_AUDIT_REPORT_VI.md`
- `POSTMAN_SUITE_REVIEW_REPORT_VI.md`, `POSTMAN_VARIABLE_AUDIT_REPORT.md`

## Archived (Bucket C)

Moved to `docs/archive/cleanup/`:

- `PROJECT_CLEANUP_AUDIT.md`, `PROJECT_CLEANUP_RESULT.md`, `PROJECT_CLEANUP_DELETE_PLAN.md`
- `DEEP_REPO_CLEANUP_AUDIT.md`, `DEEP_REPO_CLEANUP_FINAL_REPORT.md`
- `README.md` (archive index)

## Updated configuration

- **`.gitignore`:** `.repomix/`, `repomix-output*.xml`, `docs/reports/test/mqtt-full-coverage.*`
- **`repomix.config.json` (removed 2026-07-04):** formerly excluded E2E/smoke/latency dirs, Postman collections, `build/reports/`, timestamped E2E docs
- **`docs/README.md`:** archive pointer (Repomix guide removed 2026-07-04)
- **`docs/audits/README.md`:** New cleanup audit links

## Regenerated (Phase 6)

```text
python postman/suites/full-production-suite/generate_full_postman_suite.py  → PASS
python postman/suites/full-production-suite/validate_generated_assets.py    → VALIDATION_PASS (329/86/28)
python tools/check_postman_artifacts.py                                     → OK
```

Committed matrix/manifest deltas: `manifest.json`, operation matrix CSVs, minor `catalog.proto` copy under suite `grpc/proto/`.

## Validation (Phase 8)

| Check | Result |
|-------|--------|
| `gofmt -l .` | **PASS** (empty) |
| `go vet ./...` | **PASS** |
| `go test ./...` | **PASS** |
| `python tools/openapi_verify_release.py` | **PASS** (329 operations in OpenAPI; route-doc registry message may still say 327 — pre-existing registry drift) |
| `validate_generated_assets.py` | **PASS** |
| `check_postman_artifacts.py` | **PASS** |
| `bash scripts/check_migrations.sh` | **SKIP** (no bash/WSL on Windows agent) |

## Behavior unchanged (Phase 9)

| Item | Status |
|------|--------|
| Go application logic | **Unchanged** |
| HTTP routes / OpenAPI paths | **Unchanged** (329 ops) |
| Proto / migrations | **Unchanged** |
| GitHub workflows | **Unchanged** |
| Deploy / Docker | **Unchanged** |
| Test fixtures (`testdata/`) | **Unchanged** |
| Postman REST/gRPC/MQTT counts | **Unchanged** (329/86/28) |

## Intentionally kept

- All `postman/**/*.postman_collection.json` (CI + generators)
- `docs/swagger/swagger.json`
- `build/reports/api-grpc-mqtt-full-inventory.json`
- `postman/production/` E2E manifest collection
- Canonical docs under `docs/production/`, `docs/runbooks/`, `docs/deployment/`

## Future manual review

- Consider raising `DATABASE_MAX_CONNS` on production VPS (operational, not repo cleanup).
- Regenerate `docs/reports/test/mqtt-full-coverage.*` when running `scripts/test/run-mqtt-full-coverage.sh`.
- Align OpenAPI route-doc registry count (327) with live OpenAPI (329) in a separate change if desired.
