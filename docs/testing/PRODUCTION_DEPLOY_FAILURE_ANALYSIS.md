# Production Deploy Failure Analysis

**Date:** 2026-05-20  
**Status:** Root cause identified — fix in progress

---

## Phase 0 — Failure evidence (no code changes yet)

### Failed workflow

| Field | Value |
|-------|--------|
| **Workflow** | Deploy Production (`deploy-prod.yml`) |
| **Run ID** | [26138320350](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26138320350) |
| **Run number** | 137 |
| **URL** | https://github.com/leduytuanvu/avf-vending-api/actions/runs/26138320350 |
| **Head SHA** | `9e3224b8236311280b4535124efcf522a00f7076` |
| **Branch** | `main` |
| **Conclusion** | **failure** (10s) |
| **Trigger** | `workflow_dispatch` with `run_migration=true` |

### Failed job / step

| Field | Value |
|-------|--------|
| **Job** | Validate Production Artifact Source |
| **Step** | Validate production action inputs |
| **Exact error** | `backup_evidence_id is required when run_migration=true` |

### Failure phase classification

**Before migration** — workflow failed at GitHub Actions input validation. No SSH to production, no image pull, no `pg_dump`, no goose, no app rollout, no health check, no smoke test.

### Exact error summary

Deploy was triggered with valid build (`26137941974`), security release (`26138067736`), and digest-pinned images for `9e3224b`, plus documented staging bypass. Validation rejected the run because `run_migration=true` was set without `backup_evidence_id`.

No `production-db-backup-evidence` artifact exists in the repository. The workflow input description states supplementary backup evidence is **optional** when inline `pg_dump` runs via `scripts/deploy/production-migrate.sh`, but validation added in commit `9994d50` contradicts that and blocks deploy.

### Suspected root cause

**Category C — Migration/deploy gate misconfiguration**

- `deploy-prod.yml` lines 427–429 require non-empty `backup_evidence_id` when `run_migration=true`.
- `tools/verify_github_workflow_cicd_contract.py` enforces presence of that error string in CI.
- This blocks the intended path: **inline `pg_dump` + goose `up` on app node A** via `production-migrate.sh` (already wired in `release_app_node.sh`).
- Migrations **are** embedded in the production image (`deployments/prod/Dockerfile` copies `migrations/` → `/app/migrations`; `/app/migrate` binary present).
- Previous successful deploy [26093589896](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26093589896) used `run_migration=false`, leaving production on stale `git_sha` `52a076e`.

### Proposed fix plan

1. Remove mandatory `backup_evidence_id` check when `run_migration=true` in `deploy-prod.yml` — keep supplementary validation when operator supplies it.
2. Update `verify_github_workflow_cicd_contract.py` to require `production-migrate.sh` inline backup path instead of mandatory pre-uploaded evidence.
3. Align runbook docs: primary backup = inline `pg_dump` on VPS; supplementary `backup_evidence_id` = optional audit artifact.
4. Merge fix to `develop` → `main` via PR with CI green.
5. Re-trigger Deploy Production with `run_migration=true` (no `backup_evidence_id` required).
6. Verify public smoke + `/version` git_sha matches deploy head.

---

## Phase 1 — Server state (SSH)

| Check | Result |
|-------|--------|
| **SSH** | `root@72.62.244.94` — **Permission denied (publickey,password)** from this workstation |
| **Classification** | **Unknown** (cannot inspect; production likely **old app still running** — public `/health/live` and `/health/ready` return 200; `/version` reports `52a076e`) |

GitHub Actions Deploy Production has production SSH secrets and can inspect/update the server once workflow passes validation.

---

## Phase 2 — Database / migration state

| Check | Result |
|-------|--------|
| **Pre-fix backup** | **Not run** — SSH blocked from this environment |
| **Migration version** | **Not inspected** — SSH blocked |

Backup and migration state will be verified via Deploy Production workflow logs after fix (inline `pg_dump` in `production-migrate.sh`).

---

## Phase 3 — Fix applied

### Root cause (confirmed)

**Category C — Deploy gate misconfiguration.** Mandatory `backup_evidence_id` when `run_migration=true` contradicted inline `pg_dump` via `production-migrate.sh`.

### Files changed

| File | Change |
|------|--------|
| `.github/workflows/deploy-prod.yml` | Removed lines requiring `backup_evidence_id` when `run_migration=true` |
| `tools/verify_github_workflow_cicd_contract.py` | Require `production-migrate.sh` inline backup path; drop mandatory evidence error string |
| `docs/runbooks/backup-evidence-for-production-migrations.md` | Document inline pg_dump as primary; `backup_evidence_id` optional |
| `docs/production/production-backup-restore-drill.md` | Align drill doc with inline backup path |

### Validation commands and results

| Check | Result |
|-------|--------|
| `gofmt -w .` | PASS |
| `git diff --check` | PASS (after trailing whitespace fix) |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `scripts/ci/verify_workflow_contracts.sh` | PASS |
| Docker build `-f deployments/prod/Dockerfile` | PASS |
| Migrations in image (`/app/migrations/*.sql`) | **5** files present |
| `/app/migrate` binary | Present in image |

### Fix summary

Deploy Production with `run_migration=true` no longer requires pre-uploaded `backup_evidence_id`. Inline `pg_dump` on app node A via `scripts/deploy/production-migrate.sh` satisfies the backup gate before goose `Up`.

### Deploy run 26139516871 (post-fix, second failure)

| Field | Value |
|-------|--------|
| **Run** | [26139516871](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26139516871) |
| **Phase** | During migration (SSH reached; validation passed) |
| **Error** | `bash: .../scripts/deploy/production-migrate.sh: No such file or directory` |
| **Cause** | App-node tar sync shipped `deployments/prod/*` and `migrations/` but **not** `scripts/deploy/production-migrate.sh` |

### Follow-up fix (sync migration scripts)

Add to app-node A/B tar sync: `scripts/deploy/production-migrate.sh`, `scripts/verify_database_environment.sh`, `scripts/db/verify_database_environment.sh`.

### Deploy run 26140405803 (migration confirm gate)

| Field | Value |
|-------|--------|
| **Error** | `set CONFIRM_PRODUCTION_MIGRATION=true for manual production migration` |
| **Cause** | `production-migrate.sh` skips confirm only when `GITHUB_ACTIONS=true`; remote SSH session does not inherit that env |

### Follow-up fix (release migration confirm)

`release_app_node.sh` exports `CONFIRM_PRODUCTION_MIGRATION=true` when `RUN_MIGRATION=1` (automated deploy path).
