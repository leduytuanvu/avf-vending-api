# Repository cleanup — final report (2026-05-20)

**Branch:** `chore/repo-docs-source-cleanup`  
**Scope:** Documentation consolidation and `.gitignore` hardening only. No Go code, migrations, deploy scripts, or workflow changes.

---

## Summary

- Relocated **22 generated reports/audits** from `docs/testing/` to `docs/reports/` and `docs/audits/`
- Consolidated root `reports/test/` into `docs/reports/test/`
- Removed **3 unused files** (backup compose, orphan stub, duplicate Postman doc)
- Updated **docs/README.md**, reports/audits indexes, and cross-references
- Hardened **`.gitignore`** for local deploy scratch (`.tmp-phase*/`, `_verify*.txt`)

---

## Deleted files

| Path | Reason | Proof |
|------|--------|-------|
| `deployments/docker/docker-compose.yml.bak.local` | Local backup | 0 repo references |
| `docs/ci-cd/staging-production-gate.md` | Stub → `docs/cicd/` | 0 references to `docs/ci-cd/` |
| `postman/suites/full-production-suite/05_PRODUCTION_TEST_EXECUTION_ORDER.md` | Duplicate | Canonical: `docs/testing/05_PRODUCTION_TEST_EXECUTION_ORDER.md` |
| `docs/testing/_verify_full_system_last_run.txt` | Local run output | Not tracked / not CI input |

---

## Moved files

| From | To |
|------|-----|
| `docs/testing/PRODUCTION_DEPLOY_*.md`, `PUSH_MERGE_RECOVERY_REPORT.md` | `docs/reports/production-deploy/` |
| `docs/testing/*_VERIFICATION_*`, `FULL_*`, `FINAL_*`, Postman audits, E2E reports | `docs/reports/verification/` |
| `docs/testing/PRE_FLIGHT_*`, `UUID_V7_*`, `PRODUCTION_AUTO_MIGRATION_GATE_*` | `docs/audits/` |
| `reports/test/mqtt-full-coverage.*` | `docs/reports/test/` |

---

## Merged docs

No content merges — structural relocation only. Duplicate Postman `05_*` removed in favor of `docs/testing/05_PRODUCTION_TEST_EXECUTION_ORDER.md`.

---

## Updated references

- `tests/e2e/README.md` — destructive E2E report path
- `docs/testing/POSTMAN_FULL_FLOW_TESTING_GUIDE.md` — verification report paths
- Moved reports — internal cross-links (`FULL_SYSTEM_FINAL_VERIFICATION_REPORT.md`, etc.)
- `docs/README.md`, `docs/reports/README.md`, `docs/audits/README.md`

---

## Files intentionally not touched

- `migrations/**`, `deployments/prod/**` (except `.bak.local` delete)
- `.github/workflows/**`
- `Dockerfile*`, production compose files
- `postman/collections/*.json`, `postman/environments/*.json`
- `scripts/ci/validate-production-deploy.sh`, deploy/release scripts
- `docs/operations/**` CI stubs
- All Go packages, tests, proto, swagger generated output

---

## Validation results

| Command | Result |
|---------|--------|
| `gofmt -w .` | PASS |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `go list ./...` | PASS |
| `bash scripts/ci/validate-production-deploy.sh` | PASS |
| `scripts/audit/verify-uuid-v7.sh` | PASS |
| `scripts/checks/check-uuid-v7.sh` | PASS |
| JSON validation (Python) | PASS |
| `bash -n` on `scripts/` + `deployments/**/*.sh` | PASS |

---

## Risks

| Item | Notes |
|------|-------|
| `docs/operations/` stubs | Still required by CI; removal deferred to P2 workflow update |
| External bookmarks | Old `docs/testing/PRODUCTION_*` URLs broken — use `docs/reports/production-deploy/` |
| Postman suite embedded text | May mention removed duplicate path; regenerate suite when convenient |

---

## Next recommended cleanup PR

1. Update workflows + `verify_workflow_contracts.sh` to canonical `docs/deployment/` paths; remove `docs/operations/` stubs
2. Consolidate duplicate Postman audit docs under `docs/reports/postman/`
3. Merge `local-e2e.md` / `p06-local-e2e.md` with cross-links

See [`REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md`](REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md).
