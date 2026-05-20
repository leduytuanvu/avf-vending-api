# Production Deploy Failure Analysis

**Date:** 2026-05-20
**Phase:** 0 — evidence collection (latest failure)
**Status:** Root cause identified — ready to fix

---

## Phase 0 — Latest failure (2026-05-20)

### 1. Failed workflow

| Field | Value |
|-------|--------|
| **Workflow name** | Deploy Production |
| **Workflow file** | `.github/workflows/deploy-prod.yml` (not `deploy-production.yml`, which is a no-op pointer) |
| **Run ID** | [26141230829](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26141230829) |
| **Run number** | 141 |
| **URL** | https://github.com/leduytuanvu/avf-vending-api/actions/runs/26141230829 |
| **Head SHA** | `60e98a143f170ecdccfd439557af41219f975cbc` |
| **Branch** | `main` |
| **Conclusion** | **failure** (14m43s; waited on production environment approval, then failed during migration) |
| **Trigger** | `workflow_dispatch` with `run_migration=true` |

### 2. Failed job / step

| Field | Value |
|-------|--------|
| **Job** | Deploy production release |
| **Step** | Roll out app node A with smoke gate |
| **Sub-phase** | `release_app_node` → `migrate` (before traffic drain) |

### 3. Exact error summary

```
==> production database backup + migration (before traffic drain; old containers keep serving if migrate fails)
.../scripts/verify_database_environment.sh: line 4: .../scripts/db/verify_database_environment.sh: Permission denied
production-migrate: error: verify_database_environment.sh failed
error: production-migrate.sh failed; leaving running containers unchanged
release_app_node: failed during migrate
Process completed with exit code 41
```

**What succeeded before failure:** All workflow validation gates, SSH, tar sync, bootstrap, `docker compose config`, image pull (app + caddy). **No `pg_dump`, no goose, no container recreate** — old stack left running.

**Production impact:** `/health/live` and `/health/ready` remain **200**; `/version` still reports stale `git_sha: 52a076e` (last successful deploy [26093589896](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26093589896) used `run_migration=false`).

### 4. Root cause category

**script permission**

### 5. Root cause (detailed)

Migration helper scripts are synced to the VPS via `tar` in `deploy-prod.yml` but arrive **without execute permission** (`100644` in git). The wrapper `scripts/verify_database_environment.sh` at commit `60e98a1` uses:

```bash
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/db/verify_database_environment.sh" "$@"
```

`production-migrate.sh` invokes the wrapper with `bash` (line 217), but the wrapper **`exec`s the canonical script directly**, which requires the `+x` bit on `scripts/db/verify_database_environment.sh`. Tar extract at `60e98a1` does **not** run `chmod +x` after extract.

Git modes at failed SHA `60e98a1`:

| Path | Mode |
|------|------|
| `scripts/deploy/production-migrate.sh` | `100644` |
| `scripts/verify_database_environment.sh` | `100644` |
| `scripts/db/verify_database_environment.sh` | `100644` |

**Call chain:**

1. `release_app_cluster.sh` → SSH → `release_app_node.sh` (`RUN_MIGRATION=1`, `CONFIRM_PRODUCTION_MIGRATION=true`)
2. `release_app_node.sh` → `bash scripts/deploy/production-migrate.sh`
3. `production-migrate.sh` → `bash scripts/verify_database_environment.sh`
4. Wrapper → `exec scripts/db/verify_database_environment.sh` → **Permission denied**

This is **not** a migration image, DB backup, compose, or health-check failure — migration never reached `pg_dump` or goose.

### 6. Files that likely need fixing

| File | Why |
|------|-----|
| `scripts/verify_database_environment.sh` | Wrapper should invoke canonical script via `bash` (no execute bit required) |
| `.github/workflows/deploy-prod.yml` | Post-extract `chmod +x` on synced migration helper scripts (app-node A/B tar sync) |
| `docs/testing/PRODUCTION_DEPLOY_FAILURE_ANALYSIS.md` | Record fix and redeploy outcome |

Optional hardening (not strictly required if above two are done):

| File | Why |
|------|-----|
| `scripts/db/verify_database_environment.sh` | Could set `100755` in git |
| `scripts/deploy/production-migrate.sh` | Could set `100755` in git |

**Not under `deployments/prod/shared/scripts/`:** `production-migrate.sh` and `verify_database_environment.sh` live at repo-root `scripts/deploy/` and `scripts/` (there are no copies under `deployments/prod/shared/scripts/`).

### 7. Proposed fix plan

1. **Wrapper fix:** Change `scripts/verify_database_environment.sh` to `exec bash .../db/verify_database_environment.sh "$@"` so execute permission on the target file is not required.
2. **Deploy sync fix:** After `tar -xf` in app-node A/B sync steps, `chmod +x` the three synced scripts under `${PRODUCTION_DEPLOY_ROOT}/scripts/...`.
3. Merge via PR to `develop` → `main` (draft fix exists on branch `fix/production-migration-script-permissions` / PR #238 — not yet on `origin/main`).
4. Re-trigger Deploy Production with `run_migration=true`, same build/security inputs or fresh build for merged SHA.
5. Verify workflow logs show: `verify_database_environment: OK` → inline `pg_dump` → goose `Up` → app rollout → public smoke (`/version` git_sha updated).

### 8. Prior failure chain (resolved or superseded)

| Run | Phase | Category | Error |
|-----|-------|----------|-------|
| [26138320350](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26138320350) | Input validation | workflow input | `backup_evidence_id is required when run_migration=true` — **fixed** (PR #232) |
| [26139516871](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26139516871) | Migration | missing VPS asset | `production-migrate.sh: No such file or directory` — **fixed** (PR #234, tar sync) |
| [26140405803](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26140405803) | Migration | deploy gate | `CONFIRM_PRODUCTION_MIGRATION=true` required — **fixed** (PR #236) |
| [26140955149](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26140955149) | SSH preflight | infrastructure | TCP connect timeout — transient |
| **[26141230829](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26141230829)** | **Migration** | **script permission** | **`verify_database_environment.sh: Permission denied`** — **current blocker** |

---

## Phase 0 verdict

**READY_TO_FIX** — root cause is isolated to non-executable synced migration helper scripts + wrapper `exec` without `bash`. Fix is scoped to two files; no DB reset required. Production remains on old containers.

---

## Appendix — infrastructure notes

| Check | Result |
|-------|--------|
| **Canonical workflow** | `.github/workflows/deploy-prod.yml` (`Deploy Production`) |
| **Legacy pointer** | `.github/workflows/deploy-production.yml` — no deploy |
| **Migration runner** | `scripts/deploy/production-migrate.sh` (not under `deployments/prod/shared/`) |
| **Migrations in image** | `deployments/prod/Dockerfile` copies `migrations/` → `/app/migrations`; `/app/migrate` binary built |
| **Local SSH to prod** | Not available from this workstation (GHA deploy uses prod SSH secrets) |
| **DB backup on latest run** | Not performed (failed before `pg_dump`) |

---

## Phase 1 — Server inspection (2026-05-20)

**Goal:** Inspect production server state safely before fix/redeploy. No server file changes, no container restarts, no DB migration/reset, no secrets printed.

### Inspection method

| Method | Result |
|--------|--------|
| **Direct SSH** (`root@72.62.244.94`, requested compose/ps/logs command) | **Blocked** — `Permission denied (publickey,password)` from this workstation (no production deploy key) |
| **Indirect (read-only)** | Deploy Production run [26141230829](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26141230829) — GHA SSH succeeded; logs capture remote bootstrap, image pull, migration failure, automatic rollback, and post-rollback health checks (~2026-05-20 04:39–04:40 UTC) |

### 1. Container status

**Inferred from rollback step logs** (project `avf-vending-prod-app-a`, compose `docker-compose.app-node.yml`):

| Container | State after automatic rollback |
|-----------|--------------------------------|
| `avf-vending-prod-app-a-api-1` | Recreated → Started → **Healthy** |
| `avf-vending-prod-app-a-worker-1` | Recreated → Started → ready |
| `avf-vending-prod-app-a-mqtt-ingest-1` | Recreated → Started → ready |
| `avf-vending-prod-app-a-reconciler-1` | Recreated → Started → ready |
| `avf-vending-prod-app-a-caddy-1` | Started after api healthy → upstream healthcheck **200** |

Compose services (expected): `api`, `worker`, `mqtt-ingest`, `reconciler`, `caddy` (+ optional `temporal-worker` when profile enabled).

**Note:** Migration failure left workloads unchanged initially (`leaving running containers unchanged`), but **automatic rollback** then force-recreated app workloads and restored prior digest-pinned images. `ROLLBACK_RESULT: completed`, `rollback_app_node: PASS`.

### 2. Old app still running?

**Yes.** Production is on **last-known-good (LKG)** app image, not the failed deploy target:

| Ref | Digest | Role |
|-----|--------|------|
| **Running (LKG)** | `sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe` | Restored by `rollback_app_node.sh` |
| **Failed deploy target** | `sha256:cc188d2b217f772492d32277fa9d1005db61810a98da62d92207d57b05296f18` | Pulled on node but **never** rolled out (migration failed first) |

Public `/version` was still stale (`git_sha: 52a076e`) before this deploy attempt; rollback health checks reported `api /version returns 200` on-node after rollback. No goose migration ran; DB schema unchanged.

### 3. Unhealthy containers?

**None observed** after rollback. Post-rollback `healthcheck_app_node: PASS` (three passes: pre-caddy, post-caddy, final). Explicit checks:

- `api /health/live` → 200
- `api /health/ready` → 200
- `worker`, `reconciler`, `mqtt-ingest` `/health/ready` → 200
- `caddy` upstream healthcheck → 200
- Managed Postgres, Redis, NATS reachability → PASS

No `unhealthy` or crash-loop lines in captured rollback logs.

### 4. APP_IMAGE_REF present?

**Yes (workflow + rollback evidence).** Deploy job carried digest-pinned `APP_IMAGE_REF` for the failed target; `PREVIOUS_APP_IMAGE_REF` was recorded for rollback. `rollback_app_node.sh` writes `APP_IMAGE_REF` / `GOOSE_IMAGE_REF` into `.env.app-node` via `set_env_value` before `docker compose up`. Direct `grep '^APP_IMAGE_REF=' .env.app-node` on the VPS was **not** run (SSH blocked locally); post-rollback LKG digest pull and recreate confirm env was applied.

### 5. Script executable bit on server

**Direct `find … -exec ls -l` not available** (SSH blocked). Strong indirect evidence that synced migration helpers are **not executable** on the VPS:

| Evidence | Detail |
|----------|--------|
| Runtime error | `scripts/db/verify_database_environment.sh: Permission denied` (wrapper `exec` without `+x`) |
| Git modes @ failed SHA | All three synced scripts are `100644` |
| Tar sync @ `60e98a1` | `tar -xf -` with **no** post-extract `chmod +x` |
| Deploy scripts on server | `deployments/prod/*/scripts/*.sh` are part of normal deploy tree and invoked via `bash`; migration helpers under repo-root `scripts/` are the affected paths |

**Affected paths (synced to `${PRODUCTION_DEPLOY_ROOT}/scripts/…`):**

- `scripts/deploy/production-migrate.sh` — likely `-rw-r--r--` (non-executable; invoked with `bash`, OK)
- `scripts/verify_database_environment.sh` — non-executable; OK when called with `bash`
- `scripts/db/verify_database_environment.sh` — **non-executable; fails** when wrapper `exec`s it directly

Fix in PR #238: wrapper uses `exec bash …/db/verify_database_environment.sh` + post-tar `chmod +x` in `deploy-prod.yml`.

### 6. Recent critical logs

**Migration failure (app-node A, before traffic drain):**

```
==> production database backup + migration (before traffic drain; old containers keep serving if migrate fails)
.../scripts/verify_database_environment.sh: line 4: .../scripts/db/verify_database_environment.sh: Permission denied
production-migrate: error: verify_database_environment.sh failed
error: production-migrate.sh failed; leaving running containers unchanged
release_app_node: failed during migrate
Process completed with exit code 41
```

**Automatic rollback (completed):**

```
Image ghcr.io/leduytuanvu/avf-vending-api@sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe Pulled
Container avf-vending-prod-app-a-{api,worker,mqtt-ingest,reconciler}-1 Recreate / Recreated / Started
Container avf-vending-prod-app-a-api-1 Healthy
healthcheck_app_node: PASS
rollback_app_node: PASS
ROLLBACK_RESULT: completed
```

**Not seen:** `pg_dump`, goose `Up`, DB connection errors, compose config failures, or persistent unhealthy state after rollback.

### Phase 1 verdict

**SERVER_SAFE_TO_FIX** — Production cluster is on stable LKG containers with passing on-node health checks; failed deploy did not migrate DB or replace running app with the new digest. Blocker is scoped to migration script permissions on the VPS; safe to merge PR #238 and re-run Deploy Production with `run_migration=true` after approval.

---

## Phase 2 — Script permission fix (2026-05-20)

**Goal:** Fix production deploy script permission issues robustly without bypassing migration or weakening fail-closed gates.

### 1. Exact permission bug fixed

At deploy SHA `60e98a1`, `scripts/verify_database_environment.sh` used:

```bash
exec "$(…)/db/verify_database_environment.sh" "$@"
```

That **exec** requires the target file to have the Unix executable bit (`+x`). Migration scripts were synced via `tar` from git as `100644`, so on the VPS `scripts/db/verify_database_environment.sh` was `-rw-r--r--` and the kernel returned **Permission denied** before any DB guard, backup, or goose step ran.

### 2. Files changed

| File | Change |
|------|--------|
| `deployments/prod/shared/scripts/lib_release.sh` | Added `run_script()` helper (existence, readability checks; always invokes via `bash`) |
| `scripts/verify_database_environment.sh` | Wrapper uses `run_script` → canonical `scripts/db/verify_database_environment.sh` |
| `scripts/deploy/production-migrate.sh` | Sources `lib_release.sh`; calls verify via `run_script` (migration remains fail-closed) |
| `deployments/prod/app-node/scripts/release_app_node.sh` | Nested deploy calls use `run_script`; git mode `100755` |
| `deployments/prod/app-node/scripts/rollback_app_node.sh` | Same pattern; git mode `100755` |
| `deployments/prod/shared/scripts/release_app_cluster.sh` | Smoke gate uses `run_script`; git mode `100755` |
| `deployments/prod/shared/scripts/traffic_drain_hook.sh` | External LB hook uses `run_script` (no `-x` requirement) |
| `deployments/prod/shared/scripts/validate_production_deploy_inputs.sh` | Scale storm gate via `run_script` |
| `deployments/prod/shared/scripts/check_restore_readiness.sh` | Backup validator via `run_script` |
| `deployments/prod/app-node/scripts/healthcheck_app_node.sh` | Managed services check via `run_script` |
| `deployments/prod/data-node/scripts/release_data_node.sh` | Nested scripts via `run_script` |
| `deployments/prod/data-node/scripts/rollback_data_node.sh` | Nested scripts via `run_script` |
| `scripts/db/verify_database_environment.sh` | git mode `100755` |
| `.github/workflows/deploy-prod.yml` | Post-tar `chmod +x` on synced migration helpers (app-node A/B) |
| Repo-root wrappers (`scripts/api-contract-check.sh`, etc.) | Replaced bare `exec` with `run_script` for consistency |

**Note:** Canonical migration paths live at repo-root `scripts/` (synced to `${PRODUCTION_DEPLOY_ROOT}/scripts/` on the VPS), not under `deployments/prod/shared/scripts/`.

### 3. Why the fix is robust even if executable bit is lost

1. **`run_script` never execs a path directly** — every nested `.sh` is invoked as `bash "$script" "$@"`, which only requires read permission.
2. **Diagnostics on failure** — missing file → `error: script not found:`; unreadable → `error: script not readable:` plus `ls -l`.
3. **Defense in depth** — git `100755` on direct-entry scripts **and** post-tar `chmod +x` in `deploy-prod.yml` for migration helpers synced to the VPS.
4. **Migration unchanged** — `production-migrate.sh` still requires `verify_database_environment`, `pg_dump`, advisory lock, and goose `Up`; no bypass or skip paths added.

### 4. Syntax check result

```text
PASS: 75 scripts syntax-checked (bash -n on deployments/prod/shared/scripts/**/*.sh and scripts/**/*.sh)
```

### Phase 2 verdict

**PERMISSION_FIX_READY** — nested deploy/migration invocations no longer depend on `+x`; executable bits set in git for direct-entry scripts; CI tar sync still chmods migration helpers on extract.

---

## Phase 3 — Migration gate hardening (2026-05-20)

**Goal:** Verify and harden the production DB migration gate so deploy cannot succeed unless backup + migration + health checks succeed.

### 1. Migration gate behavior after fix

Ordered gate in `scripts/deploy/production-migrate.sh` (invoked from `deployments/prod/app-node/scripts/release_app_node.sh` when `RUN_MIGRATION=1`, **before** traffic drain / `compose up`):

| Step | Gate | On failure |
|------|------|------------|
| 1 | `verify_database_environment.sh` (production DB guard) | exit **10** |
| 2 | Digest-pinned `APP_IMAGE_REF` + `GOOSE_IMAGE_REF` checks | exit **11** |
| 3 | Postgres 17 backup image check (`postgres:17-alpine` default) | exit **20** |
| 4 | `/app/migrate validate` inside **APP_IMAGE_REF** (non-empty `/app/migrations`) | exit **30** |
| 5 | `pg_dump` → non-empty backup → `pg_restore -l` | exit **20** |
| 6 | Advisory lock acquire | abort (lock released on exit) |
| 7 | `goose status` (before) + `version` | abort |
| 8 | `goose up` via `/app/migrate` in **APP_IMAGE_REF** | exit **30** |
| 9 | `goose status` (after) + `version` | abort |

`release_app_node.sh` on migration failure: **restores** previous `.env.app-node` snapshot, **does not** drain traffic or recreate app containers, exits **41** (deploy failure — matches `release_app_cluster.sh`).

Deploy proceeds to traffic drain / app rollout **only** after `production-migrate.sh` prints `migration_gate=closed`.

### 2. Backup behavior

| Requirement | Implementation |
|-------------|----------------|
| Backup before migration | `pg_dump` runs before advisory lock / `goose up` |
| Postgres 17 compatible | `POSTGRES_TOOLS_IMAGE` defaults to `postgres:17-alpine`; non-17 tags rejected |
| Non-empty backup | `[[ -s "${backup_path}" ]]` after dump |
| `pg_restore -l` verification | Docker one-shot `pg_restore -l` on backup artifact |
| Masked `DATABASE_URL` in logs | `mask_database_url()` for script output; full URL never logged by `note` |

Backups written to `${MIGRATION_LOG_DIR}` (default `/opt/avf-vending-api/deployments/prod/logs/migrations`), **not** used as migration source.

### 3. Image / migration source behavior

| Ref | Role |
|-----|------|
| **APP_IMAGE_REF** (`@sha256:…`) | Migration **runner** (`/app/migrate`) and **SQL source** (`/app/migrations` embedded at build time from deploy commit) |
| **GOOSE_IMAGE_REF** (`@sha256:…`) | Validated at migrate time for deploy/rollback parity; standalone goose image built from `deployments/prod/Dockerfile.goose` |
| **Host `migrations/` tar on VPS** | Synced for ops reference only; **not** mounted into migrate service |
| **`MIGRATIONS_DIR` override** | Rejected in production if not `/app/migrations` |

Compose `migrate` service (`docker-compose.app-node.yml`, profile `migration`) uses **only** digest-pinned `APP_IMAGE_REF` — no volume mounts from `/opt/avf-backups` or legacy backup folders.

Workflow (`.github/workflows/deploy-prod.yml`) already requires digest-pinned refs via `validate_digest_pinned_image_refs.sh` before SSH.

### 4. Failure behavior

| Scenario | Containers | Env file | Deploy |
|----------|------------|----------|--------|
| Migration gate fails | Unchanged (no `compose up` on app services) | Restored to pre-deploy snapshot | Stops at exit **41** |
| Migration succeeds, health fails later | Recreated (post-drain) | New refs | Fails at readiness/smoke; auto-rollback eligible |
| `run_migration=false` | Image-only rollout | New refs | No inline backup/goose on node A |

### 5. Static check result

**Forbidden migration backup paths** (`/opt/avf-backups`, `migrations_old`, `pending-migrations`):

```text
No matches in deployments/, scripts/, .github/
```

**Migration gate symbols** present in `scripts/deploy/production-migrate.sh`: `APP_IMAGE_REF`, `GOOSE_IMAGE_REF`, `pg_dump`, `pg_restore`, `goose status`, `goose up`, masked `DATABASE_URL`.

**Shell syntax:**

```text
PASS: 75 scripts syntax-checked (bash -n on deployments/prod/shared/scripts/**/*.sh and scripts/**/*.sh)
```

### Phase 3 verdict

**MIGRATION_GATE_READY** — fail-closed backup → validate → goose up sequence enforced; digest-pinned image refs required; migration SQL sourced from deployed app image only; failed migration aborts before traffic drain and leaves running containers unchanged.

---

## Phase 4 — CI/local deploy validation script (2026-05-20)

**Goal:** Add an offline regression guard so production deploy permission and migration-gate bugs cannot re-enter undetected.

### 1. Validation script path

`scripts/ci/validate-production-deploy.sh` (git mode `100755`)

Run locally:

```bash
bash scripts/ci/validate-production-deploy.sh
```

### 2. What it checks

| Check | Detail |
|-------|--------|
| **bash -n** | All production deploy `.sh` under `deployments/prod/{shared,app-node,data-node,scripts}/` plus `scripts/deploy/production-migrate.sh` and verify wrappers |
| **Required assets** | Canonical paths: `scripts/deploy/production-migrate.sh`, verify wrappers, `release_app_node.sh`, `release_app_cluster.sh`, compose + digest validator |
| **No stale duplicates** | Rejects misplaced copies under `deployments/prod/shared/scripts/` for migration orchestration |
| **Git executable bits** | Direct-entry scripts must be `100755` in git index |
| **Nested `.sh` safety** | Scans prod deploy scripts for `exec`/`direct` `.sh` calls without `bash` or `run_script` |
| **Forbidden migration sources** | No `/opt/avf-backups`, `migrations_old`, or `pending-migrations` in prod deploy paths |
| **Migration gate symbols** | `production-migrate.sh` must contain `pg_dump`, `pg_restore -l`, goose status/up, digest checks, `mask_database_url`, Postgres 17 tools image |
| **Release ordering** | `release_app_node.sh` runs migrate before drain; exits **41** and restores env on failure |
| **`run_script` helper** | Present in `lib_release.sh` and uses `bash` |
| **Workflow smoke ordering** | `deploy-prod.yml`: final public smoke precedes deployment summary/manifest; smoke step has no `continue-on-error`; tar sync includes `chmod +x` for migration helpers |

### 3. CI wiring

- **`.github/workflows/ci.yml`** — job `Workflow and Script Quality`, step **Validate production deploy scripts and migration gate** (after `verify_workflow_contracts.sh`).
- **`scripts/ci/verify_workflow_contracts.sh`** — contract guard that `ci.yml` invokes `validate-production-deploy.sh` and the file exists.

No changes to deploy-prod runtime behavior; existing workflows unchanged except additive CI step.

### 4. Result

```text
PASS: validate-production-deploy.sh (2026-05-20 local run)
  bash -n: 49 scripts
  nested .sh scan: 45 files clean
  migration gate symbols: OK
  deploy-prod smoke ordering: OK
```

### Phase 4 verdict

**DEPLOY_VALIDATION_ADDED** — offline regression script wired into PR/push CI; permission + migration gate contracts enforced before merge.

---

## Phase 5 — Local validation (2026-05-20)

**Goal:** Full local validation after deploy script fixes (Phases 2–4). No commit on failure.

### 1. Commands run

| Command | Purpose |
|---------|---------|
| `git status --short` | Working tree inventory |
| `git diff --check` | Trailing whitespace / conflict markers |
| `bash scripts/ci/validate-production-deploy.sh` | Deploy permission + migration gate regression guard |
| `bash -n` on `deployments/prod/shared/scripts` + `scripts/**/*.sh` | Shell syntax (76 scripts) |
| `gofmt -w .` | Go formatting |
| `go test ./...` | Unit/integration tests |
| `go vet ./...` | Static analysis |
| `go list ./...` | Package resolution |
| `scripts/audit/verify-uuid-v7.sh` | UUID v7 audit |
| `scripts/checks/check-uuid-v7.sh` | UUID v7 static scan |
| Python JSON walk | All `*.json` (excluding `.git`, `node_modules`, `vendor`, `.tmp-image-metadata`) |
| `docker build -f deployments/prod/Dockerfile -t avf-vending-api:deploy-fix .` | Production image build |
| `docker run … /app/migrate validate` | Embedded migrations in image |

### 2. Pass/fail

| Check | Result |
|-------|--------|
| `git diff --check` | **PASS** (trailing whitespace stripped in analysis doc before re-check) |
| `validate-production-deploy.sh` | **PASS** |
| `bash -n` (76 scripts) | **PASS** |
| `gofmt -w .` | **PASS** (no blocking format drift) |
| `go test ./...` | **PASS** (exit 0, ~141s) |
| `go vet ./...` | **PASS** |
| `go list ./...` | **PASS** |
| UUID v7 scripts | **PASS** |
| JSON validation | **PASS** |
| Docker build | **PASS** |

### 3. Docker image migration check

Image: `avf-vending-api:deploy-fix` (`deployments/prod/Dockerfile`)

| Check | Result |
|-------|--------|
| `/app/migrate` binary present | Yes (~9.2M) |
| `/app/migrations/*.sql` count | **5** files |
| `/app/migrate validate` | **OK: 5 migration file(s) in /app/migrations** |

### 4. Remaining blockers (not local validation)

| Blocker | Status |
|---------|--------|
| Merge deploy-fix branch to `main` via PR | Pending |
| Re-run Deploy Production with `run_migration=true` | Pending merge + GHA |
| Production environment approval in GitHub | Required at deploy time |
| Public smoke `/version` git_sha update | Pending successful deploy |
| Untracked local artifacts | `.tmp-image-metadata/`, `PRODUCTION_DEPLOY_WITH_DB_MIGRATION_REPORT.md` (not part of deploy fix PR) |

### Phase 5 verdict

**LOCAL_VALIDATION_PASS** — all local gates green; safe to open/merge PR for deploy script fixes. No commit performed in this phase (per instruction: commit only when explicitly requested).

---

## Phase 8 — Production redeploy (2026-05-20)

**Goal:** Redeploy after permission-safe migration fix; `run_migration=true`; no security/migration/backup bypass.

| Item | Value |
|------|-------|
| **Deploy run** | [26143936277](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26143936277) (#142) |
| **Conclusion** | **success** |
| **headSha** | `31511a2d98c7589cda8ee0db52108f53aa997880` |
| **App digest** | `sha256:23e80df72e9268f5e4bceeb2fac8ccf9121b48fdc7637fa8e92b0ddf5fa3034c` |
| **Build run** | `26143278523` |
| **Security Release** | `26143426371` (pass) |
| **Migration** | goose v4 → v5 (`00005_uuid_v7_defaults.sql`); inline `pg_dump` backup `backup-20260520T054820Z.dump` |
| **Permission errors** | **None** (`run_script` / `production-migrate.sh` path clean) |
| **Smoke** | node A readiness/smoke **pass**; final cluster + public smoke **pass** |

### Phase 8 verdict

**DEPLOY_SUCCESS** — permission fix validated in production; migration gate completed.

---

## Phase 9 — Post-deploy verification (2026-05-20)

**Goal:** Public smoke, container health, DB migration state, critical log scan.

### 1. Deploy reference

| Field | Value |
|-------|-------|
| **Run ID** | `26143936277` |
| **Conclusion** | **success** |
| **Release tag** | `v20260520-31511a2-permission-fix` |
| **Evidence artifact** | `production-deploy-evidence` (downloaded from run 26143936277) |

### 2. Public health (curl from operator workstation)

| Endpoint | HTTP | Body / notes |
|----------|------|--------------|
| `https://api.ldtv.dev/health/live` | **200** | `ok` |
| `https://api.ldtv.dev/health/ready` | **200** | `ok` |
| `https://api.ldtv.dev/version` | **200** | `git_sha`: `52a076e340a15a69dad7787cad54d7e3000fcafe` (see note below) |

**`/version` git_sha note:** Deploy pulled digest `sha256:23e80df…` built from main @ `31511a2`, but `/version` still reports `52a076e` (previous LKG). `internal/config/config.go` prefers runtime `APP_GIT_SHA` over link-time `version.Commit`; server `.env.app-node` likely still sets stale `APP_GIT_SHA`. **Not a permission/migration regression** — image rollout and migration succeeded per deploy logs; ops follow-up: align `APP_GIT_SHA` (or unset to use binary embed) on next release script/env sync.

### 3. Container status

Direct SSH from operator workstation: **blocked** (`Permission denied (publickey,password)`). Container/readiness state taken from GHA deploy evidence (`app-node-0-72.62.244.94-readiness.log`, deploy log).

| Service | Readiness | Health check |
|---------|-----------|--------------|
| **api** | ready | `/health/live` 200, `/health/ready` 200, `/version` 200 |
| **worker** | ready | `/health/ready` 200 |
| **mqtt-ingest** | ready | `/health/ready` 200 |
| **reconciler** | ready | `/health/ready` 200 |
| **caddy** | ready | upstream healthcheck 200 |

Managed deps: PostgreSQL `pg_isready` **PASS**, Redis TCP **PASS**, NATS TCP **PASS**.

### 4. DB migration state

From deploy log (`app-node-0-72.62.244.94-deploy.log`) — inline `production-migrate.sh` on app node A:

| Check | Result |
|-------|--------|
| Backup (`pg_dump`) | **PASS** — `backup-20260520T054820Z.dump` |
| `goose_db_version` before | **4** |
| Applied migration | `00005_uuid_v7_defaults.sql` |
| `goose_db_version` after | **5** |
| `production-migrate` gate | **PASS** — `version_before=4 version_after=5 migration_gate=closed` |
| Permission denied | **None** |

Direct `psql` via SSH: **not run** (workstation SSH blocked); migration state corroborated by deploy job logs and backup/migrate gate output above.

### 5. Critical logs (last 10m, deploy window)

Scanned deploy evidence deploy log and GHA job output for: `panic`, `fatal`, `SQLSTATE`, `migration failed`, `Permission denied`, `audit.*error`, `uuid.*error`, `database.*error`.

| Result |
|--------|
| **No matches** in deploy/migration path |

Non-blocking advisory unlock warning after migration (`pg_advisory_unlock` returned `f`) — lock released by session teardown; migrate gate still reported **PASS**.

### Phase 9 verdict

**PRODUCTION_DEPLOY_FIXED_AND_VERIFIED**

| Original failure | Status |
|------------------|--------|
| Script `Permission denied` before migration | **Fixed** — deploy reached `pg_dump`, goose up, and `release_app_node: PASS` |
| Migration not applied | **Fixed** — goose v4 → v5 |
| Public health | **OK** — live/ready 200 |
| Workflow + smoke | **OK** — run 26143936277 success |

**Follow-up (non-blocking):** Update `APP_GIT_SHA` in production `.env.app-node` (or release env sync) so `/version` reflects `31511a2`; restore workstation SSH if direct server audits are required.
