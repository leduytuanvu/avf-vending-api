# 2026-07-02 repository cleanup final report

**Branch:** `cleanup/repo-structure-and-junk-files`  
**Baseline commit:** `c69a5995df3b7fc2298da32504cee56549824213`  
**Date:** 2026-07-02

Related: [baseline](2026-07-02-repo-cleanup-baseline.md) · [inventory](2026-07-02-repo-cleanup-inventory.md) · [plan](2026-07-02-repo-cleanup-plan.md)

---

## 1. Summary of changes

This cleanup restored CI-critical Postman paths broken by c69a5995, removed committed local Security Release deploy artifacts, relocated the consolidated production full Postman suite to a dedicated subdirectory, archived completed investigation reports, and updated documentation indexes. No migrations, workflows, deployment scripts, or generated Go/sqlc/protobuf code were modified.

---

## 2. Files/folders deleted

| Path | Reason |
|------|--------|
| `_deploy_artifacts/` (7 files) | Local `production-deploy-candidate` snapshot; zero repo references; CI generates dynamically |

Added to [`.gitignore`](../../.gitignore): `_deploy_artifacts/`, `prod-deploy-candidate/`, `production-deploy-candidate/`.

---

## 3. Files/folders moved

| From | To |
|------|-----|
| `postman/avf-vending-production.full.postman_collection.json` | `postman/suites/production-full/` |
| `postman/avf-vending-production.full.postman_environment.json` | `postman/suites/production-full/` |
| `docs/reports/cash-authority/*` (5 reports) | `docs/archive/reports/cash-authority/` |
| `docs/reports/protocol-hardening/*` (3 files) | `docs/archive/reports/protocol-hardening/` |

---

## 4. Files/folders restored

| Path | Source | Reason |
|------|--------|--------|
| `postman/environments/` (3 JSON) | `HEAD~1` | Required by `make postman-check` |
| `postman/production/` (generator + E2E collection/env) | `HEAD~1` | Required by `scripts/ci/verify_production_postman_parity.sh` |
| `postman/scripts/` (2 JS) | `HEAD~1` | Postman collection scripts |

---

## 5. Files/folders intentionally kept

- All `migrations/` (15 files), `db/queries/`, `db/schema/`
- `internal/gen/` — no manual edits
- All 20 `.github/workflows/*.yml` including `deploy-prod.yml` and `deploy-production.yml` pointer
- `deployments/prod/**` including `legacy/`, `monitoring/`, `prometheus/`, `grafana/` pointers
- `scripts/repair/`, `scripts/governance/`, root `scripts/*.sh` wrappers
- `docs/runbooks/**`, active `docs/audits/**`, `docs/reports/production-deploy/`, `docs/reports/verification/`

---

## 6. Documentation and config updates

| File | Change |
|------|--------|
| `postman/README.md` | Document `suites/production-full/` and restored CI paths |
| `docs/README.md` | Link 2026-07 cleanup reports |
| `docs/reports/README.md` | Cleanup report index + archive pointers |
| `docs/archive/README.md` | Note archived cash-authority and protocol-hardening |
| `docs/archive/reports/cash-authority/README.md` | New archive stub |
| `docs/archive/reports/protocol-hardening/README.md` | New archive stub |
| `scripts/README.md` | Added `repair/`, `governance/`, `e2e/` rows |
| `.gitignore` | Ignore local deploy artifact dirs |
| `.gitattributes` | LF for `postman/suites/**/*.json` |
| `repomix.config.json` | Ignore `_deploy_artifacts/**` |

---

## 7. Risky files not touched

- P2 deferred: `internal/modules/postgres` path move (58 importers)
- Transport package splits (`internal/httpserver`, `internal/grpcserver`)
- All GitHub workflow YAML content
- Legacy production compose under `deployments/prod/docker-compose.prod.yml`
- `scripts/postman/generate_production_full_suite.py` OUT_DIR (still `postman/production-full-suite/`, gitignored)

---

## 8. Commands run and results

| Command | Result | Notes |
|---------|--------|-------|
| `go test ./... -short` | **PASS** | exit 0 (before and after) |
| `go vet ./...` | **PASS** | exit 0 |
| `python postman/production/generate_postman_from_manifest.py` | **PASS** | 48 REST requests generated |
| `git diff --ignore-cr-at-eol -- postman/production/` | **PASS** | no drift after regen |
| `python tests/e2e/production/scripts/validate_postman_shell_parity.py` | **PASS** | `POSTMAN_PARITY_OK` |
| `python tools/check_postman_artifacts.py` | **PASS** | `OK: Postman artifact checks` |
| `bash scripts/ci/verify_production_postman_parity.sh` | **PASS** | Git Bash; `POSTMAN_PARITY_CI_OK` |
| `bash scripts/ci/verify_migrations.sh` | **PASS** | 15 files, 0 findings |
| `bash scripts/ci/verify_workflow_contracts.sh` | **PASS** | Git Bash; workflow contract checks passed |
| `make postman-check` | **PASS (manual)** | Regen + `check_postman_artifacts.py` + clean diff on collections/environments (PowerShell `python`) |
| `make api-contract-check` | **Deferred** | Requires buf/sqlc pin + full make chain |
| `make verify-enterprise-release` | **Deferred** | Requires full bash make chain |
| `rg _deploy_artifacts` | **PASS** | Only `.gitignore`, `repomix.config.json`, cleanup docs |

**Tracked file count:** 1796 → 1803 (net +7: cleanup reports + archive READMEs; −7 deploy artifacts; + restored Postman paths).

---

## 9. Remaining cleanup backlog

1. Run `make api-contract-check` and `make verify-enterprise-release` in CI or a full Git Bash environment with `make` on PATH before merge.
2. Align `scripts/postman/generate_production_full_suite.py` OUT_DIR with `postman/suites/production-full/` (owner decision).
3. Optional bulk archive of superseded `docs/reports/verification/*` files after per-file review.
4. Update stale headers inside archived cash-authority reports (still mention old `docs/reports/cash-authority/` path).

---

## 10. Safety confirmations

| Check | Status |
|-------|--------|
| Build/tests pass | **Yes** — `go test ./... -short`, `go vet ./...` |
| Production workflow protected | **Yes** — no workflow edits |
| `deploy-prod.yml` intact | **Yes** |
| `deploy-production.yml` pointer intact | **Yes** |
| Migrations intact | **Yes** — 15 files unchanged |
| Generated code policy consistent | **Yes** — no manual edits to `internal/gen/` or `.pb.go` |
| No secrets introduced | **Yes** |
| No production deploy performed | **Yes** |
| No production DB mutation performed | **Yes** |
| No obvious committed local artifacts | **Yes** — `_deploy_artifacts/` removed and gitignored |

---

## 11. Rollback

```bash
git checkout chore/sync-local-main-20260702
git branch -D cleanup/repo-structure-and-junk-files
```

Per-path restore documented in [plan](2026-07-02-repo-cleanup-plan.md).
