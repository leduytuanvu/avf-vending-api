# Production Deploy Input Resolution

Phase 6 — resolved from GitHub Actions on **2026-05-20**. No production deploy triggered in this phase.

---

## Release chain (main)

```
CI (push) → Build and Push Images → Security Release → [Staging Deployment Contract on develop] → Deploy Production (manual)
```

| Step | Workflow | File | Trigger |
|------|----------|------|---------|
| 1 | CI | `.github/workflows/ci.yml` | `push` to `main` |
| 2 | Build and Push Images | `.github/workflows/build-push.yml` | `workflow_run` after CI success |
| 3 | Security Release | `.github/workflows/security-release.yml` | `workflow_run` after Build success |
| 4 | Staging Deployment Contract | `.github/workflows/deploy-develop.yml` | `workflow_run` after Security Release on **develop** |
| 5 | Deploy Production | `.github/workflows/deploy-prod.yml` | **`workflow_dispatch` only** (manual, `main`) |

Deploy is **never automatic** on merge to `main`.

---

## 1. Main commit SHA to deploy

| Field | Value |
|-------|-------|
| **SHA** | `6527d502437f5137fb05c56d4851043b258afbc1` |
| **Message** | Merge pull request #226 from leduytuanvu/develop |
| **Branch tip** | `origin/main` @ same SHA (verified 2026-05-20) |
| **Upstream CI run** | `26091978633` — CI, push, **success** |

> Phase 4 verification work (`chore/final-full-system-verification-uuidv7-postman-tests`) is **not** on `main`. Deploying this SHA does not include UUID v7 / migration gate changes from that branch.

---

## 2. Build workflow run id

| Field | Value |
|-------|-------|
| **Workflow** | Build and Push Images |
| **Run id** | **`26092186016`** |
| **URL** | https://github.com/leduytuanvu/avf-vending-api/actions/runs/26092186016 |
| **Conclusion** | success |
| **headSha** | `6527d502437f5137fb05c56d4851043b258afbc1` |
| **Triggered by** | workflow_run (CI push success) |
| **Artifact source** | `immutable-image-contract` (downloaded and verified) |

---

## 3. Image digest / ref

From artifact `immutable-image-contract` on build run `26092186016` (matches `security-verdict` on run `26092412966`):

| Image | Digest-pinned ref |
|-------|-------------------|
| **App** | `ghcr.io/leduytuanvu/avf-vending-api@sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe` |
| **Goose** | `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:420301b45a445f8f70b780f915c373d973d3b88b9a095914b35020684d1142ee` |

| Digest only | Value |
|-------------|-------|
| app_digest | `sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe` |
| goose_digest | `sha256:420301b45a445f8f70b780f915c373d973d3b88b9a095914b35020684d1142ee` |

---

## 4. Security workflow run id

| Field | Value |
|-------|-------|
| **Workflow** | Security Release |
| **Run id** | **`26092412966`** |
| **URL** | https://github.com/leduytuanvu/avf-vending-api/actions/runs/26092412966 |
| **Conclusion** | success |
| **headSha** | `6527d502437f5137fb05c56d4851043b258afbc1` |
| **source_build_run_id** (verdict) | `26092186016` |
| **security_verdict** | pass |
| **Artifact** | `security-verdict` |

---

## 5. Production workflow

| Field | Value |
|-------|-------|
| **Name** | Deploy Production |
| **File** | `.github/workflows/deploy-prod.yml` |
| **Trigger** | `workflow_dispatch` on branch **`main`** only |
| **Environment** | `production` (GitHub Environment protection — configured in UI, not in YAML) |

---

## 6. Required `workflow_dispatch` inputs (deploy mode)

### Resolved from successful main workflows (do not guess)

| Input | Required | Resolved value |
|-------|----------|----------------|
| `action_mode` | yes | `deploy` |
| `build_run_id` | yes (deploy) | **`26092186016`** |
| `security_release_run_id` | yes (deploy) | **`26092412966`** |
| `app_image_ref` | yes (deploy) | **`ghcr.io/leduytuanvu/avf-vending-api@sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe`** |
| `goose_image_ref` | yes (deploy) | **`ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:420301b45a445f8f70b780f915c373d973d3b88b9a095914b35020684d1142ee`** |
| `deploy_production_confirmation` | yes (deploy) | **`DEPLOY_PRODUCTION`** (literal) |
| `source_commit_sha` | optional confirm | **`6527d502437f5137fb05c56d4851043b258afbc1`** |

### Operator-provided (not derivable from CI artifacts)

| Input | Required | Notes |
|-------|----------|-------|
| `release_tag` | yes | Operator label, e.g. `v20260519-6527d50` (used in prior successful deploy) |

### Staging / pre-prod gate (see §8)

| Input | Strict path | Bypass path (workflow-supported) |
|-------|-------------|----------------------------------|
| `staging_evidence_id` | **Required** — run id of successful **Staging Deployment Contract** with artifact `staging-deploy-evidence` | Leave empty when bypassing |
| `allow_missing_staging_evidence` | `false` (default) | `true` |
| `missing_staging_evidence_reason` | n/a | **Required** non-empty single-line operator justification |

### Optional inputs (defaults from prior successful deploy run `26093589896`)

| Input | Prior deploy value |
|-------|-------------------|
| `run_migration` | `false` |
| `deploy_data_node` | default `false` |
| `allow_app_node_on_data_node` | default `false` |
| `fleet_scale_target` | default `pilot` |
| `allow_scale_gate_bypass` | default `false` |
| `enable_business_synthetic_smoke` | default `false` |
| `staging_evidence_max_age_hours` | default `168` |

When `run_migration` is `true`, `backup_evidence_id` becomes **required** (not resolved in this phase).

---

## 7. Automatic vs manual deploy

| Aspect | Behavior |
|--------|----------|
| Build / Security Release | **Automatic** after green CI push to `main` |
| Staging (develop) | **Automatic** after Security Release on `develop` (separate chain) |
| **Production** | **Manual only** — operator runs **Deploy Production** via Actions → workflow_dispatch |
| Latest production deploy | Run **`26093589896`** — success, same SHA/images (2026-05-19) |

---

## 8. Staging / pre-prod gate

| Check | Result |
|-------|--------|
| Gate required by default? | **Yes** — `staging_evidence_id` OR explicit bypass |
| Successful Staging Deployment Contract for matching digests? | **No** — recent runs (15 queried) are failure/skipped/cancelled; none success |
| `staging_evidence_id` resolvable from workflows? | **No** — cannot populate without guessing |
| Bypass supported? | **Yes** — `allow_missing_staging_evidence: true` + `missing_staging_evidence_reason` |
| Prior approved bypass (run `26093589896`) | `allow_missing_staging_evidence: true`, reason: *"Temporary production pilot before real staging is enabled; approved by operator; no DB migration."* |

**Strict enterprise path:** staging gate **blocks** deploy until a successful **Staging Deployment Contract** run produces `staging-deploy-evidence` with matching app/goose digests.

---

## 9. Bypass inputs (only if staging evidence unavailable)

| Input | When | Why |
|-------|------|-----|
| `allow_missing_staging_evidence` | `true` | No successful staging run exists; workflow explicitly allows temporary bypass (default `false`) |
| `missing_staging_evidence_reason` | non-empty string | Mandatory audit trail per `deploy-prod.yml`; echoed in summary and production manifest |
| `allow_scale_gate_bypass` | only if `fleet_scale_target` ≠ `pilot` | Requires `scale_gate_bypass_reason`; not needed for default `pilot` |

Do **not** bypass without documented operator approval. Bypass does not replace Security Release or digest-pinned image confirmation.

---

## Example: strict-path dispatch (blocked until staging exists)

```yaml
action_mode: deploy
build_run_id: "26092186016"
security_release_run_id: "26092412966"
release_tag: "<operator-label>"
source_commit_sha: "6527d502437f5137fb05c56d4851043b258afbc1"
app_image_ref: "ghcr.io/leduytuanvu/avf-vending-api@sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe"
goose_image_ref: "ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:420301b45a445f8f70b780f915c373d973d3b88b9a095914b35020684d1142ee"
deploy_production_confirmation: "DEPLOY_PRODUCTION"
staging_evidence_id: "<UNRESOLVED — no successful Staging Deployment Contract run>"
allow_missing_staging_evidence: false
run_migration: false
```

## Example: bypass-path dispatch (matches prior successful deploy `26093589896`)

Only use with **documented operator approval** — same inputs as the last successful production deploy for this SHA:

```yaml
action_mode: deploy
build_run_id: "26092186016"
security_release_run_id: "26092412966"
release_tag: "v20260519-6527d50"
source_commit_sha: "6527d502437f5137fb05c56d4851043b258afbc1"
app_image_ref: "ghcr.io/leduytuanvu/avf-vending-api@sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe"
goose_image_ref: "ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:420301b45a445f8f70b780f915c373d973d3b88b9a095914b35020684d1142ee"
deploy_production_confirmation: "DEPLOY_PRODUCTION"
allow_missing_staging_evidence: true
missing_staging_evidence_reason: "<operator-approved reason — update per incident/release>"
run_migration: false
```

---

## Main CI / build / security status (@ `6527d502`)

| Workflow | Run id | Conclusion |
|----------|--------|------------|
| CI | 26091978633 | success |
| Security (push) | 26091978720 | success |
| Enterprise release verification | 26091978636 | success |
| Build and Push Images | 26092186016 | success |
| Security Release | 26092412966 | success |
| Deploy Production (prior) | 26093589896 | success |

---

## Final verdict

### **BLOCKED_INPUTS_MISSING** (strict / default policy)

**Resolved:** main SHA, build run id, security release run id, app/goose digest-pinned refs, production workflow and deploy-mode inputs (except operator `release_tag`).

**Missing:** `staging_evidence_id` — no successful **Staging Deployment Contract** run with artifact `staging-deploy-evidence` matching these image digests exists in workflow history.

**Additional context:**

- Production for this SHA/images was **already deployed** successfully on 2026-05-19 (run `26093589896`) using staging bypass with operator reason.
- A **re-deploy** of the same SHA is optional, not required.
- New verification work (UUID v7, migration gates) requires merging to `main` and a **new** CI → Build → Security Release chain before deploy.

**Do not deploy in Phase 6.** To reach **READY_TO_DEPLOY** under strict policy: run and pass **Staging Deployment Contract** on `develop`, then supply `staging_evidence_id` with digest cross-check. Alternatively, use the documented bypass path only with fresh operator approval.

---

*Generated Phase 6 — input resolution only; no `workflow_dispatch` triggered.*
