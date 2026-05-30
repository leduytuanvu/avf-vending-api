# Deep repository cleanup audit (2026-05-20)

**Branch:** `chore/deep-cleanup-junk-docs-tests` (from `develop`)  
**Scope:** Junk removal, consolidate docs to canonical set, remove obsolete skipped tests.

---

## Must delete (local, untracked)

| Path | Reason | Evidence |
|------|--------|----------|
| `.go-tmp/`, `.tmp-phase*/`, `.tmp-image-metadata/` | Deploy/build scratch | `.gitignore`; not in `git ls-files` |
| `tmp/`, `ci-reports/`, `bin/` | Local artifacts | `.gitignore`; not tracked |
| `repomix-output.xml` (~22 MB) | Generated code dump | `.gitignore`; not tracked |

---

## Must keep

- `migrations/**`, `deployments/**`, `.github/workflows/**`, `cmd/**`, `internal/**`, `pkg/**`, `api/**`, `proto/**`, `db/**`
- CI/deploy scripts, Postman/OpenAPI canonical JSON, active tests, production runbooks
- `docs/operations/**` CI contract stubs (workflows grep these paths)
- `docs/architecture/UUID_V7_POLICY.md`, `scripts/audit/verify-uuid-v7.sh`

---

## Markdown classification

| Class | Action | Paths |
|-------|--------|-------|
| **KEEP_CANONICAL** | Keep | `README.md`, `docs/README.md`, `docs/architecture/*`, `docs/runbooks/*`, `docs/deployment/*`, `docs/production/*` (checklists) |
| **KEEP_PRODUCTION_CRITICAL** | Keep + index | `docs/reports/production-deploy/PRODUCTION_DEPLOY_FAILURE_ANALYSIS.md`, `PRODUCTION_DEPLOY_RETRY_INPUTS.md` |
| **DELETE_OLD_REPORT** | Delete | `PRODUCTION_DEPLOY_FAILURE_REPORT.md`, `PRODUCTION_DEPLOY_WITH_DB_MIGRATION_REPORT.md`, `PUSH_MERGE_RECOVERY_REPORT.md` — superseded by FAILURE_ANALYSIS Phases 0–9 |
| **DELETE_OLD_REPORT** | Delete | `docs/reports/test/develop-merge-verification-report.md`, `final-remove-scope-id-100-percent-report.md` — merge-scope PR evidence, 0 CI refs |
| **DELETE_OLD_REPORT** | Delete | `docs/reports/final-gate-audit/*` — historical gate; conclusions in repo state |
| **DELETE_OLD_REPORT** | Delete | `docs/reports/final-single-scope-audit/final-report.md` — narrative only; keep generated `postman-import-check-report.md` |
| **DELETE_OLD_VERSION** | Delete | `docs/reports/product-media-offline-cache/phase0–phase5*.md` — interim phases; finals + runbook remain |
| **DELETE_DUPLICATE** | Delete | `REPO_CLEANUP_*`, `REPO_JUNK_*`, `REPO_STRUCTURE_CLEANUP_AUDIT.md` — superseded by this deep cleanup |
| **KEEP_CANONICAL** | Keep | `UUID_V7_AUDIT_REPORT.md`, `UUID_V7_STANDARDIZATION_AUDIT.md` (referenced from policy + migration 00005) |
| **KEEP_CANONICAL** | Keep | `docs/reports/verification/FULL_SYSTEM_FINAL_VERIFICATION_REPORT.md` + protocol verification set |
| **MERGE_INTO_CANONICAL** | Create | `docs/production/TROUBLESHOOTING.md`, `DEPLOYMENT_RUNBOOK.md`, `README.md` |

---

## Test classification

| Class | Path | Decision |
|-------|------|----------|
| **DELETE_OBSOLETE** | `internal/httpserver/admin_scope_test.go` | Entire file skipped; obsolete company-scoped REST |
| **DELETE_OBSOLETE** | `internal/httpserver/admin_fleet_scope_http_test.go` | All tests skipped |
| **DELETE_OBSOLETE** | Skipped functions in `reporting_parse_test.go`, `operator/service_test.go`, `artifacts_http_test.go`, `grpcserver/server_auth_test.go` | Remove dead skipped tests; keep active tests in same files |

---

## Delete candidates (tracked)

See Phase 2 execution. Every deletion has grep evidence showing no CI/workflow dependency except updated doc indexes.
