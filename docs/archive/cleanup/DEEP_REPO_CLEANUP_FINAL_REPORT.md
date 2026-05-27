# Deep repository cleanup — final report (2026-05-20)

**Branch:** `chore/deep-cleanup-junk-docs-tests`  
**Verdict:** CLEANUP_READY_TO_MERGE

---

## Summary

- Removed **local untracked junk** (~22 MB `repomix-output.xml`, `.tmp-phase*/`, `bin/`, `ci-reports/`, etc.)
- Deleted **20 tracked obsolete docs** (meta-audits, superseded deploy reports, interim phase reports, historical gate reports)
- Created **canonical production/testing indexes** and troubleshooting runbook
- Removed **2 obsolete test files** and **6 permanently skipped test functions**
- Hardened **`.gitignore`** with example-env negation rules
- Fixed **`tools/audit_postman_single_scope.py`** report output path to `docs/reports/`

---

## Deleted junk files/folders (local, untracked)

| Path | Reason |
|------|--------|
| `repomix-output.xml` | Generated code dump (~22 MB); already gitignored |
| `.go-tmp/`, `.tmp-phase7/8/9/`, `.tmp-image-metadata/` | Deploy/build scratch |
| `tmp/`, `ci-reports/`, `bin/` | Local artifacts |

---

## Deleted Markdown files

| Path | Reason | Canonical replacement |
|------|--------|----------------------|
| `docs/audits/REPO_CLEANUP_*`, `REPO_JUNK_*`, `REPO_STRUCTURE_CLEANUP_AUDIT.md` | Superseded meta-audits | `DEEP_REPO_CLEANUP_AUDIT.md` |
| `docs/reports/production-deploy/PRODUCTION_DEPLOY_FAILURE_REPORT.md` | Superseded by phased analysis | `PRODUCTION_DEPLOY_FAILURE_ANALYSIS.md` + `production/TROUBLESHOOTING.md` |
| `PRODUCTION_DEPLOY_WITH_DB_MIGRATION_REPORT.md` | Interim attempt report | FAILURE_ANALYSIS |
| `PUSH_MERGE_RECOVERY_REPORT.md` | Recovery merged into analysis | FAILURE_ANALYSIS |
| `docs/reports/test/develop-merge-verification-report.md` | Old PR evidence | — |
| `docs/reports/test/final-remove-scope-id-100-percent-report.md` | Old scope cleanup evidence | — |
| `docs/reports/final-gate-audit/*` | Historical gate narrative | Repo state + audits |
| `docs/reports/final-single-scope-audit/final-report.md` | Historical narrative | `postman-import-check-report.md` |
| `docs/reports/product-media-offline-cache/phase0–phase5*.md` | Interim phases | `phase6-sell-readiness.md`, `final-*` reports, runbook |

---

## Merged Markdown docs

| Source | Canonical doc |
|--------|---------------|
| Deploy failure / permission / stale version notes | `docs/production/TROUBLESHOOTING.md` |
| Deploy workflow index | `docs/production/DEPLOYMENT_RUNBOOK.md` |
| Testing guide index | `docs/testing/README.md` |

---

## Deleted tests

| Path | Reason | Replacement coverage |
|------|--------|---------------------|
| `internal/httpserver/admin_scope_test.go` | Entire file permanently skipped | Current admin scope in fleet/catalog tests |
| `internal/httpserver/admin_fleet_scope_http_test.go` | All tests skipped (obsolete company scope) | Active fleet admin tests |
| Skipped functions removed from `reporting_parse_test.go` | Obsolete company-scoped REST | `TestParseRequiredRFC3339Range`, `TestParseAdminCompanyReportingQuery_RejectsBadDateRange` |
| Skipped functions removed from `operator/service_test.go` | Single-company mode | Active operator session tests |
| `TestArtifactScopeAllowed_skippedLegacy` | Obsolete | `TestMountArtifactRoutes_smokeReserve` |
| `TestInternalQueryServices_RejectScopeScopeMismatch` | Obsolete single-company gRPC | Other internal gRPC auth tests |

---

## Files intentionally kept

- `migrations/**`, `deployments/**`, `.github/workflows/**`, Postman/OpenAPI, active tests
- `docs/operations/**` CI stubs, production runbooks, `UUID_V7_*` audits
- `docs/reports/production-deploy/PRODUCTION_DEPLOY_FAILURE_ANALYSIS.md` (full incident timeline)
- `docs/reports/verification/*` (protocol verification archive)

---

## .gitignore updates

- `/.env.*` with `!.env.example`, `!.env.local.example`, `!.env.production.example`, `!.env.staging.example`
- Explicit `.go-tmp/` under deploy scratch section

---

## Validation results

| Command | Result |
|---------|--------|
| `gofmt -w .` | PASS |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `go list ./...` | PASS |
| `validate-production-deploy.sh` | PASS |
| UUID v7 checks | PASS |
| JSON validation | PASS |
| `bash -n` shell scripts | PASS |
| Tracked junk / `.env` checks | PASS |

---

## Remaining risks

| Item | Notes |
|------|-------|
| `docs/reports/verification/*` | Many one-off verification reports retained; could consolidate further into FULL_SYSTEM_FINAL only |
| `docs/operations/` stubs | Still required by CI until P2 workflow path update |
| `docs/api/device-offline-replay-samples.md` | Unlinked gRPC doc; consider cross-link from machine-grpc.md |
