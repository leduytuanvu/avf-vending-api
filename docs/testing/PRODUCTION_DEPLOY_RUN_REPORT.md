# Production Deploy Run Report

Phase 7 — **2026-05-20**. No production deploy triggered in this phase.

---

## Phase 7 precondition

| Check | Result |
|-------|--------|
| Phase 6 verdict | **`BLOCKED_INPUTS_MISSING`** — not `READY_TO_DEPLOY` |
| Missing input | `staging_evidence_id` (no successful Staging Deployment Contract run) |
| Policy | Do not bypass gates without documented operator approval |
| Action | **Deploy not triggered** |

See `docs/testing/PRODUCTION_DEPLOY_INPUT_RESOLUTION.md` for resolved build/security inputs and bypass requirements.

---

## Pre-flight: production workflow status (2026-05-20)

| Check | Result |
|-------|--------|
| Deploy Production in progress? | **No** — no `in_progress` runs |
| Deploy mode | **Manual** (`workflow_dispatch` only) |
| Automatic production deploy? | **No** — nothing to monitor; manual dispatch required |
| Latest Deploy Production run | `26093589896` — **success** (2026-05-19, prior operator dispatch) |

---

## Phase 7 deploy run

| Field | Value |
|-------|-------|
| **Run id** | *N/A — not triggered* |
| **Workflow URL** | *N/A* |
| **Head SHA** | Target would have been `6527d502437f5137fb05c56d4851043b258afbc1` (current `main`) |
| **Inputs used** | *None — dispatch skipped* |

### Inputs that would be required (from Phase 6 resolution)

Only for reference if Phase 6 reaches `READY_TO_DEPLOY`:

| Input | Resolved value |
|-------|----------------|
| `action_mode` | `deploy` |
| `build_run_id` | `26092186016` |
| `security_release_run_id` | `26092412966` |
| `app_image_ref` | `ghcr.io/leduytuanvu/avf-vending-api@sha256:91608b23870da8f219145c42095054e3f8fdbd91c44fc3bb2f59b1d3ef9d1efe` |
| `goose_image_ref` | `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:420301b45a445f8f70b780f915c373d973d3b88b9a095914b35020684d1142ee` |
| `deploy_production_confirmation` | `DEPLOY_PRODUCTION` |
| `staging_evidence_id` | **Unresolved** — blocks strict-path deploy |
| `run_migration` | `false` (no DB migration; no backup required) |

---

## Step results (Phase 7 run)

| Step | Result |
|------|--------|
| Migration | **N/A** — no run triggered (`run_migration` would be `false`) |
| DB backup | **N/A** — not required when `run_migration=false` |
| Health check | **N/A** |
| Smoke test | **N/A** |

Hard rules observed: no production DB reset, no manual DB edits, no gate bypass, no duplicate dispatch.

---

## Reference: last successful production deploy (not Phase 7)

Prior operator deploy for the same `main` SHA (informational only):

| Field | Value |
|-------|-------|
| Run id | `26093589896` |
| URL | https://github.com/leduytuanvu/avf-vending-api/actions/runs/26093589896 |
| headSha | `6527d502437f5137fb05c56d4851043b258afbc1` |
| Conclusion | **success** |
| Staging gate | Bypassed (`allow_missing_staging_evidence: true`) with operator reason |

This run is **not** a Phase 7 outcome; production already holds this release.

---

## Final conclusion

### **DEPLOY_NOT_TRIGGERED**

Phase 6 did not return `READY_TO_DEPLOY`. Production deploy was **not** dispatched per Phase 7 rules.

To proceed:

1. Complete staging gate (successful **Staging Deployment Contract** → supply `staging_evidence_id`), **or**
2. Obtain fresh operator approval and update Phase 6 to `READY_TO_DEPLOY` via documented bypass path, **then** re-run Phase 7.

Do **not** re-deploy the same SHA without a documented reason — production already matches `6527d502` from run `26093589896`.

---

*Phase 7 complete — no GitHub Actions deploy workflow started.*
