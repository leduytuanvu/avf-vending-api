# develop merge verification — single-company scope removal

Date: 2026-05-18  
Source branch: `test/openapi-json-body-shape-proof`  
Target branch: `develop`  
PR: [#221](https://github.com/leduytuanvu/avf-vending-api/pull/221) — *fix: complete single-company scope removal*

## Branch tip (pre-merge on source branch)

- Latest commit on branch: `cda359c51faa4db6d6b67adb321f4bb5a0bf7b78`
- Recent commits on branch:
  - `6afded1` — `fix: complete single-company scope removal`
  - `cda359c` — `fix: unblock CI migration gate and native Postman generator`

## develop baseline before merge

- `origin/develop` at verification time: `742658b3a3989a7a31604659f2c4306edb4ba174`

## Files changed summary (PR scope, high level)

- **SQL / schema:** `db/queries/*.sql`, `db/schema/01_platform.sql`, sqlc outputs under `internal/gen/db/`, migration marker `migrations/00076_drop_legacy_scope_organization_tenant.sql`, manual teardown `docs/runbooks/manual-db-cleanup/drop_legacy_scope_organization_tenant.sql`.
- **Runtime:** `internal/httpserver/*`, `internal/modules/postgres/*`, catalog/fleet/feature-flag/reporting paths, removal of `internal/app/api/scope_errors.go`, integration tests including `fleet_site_integration_test.go`.
- **Contracts / docs:** `docs/swagger/swagger.json`, `docs/postman/*.json`, testing docs, Postman generator scripts added under `postman/full-production-suite/` (`generate_full_postman_suite.py`, `validate_generated_assets.py`).
- **Tooling:** `tools/build_openapi.py`, `tools/build_postman_collection.py`, `tools/postman/collection_test.js`, loadtest helpers.

## Verification commands and results (local)

| Gate | Result |
|------|--------|
| `sqlc generate` | PASS |
| `python tools/build_openapi.py` | PASS |
| `python postman/full-production-suite/generate_full_postman_suite.py` | PASS (`VALIDATION_PASS`, REST 325) |
| `python tools/build_postman_collection.py` then `git diff --exit-code docs/postman/` | PASS (no drift vs committed native Postman) |
| `gofmt` + `go vet ./...` | PASS |
| `go test ./... -count=1` | PASS |
| `DEPLOY_TARGET=ci bash scripts/ci/verify_migrations.sh` | PASS (after 00076 reduced to non-destructive marker) |
| Forbidden-token `git grep` (pathspec excludes per project gate, incl. `reports/**`, `docs/runbooks/manual-db-cleanup/**`, migrations `00000`–`00075`) | PASS — empty |
| `bash tests/e2e/run-all-local.sh --fresh-data` | PASS — exit 0, failed=0, skipped=0 |

### E2E run directory (latest full pass)

`.e2e-runs/run-20260518T021608Z-20847-26868`

## CI status (PR #221)

All required checks **SUCCESS** on workflow run `26010299549` (CI), `26010299548` (Linux race and contract gates), `26010299552` (Security jobs present).  
Examples: Migration Safety Check ✓, Go CI Gates ✓, Linux race and contract gates ✓.

**Merge blockers (GitHub policy):** `reviewDecision: REVIEW_REQUIRED` — PR is mergeable but blocked until an approving review (and any org rules) complete. `gh pr merge` without `--admin` fails by design.

## Remaining allowed legacy references

- **Historical goose migrations** `00000`–`00075` (excluded from forbidden grep gate).
- **`docs/runbooks/manual-db-cleanup/**`** — documents and SQL that intentionally mention legacy column names for operator teardown (excluded from forbidden grep gate).
- **Untracked local-only trees** such as `postman/collections/` (not in `git grep` scan unless tracked).

## Final decision

**BLOCKED** — CI is green and the branch is pushed, but **`develop` does not yet contain this work** because the PR cannot be merged without satisfying branch-protection review (and this environment cannot use `--admin` bypass).

**After** maintainers squash-merge PR #221 using the preferred method:

```text
gh pr merge 221 --squash --delete-branch
```

verify:

```bash
git fetch origin
git checkout develop
git pull --ff-only origin develop
git log -1 --oneline
```

Then update this report’s “Final decision” section to **MERGED_TO_DEVELOP** and record the squash merge commit SHA.
