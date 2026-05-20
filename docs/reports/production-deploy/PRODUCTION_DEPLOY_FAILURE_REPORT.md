# Production Deploy Failure Report

Phase 10 — **2026-05-20**. Incident analysis for failed post-deploy verification and related workflow evidence.  
No production DB reset, manual DB edits, rollback execution, or secret output.

**Related:** [POST_PRODUCTION_SMOKE_TEST_REPORT.md](./POST_PRODUCTION_SMOKE_TEST_REPORT.md) (Phase 8)

---

## Incident summary

| Field | Value |
|-------|-------|
| **Incident type** | **Post-deploy smoke test failure** — not a failed GitHub Actions deploy conclusion |
| **Trigger** | Phase 8 public verification: `/version` `git_sha` does not match deploy workflow headSha or `origin/main` |
| **Service impact** | **Low** — `/health/live` and `/health/ready` return **200**; API reachable |
| **Data impact** | **None observed** — migration not enabled on referenced deploy |

---

## 1. Failed workflow run

### Primary incident (verification failure)

| Field | Value |
|-------|-------|
| Failing check | Phase 8 smoke — **PRODUCTION_SMOKE_FAILED** |
| Associated deploy workflow | **26093589896** — Deploy Production |
| Workflow conclusion | **success** (GitHub Actions) |
| URL | https://github.com/leduytuanvu/avf-vending-api/actions/runs/26093589896 |
| headSha (workflow) | `6527d502437f5137fb05c56d4851043b258afbc1` |
| headSha (`/version` public) | `52a076e340a15a69dad7787cad54d7e3000fcafe` |
| Mismatch | **Deploy workflow green; runtime reports stale build** |

### Historical failed Deploy Production runs (context only)

| Run id | Conclusion | headSha | Failed step | Error |
|--------|------------|---------|-------------|-------|
| **26011898871** | failure | `b5651000…` | Validate Production Action Safety | `target digests already match the latest successful production deployment (run #132)` |
| **26011771134** | failure | `b5651000…` | Validate Production Action Safety | Same — rollback/no-op rejected |

These are **rollback attempts that correctly failed** (nothing to change). Not the current smoke-test incident.

### Superseded Security Release failure

| Run id | Conclusion | Note |
|--------|------------|------|
| 26091763964 | failure | Cancelled/replaced; **26092412966** succeeded for `6527d502` |

---

## 2. Failed step

| Layer | Step / check | Result |
|-------|--------------|--------|
| GitHub Actions (26093589896) | All 10 jobs including **Deploy production release** | **success** |
| Post-deploy verification (Phase 8) | `/version` `git_sha` vs deploy headSha | **FAIL** |
| Post-deploy verification | SSH container inspection | **SKIPPED** — no SSH access |

**Deploy job notes (from logs):**

- `INPUT_RUN_MIGRATION: false`
- `INPUT_BACKUP_EVIDENCE_ID:` (empty)
- Staging gate: **bypassed** (`allow_missing_staging_evidence: true`)
- `Skipping app-node-b because it shares the data-node host and ALLOW_APP_NODE_ON_DATA_NODE is not true`
- Target app digest: `ghcr.io/leduytuanvu/avf-vending-api@sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe`

---

## 3. Error logs

### Public health (2026-05-20)

```
GET /health/live  → HTTP/1.1 200 OK  body: ok
GET /health/ready → HTTP/1.1 200 OK  body: ok
GET /version      → HTTP/1.1 200 OK
```

```json
{
  "version": "v1.0.01",
  "git_sha": "52a076e340a15a69dad7787cad54d7e3000fcafe",
  "node_name": "app-node-a",
  "app_env": "production"
}
```

**Verification error (logical, not HTTP):** expected `git_sha` **`6527d502437f5137fb05c56d4851043b258afbc1`**, got **`52a076e340a15a69dad7787cad54d7e3000fcafe`** (PR #99 era).

### GitHub Actions — failed rollback runs (26011898871)

```
error: target digests already match the latest successful production deployment (run #132: https://github.com/leduytuanvu/avf-vending-api/actions/runs/26011255157)
##[error]Process completed with exit code 1.
```

### SSH container logs

```
root@72.62.244.94: Permission denied (publickey,password).
```

Container and application logs **not collected** from this environment.

---

## 4. Whether migration ran

| Source | Result |
|--------|--------|
| Deploy run `26093589896` input | `run_migration: false` |
| Deploy job env | `INPUT_RUN_MIGRATION: false` |
| Staging bypass reason text | *"no DB migration"* |
| **Conclusion** | **Migration did not run** on referenced deploy |

---

## 5. Whether DB backup exists

| Source | Result |
|--------|--------|
| `backup_evidence_id` input | **Empty** (not required when `run_migration=false`) |
| **Conclusion** | **No backup step** for this deploy path |

Production DB was **not reset or manually edited** in any phase.

---

## 6. Whether app containers changed

| Check | Result |
|-------|--------|
| SSH `docker compose ps` | **Not verified** — SSH unavailable |
| Public `/version` git_sha | **Unchanged vs expected** — still `52a076e` after deploy run claiming `6527d502` |
| **Assessment** | **Unknown / likely not updated** on `app-node-a` — workflow success does not match runtime version identity |

Possible causes (require operator SSH to confirm):

1. Image pull/recreate did not occur on host despite workflow success.
2. Rolling deploy updated a different node; traffic still on stale `app-node-a`.
3. Caddy/load balancer still routing to old container set.
4. Deploy script completed without failing on version mismatch in workflow smoke.

---

## 7. Whether rollback is available

| Mechanism | Available? | Notes |
|-----------|------------|-------|
| **Deploy Production** `action_mode=rollback` | Yes | `.github/workflows/deploy-prod.yml` — digest-pinned rollback refs |
| **Rollback (Production — Incident)** | Yes | `.github/workflows/rollback-prod.yml` — preflight evidence only; `dry_run` default **true** |
| Prior successful deploy manifest | Run **26093589896** or earlier e.g. **26011255157** | Can supply `production-deployment-manifest` artifact |
| Automatic rollback | **No** — manual operator dispatch only |

**Rollback not executed in Phase 10** — no operator approval.

---

## 8. Recommended fix

### Immediate (operator with SSH)

1. SSH to **`72.62.244.94`** (`app-node-a`) and run:
   ```bash
   cd /opt/avf-vending-api/deployments/prod/app-node
   export COMPOSE_FILE=docker-compose.app-node.yml
   export COMPOSE_ENV_FILE=.env.app-node
   export COMPOSE_PROJECT_NAME=avf-vending-prod-app-a
   docker compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" ps
   docker compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" images
   ```
2. Compare running image digest to deploy target:
   `ghcr.io/leduytuanvu/avf-vending-api@sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe`
3. If stale: re-run **Deploy Production** with resolved inputs from [PRODUCTION_DEPLOY_INPUT_RESOLUTION.md](./PRODUCTION_DEPLOY_INPUT_RESOLUTION.md) **or** on-host `docker compose pull && up -d` per runbook (only with operator approval).
4. Re-check `curl https://api.ldtv.dev/version` → `git_sha` must match `6527d502…`.

### Process fixes (fix-forward)

1. **Staging gate:** Run successful **Staging Deployment Contract** on `develop`; stop relying on `allow_missing_staging_evidence`.
2. **Workflow smoke hardening:** Fail deploy workflow if post-deploy `/version` `git_sha` ≠ `SOURCE_COMMIT_SHA` (if not already enforced).
3. **Two-node rolling:** Investigate `app-node-b` skip — ensure both nodes or LB updated.
4. **Feature pipeline:** Fix PR #227 CI → merge verification branch → new build chain before next production release.

---

## 9. Whether manual rollback is required

| Question | Answer |
|----------|--------|
| Is production down? | **No** — health endpoints 200 |
| Is bad new code live? | **No** — old `52a076e` still running (stable but stale) |
| Did migration run? | **No** |
| **Manual rollback required?** | **No** — service healthy; issue is **failed forward rollout / verification**, not a broken new release |

If a future deploy pushes bad code with failing health checks, use **Deploy Production** rollback mode or **Rollback (Production — Incident)** with documented incident id and digest-pinned previous refs — **only with explicit operator approval**.

---

## Final verdict

### **FAILURE_ANALYZED**

### **FIX_FORWARD_REQUIRED**

**Not `ROLLBACK_REQUIRED`** — production is healthy on the current (stale) build; the failure is **version drift after a green deploy workflow** and incomplete post-deploy verification. Fix by investigating host containers and re-deploying with confirmed image digests, not by rolling back to an older digest while health is green.

---

*Phase 10 complete — no rollback triggered, no DB changes, failure documented.*
