# Repository junk & Markdown cleanup audit (2026-05-20)

**Branch:** `chore/cleanup-junk-docs` (from `develop` @ post–PR #244)  
**Scope:** Read-only inventory before deletion. Production deploy assets untouched.

---

## Markdown inventory (281 tracked `.md` files)

| Classification | Count (approx.) | Examples / notes |
|----------------|-----------------|------------------|
| **KEEP** | ~240 | `README.md`, `docs/architecture/*`, `docs/runbooks/*`, `docs/api/*`, `docs/testing/*` guides, `deployments/**/README.md` |
| **PRODUCTION CRITICAL** | 15+ | `docs/production/*`, `docs/deployment/*`, `docs/runbooks/production-*`, `docs/runbooks/deploy-failure.md`, `docs/runbooks/migration-safety.md` |
| **TESTING CRITICAL** | 20+ | `docs/testing/*`, `tests/e2e/README.md`, `postman/suites/full-production-suite/*.md` |
| **GENERATED REPORT** | 50+ | `docs/reports/**`, `docs/audits/*_REPORT.md`, `docs/cicd/CI_CD_FINAL_AUDIT.md` |
| **MOVE (stub — CI contract)** | 7 | `docs/operations/*.md` — redirect stubs; **workflows + `verify_workflow_contracts.sh` grep these paths** |
| **MERGE (redirect stub)** | 1 | `docs/api/kiosk-implementation-payloads.md` → canonical `docs/api/examples/kiosk-implementation-payloads.md` |
| **NEEDS REVIEW** | 1 | `docs/api/device-offline-replay-samples.md` — unique gRPC offline-sync content; **zero inbound links** (MQTT samples live under `examples/`) |
| **DELETE CANDIDATE** | 5 | Empty/stale grep artifacts under `docs/reports/final-gate-audit/` and `final-single-scope-audit/` |

### `docs/operations/` stubs — DO NOT DELETE this PR

CI enforces presence and workflow comments reference:

- `docs/operations/production-smoke-tests.md`
- `docs/operations/production-backup-restore-drill.md`
- `docs/operations/two-vps-rolling-production-deploy.md`
- `docs/operations/release-evidence-retention.md`
- `docs/operations/deploy-monitoring-slo.md`
- `docs/operations/github-governance.md`
- `docs/operations/staging-preprod-gate.md`

Evidence: `scripts/ci/verify_workflow_contracts.sh` lines 554–579; `.github/workflows/deploy-prod.yml`, `ci.yml`, `security-release.yml`.

Canonical content lives in `docs/deployment/`, `docs/production/`, or `docs/cicd/`. Removal deferred to P2 per `REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md`.

---

## Junk file inventory

### Local untracked (gitignored — not committed)

| Path | Type | Action |
|------|------|--------|
| `.tmp-phase7/`, `.tmp-phase8/`, `.tmp-phase9/` | Deploy pipeline scratch (JSON, logs, env) | Already in `.gitignore`; leave on disk or delete locally |
| `.tmp-image-metadata/` | Build metadata download | Already gitignored |
| `tmp/` | Empty local scratch dir | Gitignored via `tmp/` |

### Tracked junk candidates

| Path | Type | Size | References |
|------|------|------|------------|
| `docs/reports/final-gate-audit/all-hits-before.txt` | Stale grep output | 0 B | None |
| `docs/reports/final-gate-audit/all-hits-final.txt` | Stale grep output | 0 B | None |
| `docs/reports/final-gate-audit/path-hits-final.txt` | Stale grep output | 0 B | None |
| `docs/reports/final-gate-audit/path-hits-before.txt` | Sanitized placeholder | 126 B | None (superseded by `classification.md`) |
| `docs/reports/final-single-scope-audit/final-zero-hit-grep.txt` | Stale grep output | 0 B | None |

### Tracked non-junk zero-byte files

| Path | Reason to keep |
|------|----------------|
| `deployments/docker/observability/grafana/provisioning/dashboards/json/.gitkeep` | Intentional placeholder |

### Secret scan

No real secrets in tracked files. Matches are **placeholders** in `.env*.example`, Makefile local defaults, and workflow secret **names** only.

---

## Delete candidates

| Path | Reason | Evidence | Risk | Decision |
|------|--------|----------|------|----------|
| `docs/reports/final-gate-audit/all-hits-before.txt` | Empty stale artifact | `git grep` → 0 refs | Low | **DELETE** |
| `docs/reports/final-gate-audit/all-hits-final.txt` | Empty stale artifact | 0 refs | Low | **DELETE** |
| `docs/reports/final-gate-audit/path-hits-final.txt` | Empty stale artifact | 0 refs | Low | **DELETE** |
| `docs/reports/final-gate-audit/path-hits-before.txt` | Placeholder superseded by `classification.md` | 0 refs | Low | **DELETE** |
| `docs/reports/final-single-scope-audit/final-zero-hit-grep.txt` | Empty stale artifact | 0 refs | Low | **DELETE** |

**Not deleting:** `docs/api/device-offline-replay-samples.md` (unique gRPC content, ambiguous name only).

---

## Files/folders that must not be touched

- `migrations/**`, `db/schema/**`
- `deployments/prod/**`, `deployments/docker/**` (compose, observability)
- `.github/workflows/**`
- `Dockerfile*`, `docker-compose*`
- `scripts/ci/validate-production-deploy.sh`, `scripts/ci/verify_workflow_contracts.sh`, deploy/release scripts
- `postman/collections/*.json`, `postman/environments/*.json`, suite generators
- `tests/**`, `testdata/**`
- `docs/operations/**` CI stubs
- Production runbooks, security docs, deploy/migration docs
- Prior cleanup audits (`REPO_CLEANUP_*`, `REPO_STRUCTURE_*`)

---

## `.gitignore` gaps (Phase 3)

Add if missing: `.DS_Store`, `*.coverage`, `temp/`, `db-backups/`, `deployment-evidence/local/`, `id_rsa`, `*.env.local`, top-level `newman/`, `reports/newman/`.

Already present: `.env`, `.env.*.local`, `*.dump`, `*.bak`, `*.tmp`, `*.old`, `*.orig`, `*.log`, `Thumbs.db`, `coverage.out`, `.e2e-runs/`, `tmp/`, `.tmp-phase*/`.

---

## Conclusion

Safe to delete **5 stale grep artifact files** and harden `.gitignore`. No Markdown deletions beyond those artifacts; no moves required beyond audit/report documentation for this pass.
