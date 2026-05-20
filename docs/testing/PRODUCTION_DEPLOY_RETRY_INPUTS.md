# Production deploy retry — resolved inputs (Phase 7)

**Generated:** 2026-05-20  
**Scope:** Resolve exact `workflow_dispatch` inputs for **Deploy Production** after Phase 6 merge to `main`. **No deploy triggered in this phase.**

---

## 1. Main SHA

| Field | Value |
|-------|-------|
| **Commit** | `31511a2d98c7589cda8ee0db52108f53aa997880` |
| **Short** | `31511a2` |
| **Message** | `merge: production deploy permission fix into main` |
| **Branch** | `main` (`origin/main`) |

---

## 2. Build run (latest successful for current main SHA)

| Field | Value |
|-------|-------|
| **Workflow** | Build and Push Images (`.github/workflows/build-push.yml`) |
| **Run ID** | [26143278523](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26143278523) |
| **Conclusion** | success |
| **headSha** | `31511a2d98c7589cda8ee0db52108f53aa997880` |
| **Triggered by** | CI success on main push (workflow_run chain) |
| **Artifact source** | `immutable-image-contract`, `image-build-metadata`, `production-deploy-candidate` |

---

## 3. APP_IMAGE_REF

```
ghcr.io/leduytuanvu/avf-vending-api@sha256:23e80df72e9268f5e4bceeb2fac8ccf9121b48fdc7637fa8e92b0ddf5fa3034c
```

| Digest | `sha256:23e80df72e9268f5e4bceeb2fac8ccf9121b48fdc7637fa8e92b0ddf5fa3034c` |

---

## 4. GOOSE_IMAGE_REF

```
ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:5ab097fecc1d0a6dea400d2a7930cdf638339763731661f0110df43842f80756
```

| Digest | `sha256:5ab097fecc1d0a6dea400d2a7930cdf638339763731661f0110df43842f80756` |

---

## 5. Security Release (required for deploy)

| Field | Value |
|-------|-------|
| **Workflow** | Security Release (`.github/workflows/security-release.yml`) |
| **Run ID** | [26143426371](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26143426371) |
| **Conclusion** | success |
| **Verdict** | `pass` (`security-verdict.json`) |
| **source_build_run_id** | `26143278523` (matches build run above) |
| **Candidate artifact** | `production-deploy-candidate` (includes `production-deploy-inputs.json`) |

---

## 6. Production deploy workflow

| Field | Value |
|-------|-------|
| **Display name** | Deploy Production |
| **File** | `.github/workflows/deploy-prod.yml` |
| **Trigger** | `workflow_dispatch` on **`main` only** |
| **Concurrency** | `production-deploy` (serialized; no cancel-in-progress) |
| **Environment** | `production` (GitHub Environment protection / approval in UI) |

---

## 7. Required `workflow_dispatch` inputs (deploy mode)

Values below are from Security Release artifact `production-deploy-inputs.json` (run 26143426371), with operator notes for this permission-fix retry.

| Input | Required | Resolved value | Notes |
|-------|----------|----------------|-------|
| `action_mode` | yes | `deploy` | |
| `build_run_id` | yes (deploy) | `26143278523` | Must match `security-verdict.source_build_run_id` |
| `security_release_run_id` | yes (deploy) | `26143426371` | Security Release run, **not** Build run |
| `release_tag` | yes | `v20260520-31511a2` | Operator may relabel |
| `source_commit_sha` | optional | `31511a2d98c7589cda8ee0db52108f53aa997880` | Confirmation only |
| `app_image_ref` | yes (deploy) | `ghcr.io/leduytuanvu/avf-vending-api@sha256:23e80df72e9268f5e4bceeb2fac8ccf9121b48fdc7637fa8e92b0ddf5fa3034c` | Digest-pinned |
| `goose_image_ref` | yes (deploy) | `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:5ab097fecc1d0a6dea400d2a7930cdf638339763731661f0110df43842f80756` | Digest-pinned |
| `deploy_production_confirmation` | yes (deploy) | `DEPLOY_PRODUCTION` | Exact string |
| `run_migration` | optional (default `true`) | **`true`** | **Override:** generated candidate has `false`; set **`true`** for this retry (permission fix + goose migration) |
| `staging_evidence_id` | yes* | **UNRESOLVED** | See §8 |
| `allow_missing_staging_evidence` | bypass only | `false` (default) | Set `true` only with reason if bypass approved |
| `missing_staging_evidence_reason` | if bypass | empty | Required when bypass enabled |
| `staging_evidence_max_age_hours` | optional | `168` | |
| `fleet_scale_target` | optional | `pilot` | |
| `deploy_data_node` | optional | `false` | |
| `allow_app_node_on_data_node` | optional | `false` | |
| `enable_business_synthetic_smoke` | optional | `false` | |

\*Required unless `allow_missing_staging_evidence=true` **and** non-empty `missing_staging_evidence_reason`.

### Example dispatch JSON (after staging resolved)

```json
{
  "action_mode": "deploy",
  "build_run_id": "26143278523",
  "security_release_run_id": "26143426371",
  "release_tag": "v20260520-31511a2-permission-fix",
  "source_commit_sha": "31511a2d98c7589cda8ee0db52108f53aa997880",
  "app_image_ref": "ghcr.io/leduytuanvu/avf-vending-api@sha256:23e80df72e9268f5e4bceeb2fac8ccf9121b48fdc7637fa8e92b0ddf5fa3034c",
  "goose_image_ref": "ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:5ab097fecc1d0a6dea400d2a7930cdf638339763731661f0110df43842f80756",
  "deploy_production_confirmation": "DEPLOY_PRODUCTION",
  "run_migration": true,
  "staging_evidence_id": "<STAGING_RUN_ID_OR_USE_BYPASS>",
  "fleet_scale_target": "pilot"
}
```

CLI (from authenticated clone, after replacing staging):

```bash
gh workflow run "Deploy Production" --ref main --json < production-deploy-inputs.json
```

Download fresh candidate from Security Release run 26143426371:

```bash
gh run download 26143426371 -n production-deploy-candidate -D ./production-deploy-candidate
```

---

## 8. Staging gate / bypass

| Check | Result |
|-------|--------|
| **Default policy** | Production deploy requires `staging_evidence_id` = run id of a **successful** **Staging Deployment Contract** workflow with artifact `staging-deploy-evidence` whose `app_image_ref` / `goose_image_ref` digests **match** production inputs |
| **Generated candidate** | `staging_evidence_id` = `TODO_STAGING_EVIDENCE_RUN_ID` (placeholder — **not deployable as-is**) |
| **Recent Staging Deployment Contract on main** | All recent runs **failed** or **skipped** (2026-04-25 and later) |
| **Last successful staging runs** | April 2026 (e.g. [24767831785](https://github.com/leduytuanvu/avf-vending-api/actions/runs/24767831785)) — **stale**; digests do **not** match current build |
| **Bypass path** | `allow_missing_staging_evidence=true` + non-empty `missing_staging_evidence_reason` (temporary; logged in manifest) |
| **Strict path** | Run **Staging Deployment Contract** (via develop/staging pipeline) for images matching digests above, then supply that run id |

**For this retry:** staging evidence for digests `23e80df…` / `5ab097fe…` is **not available** without a new staging deploy or an approved bypass.

---

## 9. Production LKG reference (pre-retry)

| Field | Value |
|-------|-------|
| **Live app digest (LKG)** | `sha256:91608b…` (`git_sha: 52a076e`) — unchanged; last failed deploy [26141230829](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26141230829) failed on script permissions before migration |

---

## 10. Phase 7 verdict

**BLOCKED_INPUTS_MISSING**

| Resolved | Blocking |
|----------|----------|
| Main SHA `31511a2` | `staging_evidence_id` — no matching successful Staging Deployment Contract for current digests |
| Build run `26143278523` | Operator must set `run_migration=true` (candidate defaults `false`) |
| APP / GOOSE digest-pinned refs | Optional: `release_tag` relabel |
| Security Release `26143426371` (pass) | GitHub **production** environment approval at dispatch time |
| Deploy workflow + input schema | |

**To reach READY_TO_REDEPLOY:** either (a) complete staging deploy and record matching `staging_evidence_id`, or (b) use approved `allow_missing_staging_evidence` + reason, set `run_migration=true`, then dispatch **Deploy Production** on `main`.
