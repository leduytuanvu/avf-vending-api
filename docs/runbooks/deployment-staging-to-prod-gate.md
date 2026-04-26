# Operator runbook: develop → staging → production

This is the end-to-end path for a **default, enterprise-style** production rollout. Production is **manual** (`Deploy Production` on `main`); develop changes must pass **CI**, **Build and Push Images**, **Security Release**, and **pre-prod staging** before an operator runs production with matching evidence.

## 1. Merge to `develop`

- Land changes via your normal process (PR review, merge to `develop`).

## 2. Wait for CI, Build, and Security Release

- **CI** runs on the merge (and related events).
- **Build and Push Images** must complete **successfully** for `develop` when the change is a releasable candidate.
- **Security Release** must complete **successfully** after that build, producing a passing **`security-verdict`** (and other release artifacts) for the develop chain.

## 3. Staging / pre-prod (mandatory for strict production)

1. Confirm **Staging Deployment Contract** (triggered from Security Release on `develop`) runs.
2. For a **real** host deploy, set **`vars.ENABLE_REAL_STAGING_DEPLOY=true`** and the staging host/SSH settings per [`staging-preprod.md`](staging-preprod.md).
3. Open the **Staging Deployment Contract** run in GitHub Actions. Verify **conclusion: success** and the published artifacts, especially **`staging-release-evidence`**.

## 4. Collect evidence ids for production

You will use these in **Deploy Production** (all numeric **GitHub Actions run ids** from the relevant workflows):

| Role | How to get it |
|------|----------------|
| **Build (main)** | From the successful **Build and Push Images** run on `main` for the version you are promoting (e.g. after merging `develop` → `main`). This is your **`build_run_id`**. |
| **Security Release (main)** | From the **Security Release** run on `main` that authorized that build, includes **`security_release_run_id`**. It must match the `security-verdict` you rely on. |
| **Staging pre-prod** | From a successful **Staging Deployment Contract** run that has artifact **`staging-release-evidence`**. This is your **`staging_evidence_id`**. Use the run that exercised the **same image digests** you will deploy to production. |

**Digest rule:** The workflow compares **app and goose image digests** in `staging-release-evidence.json` to the `app_image_ref` / `goose_image_ref` you enter for production. The staging and production **build run ids** may differ (develop vs main); matching digests are what make the gate meaningful.

## 5. Run Deploy Production (manual) on `main`

In **Actions → Deploy Production → Run workflow** (on branch **`main`**) set at minimum:

- **`action_mode`**: `deploy`
- **`build_run_id`**: main build run id (must match the security-verdict’s `source_build_run_id` for that Security Release)
- **`security_release_run_id`**: main Security Release run id
- **`staging_evidence_id`**: Staging Deployment Contract run id from step 4
- **`app_image_ref` / `goose_image_ref`**: exact digest-pinned refs (must match security-verdict and staging evidence for strict gate)
- **`deploy_production_confirmation`**: `DEPLOY_PRODUCTION`
- **`release_tag`**: your operator label (e.g. `v1.2.3`)

**Temporary bypass (only if staging is not yet enforceable in your org):** set **`allow_missing_staging_evidence: true`** and a single-line **`missing_staging_evidence_reason`**. The run will **warn loudly** and record the bypass in the manifest. Remove this when real staging is in place.

## 6. After deploy

- Keep **`production-deployment-manifest`** (artifact) and the job summary as your audit record; they include **`staging_release_gate`**, bypass flags, and staging cross-check notes.

## Related

- [Staging / pre-prod setup](staging-preprod.md)
- [CI/CD staging–production contract](../ci-cd/staging-production-gate.md)
- [Prod / GHCR image deploy flow](prod-ghcr-image-only-deploy.md) (if applicable)
