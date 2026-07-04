# 2026-07-02 repository cleanup inventory

**Baseline:** `c69a5995` on branch `cleanup/repo-structure-and-junk-files`  
**Tracked files:** 1796

Classification key: **KEEP** | **MOVE** | **ARCHIVE** | **DELETE** | **MERGE** | **RESTORE**

---

## 1. Core source code — KEEP

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| `cmd/` | API, worker, mqtt-ingest, reconciler, temporal-worker, cli, migrate | Makefile `build`, Dockerfiles | HIGH | KEEP | `go build ./...` |
| `internal/app/`, `bootstrap/`, `config/`, `domain/` | Business logic, wiring, config | All binaries | HIGH | KEEP | `go test ./internal/...` |
| `internal/grpcserver/`, `httpserver/` | Transport layers | cmd/api | HIGH | KEEP | `go test ./internal/grpcserver/...` |
| `internal/modules/postgres/` | Persistence adapter (58+ importers) | App layer | HIGH | KEEP (P2 move deferred) | `go test ./...` |
| `internal/platform/`, `observability/`, `version/` | Shared infra | Multiple packages | HIGH | KEEP | `go vet ./...` |

---

## 2. Database and migrations — KEEP

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| `migrations/` (15 files) | goose production migrations | CI, Makefile, deploy scripts | HIGH | KEEP | `make check-migrations` |
| `db/queries/` | sqlc query sources | sqlc.yaml | HIGH | KEEP | `make sqlc-check` |
| `db/schema/01_platform.sql` | sqlc schema mirror | sqlc.yaml | HIGH | KEEP | `make sqlc-check` |
| `sqlc.yaml` | sqlc config | Makefile, CI | HIGH | KEEP | `make sqlc-check` |

---

## 3. Generated code — KEEP (do not hand-edit)

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| `internal/gen/db/` | sqlc output | App, tests | HIGH | KEEP | `make sqlc-check` |
| `internal/gen/avfinternalv1/` | Internal proto stubs | gRPC server | HIGH | KEEP | `make proto-check` |
| `proto/avf/machine/v1/*.pb.go` | Machine gRPC stubs | grpcserver | HIGH | KEEP | `make proto-check` |
| `proto/avf/v1/*.pb.go` | Legacy proto stubs | Codebase | MEDIUM | KEEP | `make proto-check` |

---

## 4. Protocol/API definitions — KEEP

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| `proto/` (sources + buf config) | Protobuf SoT | buf generate, Android sync docs | HIGH | KEEP | `make proto-check` |
| `api/openapi/` | OpenAPI pointer | Docs | LOW | KEEP | — |
| `docs/swagger/` | Generated OpenAPI embed | cmd/api, make swagger-check | HIGH | KEEP | `make swagger-check` |

---

## 5. Production/staging/deployment — KEEP

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| `.github/workflows/` (20 files) | CI/CD, deploy, security | verify_workflow_contracts.sh | HIGH | KEEP all | `make verify-workflows` |
| `.github/workflows/deploy-prod.yml` | Canonical production deploy | Runbooks, governance | HIGH | KEEP | governance script |
| `.github/workflows/deploy-production.yml` | Legacy pointer (no deploy) | verify_governance_protection_window.sh | HIGH | KEEP | governance script |
| `deployments/prod/app-node/`, `data-node/`, `shared/`, `observability/` | Primary 2-VPS prod | deploy-prod.yml, runbooks | HIGH | KEEP | compose config scripts |
| `deployments/prod/legacy/` | Single-host rollback reference | Makefile prod-* targets | MEDIUM | KEEP | README only |
| `deployments/prod/monitoring/`, `prometheus/`, `grafana/` | Path pointers → observability | Internal docs | LOW | KEEP | — |
| `deployments/staging/`, `deployments/docker/` | Staging + local dev | CI, Makefile | HIGH | KEEP | `docker compose config` |
| `scripts/ci/`, `scripts/deploy/`, `scripts/db/` | CI gates, release, DB guards | Workflows, Makefile | HIGH | KEEP | ci-gates |

---

## 6. Tests and e2e — KEEP

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| `internal/e2e/correctness/` | Integration scenarios | make test-e2e-local | HIGH | KEEP | `go test ./internal/e2e/...` |
| `tests/e2e/` | Shell E2E harness | verify_e2e_assets.sh, CI | HIGH | KEEP | verify-e2e-assets |
| `testdata/` | JSON fixtures | Unit/integration tests | MEDIUM | KEEP | `go test ./...` |
| `scripts/e2e/`, `scripts/test/` | E2E helpers, coverage | Docs, local dev | MEDIUM | KEEP | bash -n |

---

## 7. Docs/runbooks — KEEP + selective ARCHIVE

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| `docs/runbooks/` | Operator procedures | README, production docs | HIGH | KEEP | link check |
| `docs/api/`, `docs/architecture/` | Contracts, design | CI contract checks | HIGH | KEEP | — |
| `docs/deployment/` (11 files) | Env, secrets, release | Runbooks | HIGH | KEEP | — |
| `docs/cicd/` (3 files) | Enterprise CI contract | README | MEDIUM | KEEP | — |
| `docs/operations/` (6 files) | Day-2 ops | docs/README | LOW | KEEP | — |
| `docs/production/` | Go-live checklists | Field ops | HIGH | KEEP | — |
| `docs/testing/` | Test guides | E2E scripts | MEDIUM | KEEP | — |
| `docs/audits/` (6 active files) | Current readiness | docs/README canonical | MEDIUM | KEEP | — |
| `docs/reports/cleanup/` | This cleanup pass | docs/README | LOW | KEEP + extend | — |

---

## 8. Archive/report candidates

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| `docs/archive/` (46 files) | Historical audits/reports | docs/README | LOW | KEEP | — |
| `docs/reports/cash-authority/` (5 files) | Completed cash investigation | Self-references only | LOW | **ARCHIVE** → `docs/archive/reports/cash-authority/` | `rg docs/reports/cash-authority` |
| `docs/reports/protocol-hardening/` (3 files) | Phase-complete remediation | Self-references | LOW | **ARCHIVE** → `docs/archive/reports/protocol-hardening/` | `rg docs/reports/protocol-hardening` |
| `docs/reports/production-deploy/` | Active failure analysis | docs/reports/README | MEDIUM | KEEP | — |
| `docs/reports/verification/` | Ongoing verification | Testing docs | MEDIUM | KEEP (non-FINAL) | — |
| `docs/reports/product-media-offline-cache/` | Phase reports | Runbook | LOW | KEEP | — |

---

## 9. Junk / artifact candidates

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| `_deploy_artifacts/` (7 files) | Local Security Release snapshot | **None** outside folder | LOW | **DELETE** + gitignore | `rg _deploy_artifacts` |
| `prod-deploy-inputs*.json` (if tracked) | Local deploy inputs | .gitignore pattern | LOW | Already gitignored | — |

---

## 10. Root-level and Postman cleanup candidates

| Path | Purpose | Referenced by | Risk | Action | Verify |
|------|---------|---------------|------|--------|--------|
| Root configs | Tool entrypoints | All tooling | HIGH | KEEP | — |
| `postman/environments/` | CI Postman env files | make postman-check | HIGH | **RESTORE** from HEAD~1 | `make postman-check` |
| `postman/production/` | E2E manifest generator | verify_production_postman_parity.sh | HIGH | **RESTORE** from HEAD~1 | parity script |
| `postman/scripts/` | Collection JS | build_postman_collection.py | MEDIUM | **RESTORE** from HEAD~1 | postman-check |
| `postman/avf-vending-production.full.*` | Consolidated full suite | None yet | MEDIUM | **MOVE** → `postman/suites/production-full/` | `rg avf-vending-production.full` |
| `postman/collections/` | OpenAPI-generated CI collections | E2E, make postman-check | HIGH | KEEP | postman-check |
| `scripts/repair/` (2 PS1) | Field repair scripts | test-metadata-contract.ps1 | MEDIUM | KEEP; document in scripts/README | — |
| `scripts/governance/` (2 sh) | Prod protection toggles | Ops runbooks | MEDIUM | KEEP; document in scripts/README | — |
| Root `scripts/*.sh` wrappers | Back-compat entrypoints | verify_workflow_contracts.sh | HIGH | KEEP | workflow contracts |

---

## Workflow classification (all KEEP)

| Workflow | Class |
|----------|-------|
| `deploy-prod.yml` | Canonical production deploy |
| `deploy-production.yml` | Legacy pointer stub |
| `deploy-develop.yml` | Canonical staging |
| `_reusable-build.yml`, `_reusable-deploy.yml` | Reusable |
| `ci.yml`, `build-push.yml`, `security-release.yml` | Active chain |
| `security.yml`, `codeql.yml`, `nightly-security.yml` | Security scans |
| `nightly-ops.yml`, `production-backup-evidence.yml`, `restore-drill.yml` | Scheduled ops |
| `rollback-prod.yml` | Manual rollback |
| `enterprise-release-verify.yml`, `environment-separation-gates.yml`, `production-proof.yml` | Manual gates |
| `production-e2e-automation-window.yml`, `telemetry-storm-staging.yml` | Staging/ops automation |

---

## Deferred (requires separate PR / owner confirmation)

| Item | Reason |
|------|--------|
| `internal/modules/postgres` → adapters split | 58 importers; P2 PR 4 |
| `httpserver`/`grpcserver` package split | High blast radius |
| `scripts/postman/generate_production_full_suite.py` OUT_DIR alignment | Generator outputs to gitignored `production-full-suite/` |
| Bulk archive of `docs/reports/verification/*` | Needs per-file supersession review |
