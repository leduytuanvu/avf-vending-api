# Repository cleanup audit (2026-05-20)

**Branch:** `chore/repo-docs-source-cleanup`  
**Scope:** Markdown consolidation, source-tree hygiene, `.gitignore` hardening. No Go logic, migrations, deploy scripts, or workflow behavior changes.

---

## Current top-level structure

| Path | Purpose |
|------|---------|
| `.github/` | CI/CD workflows, dependabot, CODEOWNERS |
| `api/` | OpenAPI pointer; embed in `docs/swagger/` |
| `cmd/` | Go service binaries (api, worker, migrate, …) |
| `db/` | sqlc schema + queries |
| `deployments/` | Docker Compose (local/staging/prod), 2-VPS topology |
| `docs/` | Documentation hub |
| `internal/` | Application code |
| `migrations/` | **Goose SQL — production source of truth** |
| `postman/` | CI Postman collections + production suite |
| `proto/` | Protobuf sources + generated stubs |
| `scripts/` | CI, deploy, test, local helpers |
| `tests/` | Python script tests + bash E2E harness |
| `tools/` | Generators, loadtest, governance verifiers |

---

## Markdown inventory (summary)

| Classification | Count / examples |
|----------------|------------------|
| **KEEP** | `README.md`, `docs/architecture/*`, `docs/deployment/*`, `docs/production/*`, `docs/runbooks/*`, testing guides in `docs/testing/` |
| **MOVE** | 22 files from `docs/testing/` → `docs/reports/` or `docs/audits/` (executed in Phase 2) |
| **DELETE CANDIDATE** | `docs/ci-cd/staging-production-gate.md` (stub), `docker-compose.yml.bak.local`, duplicate Postman `05_*` |
| **GENERATED REPORT** | Relocated to `docs/reports/production-deploy/`, `docs/reports/verification/` |
| **PRODUCTION CRITICAL** | `deployments/prod/**`, `docs/operations/*` stubs (CI-grep), runbooks |
| **NEEDS REVIEW** | `docs/operations/` stubs until workflows updated (P2 — not in this PR) |

Full file-level inventory: see subagent audit + [`REPO_STRUCTURE_CLEANUP_AUDIT.md`](REPO_STRUCTURE_CLEANUP_AUDIT.md).

---

## Source tree inventory

| Area | Action |
|------|--------|
| `cmd/`, `internal/` | **No move** — import churn risk |
| `migrations/` | **Untouched** |
| `deployments/` | **Untouched** except delete `docker-compose.yml.bak.local` |
| `scripts/` | **Untouched** — wrapper/canonical pairs retained |
| `docs/` | Reorganized reports/audits |
| `postman/` | Delete duplicate suite doc only |
| `.tmp-*` | **Not committed** — added to `.gitignore` |

---

## Delete candidates (executed)

| Path | Why unused | Evidence | Risk |
|------|------------|----------|------|
| `deployments/docker/docker-compose.yml.bak.local` | Local backup | `git grep docker-compose.yml.bak` → 0 | Low |
| `docs/ci-cd/staging-production-gate.md` | Stub → `docs/cicd/` | `git grep docs/ci-cd/` → 0 | Low |
| `postman/suites/.../05_PRODUCTION_TEST_EXECUTION_ORDER.md` | Duplicate | Canonical: `docs/testing/05_*` | Low |
| `docs/testing/_verify_full_system_last_run.txt` | Local run output | No CI refs | Low |

**Not deleted:** `docs/operations/*` stubs — required by `scripts/ci/verify_workflow_contracts.sh` and workflows.

---

## Move candidates (executed)

| From | To | Reason |
|------|-----|--------|
| `docs/testing/PRODUCTION_*`, `PUSH_MERGE_*` | `docs/reports/production-deploy/` | Deploy evidence, not test guides |
| `docs/testing/*_VERIFICATION_*`, `FULL_*`, `FINAL_*` | `docs/reports/verification/` | Generated verification output |
| `docs/testing/PRE_FLIGHT_*`, `UUID_V7_*`, `PRODUCTION_AUTO_MIGRATION_GATE_*` | `docs/audits/` | Audit artifacts |
| `reports/test/mqtt-full-coverage.*` | `docs/reports/test/` | Phase-2 consolidation |

References updated in moved files and `tests/e2e/README.md`.

---

## Files that must not be touched

- `migrations/**`, `db/schema/**`, `db/queries/**`
- `deployments/prod/**` (except approved `.bak.local` delete)
- `.github/workflows/**`
- `Dockerfile*`, production `docker-compose*`
- `postman/collections/*.json`, `postman/environments/*.json`
- `scripts/ci/validate-production-deploy.sh`, `scripts/ci/verify_workflow_contracts.sh`
- `docs/operations/**` (CI contract stubs)
- `docs/swagger/swagger.json`, `docs/swagger/docs.go`
- All `tests/**` and Go packages under `cmd/`, `internal/`

---

## Secret scan

No secrets staged. Local `.tmp-phase*/` dirs contain deploy evidence only — gitignored, not committed.

---

## Verdict

Safe to proceed with doc moves and `.gitignore` updates. **Deferred:** `docs/operations/` stub removal (requires coordinated workflow + CI contract update per `REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md`).
