# Production Deploy With Database Migration Report

**Date:** 2026-05-20  
**Operator phase:** production deploy from `main` @ `9e3224b`  
**Verdict:** **DEPLOY_BLOCKED**

---

## 1. Main SHA deployed

| Field | Value |
|-------|--------|
| **Target SHA** | `9e3224b8236311280b4535124efcf522a00f7076` |
| **Branch** | `main` (Merge PR #230 — develop → main) |
| **Currently live** | `52a076e340a15a69dad7787cad54d7e3000fcafe` (per `/version`) — **not updated** |

---

## 2. Phase 0 — Pre-deploy validation

| Check | Result |
|-------|--------|
| `git checkout main` + `git pull --ff-only origin main` | **PASS** @ `9e3224b` |
| `gofmt -w .` | **PASS** |
| `git diff --check` | **PASS** |
| `go test ./...` | **PASS** |
| `go vet ./...` | **PASS** |
| `go list ./...` | **PASS** |
| `verify-uuid-v7.sh` | **PASS** |
| `check-uuid-v7.sh` | **PASS** |
| Migrations present | **5** files in `migrations/*.sql` |
| Destructive SQL scan (`DROP DATABASE`, `DROP SCHEMA`, `TRUNCATE`) | **None** in migrations |
| JSON validation | **PASS** |

Pilot production note: no real user data; destructive migrations would be acceptable if present, but none detected.

---

## 3. Phase 1 — Production deploy workflow resolution

| Item | Value |
|------|--------|
| **Build workflow** | Build and Push Images (`build-push.yml`) |
| **Security workflow** | Security Release |
| **Deploy workflow** | Deploy Production (`deploy-prod.yml`) — **manual `workflow_dispatch` only** |
| **Build run ID** | `26137941974` — **SUCCESS** @ `9e3224b` |
| **Security release run ID** | `26138067736` — **SUCCESS** @ `9e3224b` |
| **App image ref** | `ghcr.io/leduytuanvu/avf-vending-api@sha256:23a3f0f430cf742e95f0d662b92e2b0431d632387e07481156eb653fdd75283a` |
| **Goose image ref** | `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:5f1cabae3790bb04ba6ddd6d5756988cc147853fb9c4850b944c0b7aed399e69` |
| **Staging evidence** | No successful Staging Deployment Contract run for current digests; bypass documented below |

**Resolved from artifacts:** `immutable-image-contract.json`, `promotion-manifest.json`, `security-verdict.json` (build run `26137941974`, security run `26138067736`).

**Staging gate bypass (documented):** `allow_missing_staging_evidence=true` with reason: *Pilot production post PR230 recovery merge; staging contract not green; inline pg_dump via production-migrate.sh*.

---

## 4. Phase 2 — Server-side DB backup (pre-deploy SSH)

| Field | Result |
|-------|--------|
| **Target** | `root@72.62.244.94` |
| **Path** | `/opt/avf-vending-api/deployments/prod/app-node` |
| **Result** | **FAILED** — `Permission denied (publickey,password)` |
| **Backup path on server** | **Not created** |

Cannot run pre-deploy `pg_dump` from this environment. The Deploy Production workflow uses GitHub-hosted SSH secrets and would perform inline backup via `scripts/deploy/production-migrate.sh` **if** the workflow passed input validation.

---

## 5. Phase 3 — Production deploy trigger

| Field | Value |
|-------|--------|
| **Workflow** | Deploy Production |
| **Run ID** | [26138320350](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26138320350) |
| **Trigger** | `workflow_dispatch` on `main` |
| **Inputs** | `run_migration=true`, digest-pinned images, build `26137941974`, security `26138067736`, staging bypass |
| **Conclusion** | **FAILURE** (3s) |
| **Failed job** | Validate Production Artifact Source |
| **Root error** | `backup_evidence_id is required when run_migration=true` |

### Why migration deploy did not start

The workflow enforces supplementary **backup evidence** before SSH when `run_migration=true` (see `docs/production/production-backup-restore-drill.md`):

- `backup_evidence_id` must be a **numeric GitHub run id** with artifact `production-db-backup-evidence` containing `backup-evidence/backup-evidence.json`, **or**
- `path:relative/backup-evidence.json` in the repo checkout.

**No `production-db-backup-evidence` artifact exists** in this repository (search returned empty). No committed `backup-evidence.json` path is available.

Previous successful deploy [26093589896](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26093589896) used **`run_migration=false`** (image-only), which explains live `/version` drift without schema migration.

**No duplicate deploy triggered after failure.**

---

## 6. Phase 4 — Manual server migration fallback

**Not attempted** — same SSH blocker as Phase 2. Fallback requires `root@72.62.244.94` access to run migrations from `/app/migrations` in the deployed image.

---

## 7. Phase 5 — Post-deploy health (unchanged production)

| Check | Result |
|-------|--------|
| `/health/live` | **200** |
| `/health/ready` | **200** |
| `/version` | `git_sha: 52a076e340a15a69dad7787cad54d7e3000fcafe`, `version: v1.0.01`, `app_env: production` |

Production remains on pre-PR#230 image/commit. Services healthy; version not updated to `9e3224b`.

**Container check on server:** Not run (SSH blocked).

---

## 8. Phase 6 — Admin login smoke

**Skipped** — no production deploy completed; credentials not exercised.

---

## 9. Migration summary

| Field | Value |
|-------|--------|
| **Method planned** | Workflow auto-migration (`run_migration=true` → `production-migrate.sh` on app node A) |
| **Method executed** | **None** |
| **Destructive migration detected** | **No** |
| **Inline pg_dump on VPS** | **Not reached** (workflow failed pre-SSH) |

---

## 10. Blockers and recovery steps

| # | Blocker | Required action |
|---|---------|-----------------|
| 1 | Missing `backup_evidence_id` for `run_migration=true` | Create real production backup; upload `production-db-backup-evidence` artifact with valid `backup-evidence.json`, **or** commit validated JSON and use `path:…` |
| 2 | No SSH from this workstation | Use GitHub Actions Deploy Production (has prod SSH secrets) **after** blocker #1 is resolved |
| 3 | Staging contract not green | Provide valid `staging_evidence_id` **or** keep documented bypass |
| 4 | Live `/version` still `52a076e` | Successful deploy with `run_migration=true` (or image-only + separate migration) needed |

### Recommended operator sequence

1. On production app-node (via operator SSH): run `backup_postgres.sh` / `production-migrate.sh` dry-run OR take managed DB snapshot.
2. Build `backup-evidence.json` per `docs/contracts/backup-evidence.schema.json`; validate with `scripts/ci/validate_backup_evidence.py --for-production-migration`.
3. Upload artifact `production-db-backup-evidence` via a workflow run; note run id.
4. Re-run **Deploy Production** with:
   - `run_migration=true`
   - `backup_evidence_id=<evidence run id>`
   - Same build/security/image inputs as above (or fresh build after any new commits)
   - `deploy_production_confirmation=DEPLOY_PRODUCTION`
5. Confirm `/version` reports `git_sha` matching deploy head SHA.

---

## 11. Final verdict

**DEPLOY_BLOCKED**

Local validation and CI build/security gates for `9e3224b` are green. Production deploy workflow **26138320350 failed input validation** before backup, migration, or rollout. Production remains healthy on stale commit `52a076e`.

---

*No secrets printed. No production DB manually edited. No CI bypass beyond documented staging evidence bypass (deploy did not proceed).*
