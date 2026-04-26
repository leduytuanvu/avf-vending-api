# Release process (enterprise)

This page is the **operator-oriented** end-to-end map from code merge to production. It complements the maintainer entry point [docs/runbooks/cicd-release.md](../runbooks/cicd-release.md), artifact details in [docs/runbooks/release-process.md](../runbooks/release-process.md), and the staging contract in [staging-preprod-gate.md](staging-preprod-gate.md).

**Production deploy is manual only:** there is **no** `on.push` or `workflow_run` path that starts **Deploy Production**. Operators run it from the GitHub **Actions** UI on branch **`main`** using **`workflow_dispatch`**.

**Two-VPS “zero downtime” is not implied:** rolling deploy is sequential in CI, but **global** uninterrupted traffic requires correct DNS/load-balancing and optional traffic-drain settings. See [two-vps-rolling-production-deploy.md](two-vps-rolling-production-deploy.md) (`TRAFFIC_DRAIN_MODE`, `external-lb` hook).

---

## 1. Pull request into `develop`

| Step | What happens |
| --- | --- |
| Open PR | Target **`develop`** (or **`main`**, if your org uses direct promotion; branch protection should still apply). |
| **CI** | Workflow **`CI`** (`ci.yml`) runs: tests, static gates, workflow contracts, migration/compose checks, and related jobs. |
| **Security** | **`Security`** (`security.yml`) runs repository security scans (for example vulnerability and secret scans). This is **not** the same as **Security Release** (verdict/artifacts). |
| **No images** | **Build and Push Images** does **not** run from the PR head alone. |

Merge when required checks and reviews pass per branch protection.

---

## 2. Push to `develop` — CI, Build, Security Release, staging

After merge to **`develop`** (or a permitted direct push):

1. **CI** and **Security** (push) run.
2. **Build and Push Images** runs after a **successful** **CI** `workflow_run` for an eligible push to **`develop`**. It publishes **digest-pinned** container image coordinates and build artifacts (for example **image-metadata**, **promotion-manifest**, **release-candidate**).
3. **Security Release** runs when **Build and Push Images** **completes** (`workflow_run`). It consumes build artifacts, runs the release gate, and publishes **`security-verdict`**. Only a **`verdict: pass`** outcome satisfies downstream deploy gates.
4. **Staging Deployment Contract** runs when **Security Release** completes on **`develop`**. With real staging enabled, it can perform a remote deploy, health checks, and smoke, then upload **`staging-deploy-evidence`** (see [staging-preprod-gate.md](staging-preprod-gate.md)).

**Staging evidence** is the **pre-production proof** for promotion: a successful run yields a run id and JSON under `staging-evidence/staging-deploy-evidence.json` inside the **`staging-deploy-evidence`** artifact. Production’s strict gate uses **`promotion_eligible: true`**, **`deployment_mode: real_staging`**, and matching **image digests** (not tags alone) when the strict path is in force.

---

## 3. Promotion: `develop` → `main`

| Step | Notes |
| --- | --- |
| PR merge | Open a **pull request** from **`develop`** into **`main`**. `main` should be protected (reviews, required checks). |
| Rationale | **`main`** is the branch **Deploy Production** is allowed to use (with environment rules). |

After the merge, **CI** and **Security** run on **`main`**, then the same **Build and Push Images** → **Security Release** chain as on **`develop`**, with triggers scoped to **`main`**.

**Security Release** on `main` may also emit additional production-candidate material (for example **release-manifest**), depending on workflow version—use the **Security Release** run you intend to reference in **Deploy Production**.

There is still **no automatic production deployment** on merge to `main`.

---

## 4. Manual Deploy Production (after gates)

1. Select a **Build** run id and **Security Release** run id (and, when using the strict staging path, a **staging** run id) that are **consistent** (same digest-pinned `...@sha256:...` for app and goose images as validated by the workflows).
2. In **Actions** → **Deploy Production** → **Run workflow** on **`main`**, set inputs per your organization’s runbook: confirmation flags, `security_release_run_id`, build/candidate selection, digest-pinned **`app_image_ref`** and **`goose_image_ref`**, and **`staging_evidence_id`** when required.
3. The **`production`** **environment** may require **manual approvers** in GitHub **Settings → Environments**; approval is not replaced by a green **CI** alone.

**Automatic production deployment does not exist** in this repository’s design: no upstream workflow is allowed to “fire and forget” production without operator intent and the **`workflow_dispatch`** form.

**Evidence packages** typically include **production-deploy-evidence**, **production-deployment-manifest**, and related paths documented in [release-evidence-retention.md](release-evidence-retention.md), [deploy-monitoring-slo.md](deploy-monitoring-slo.md), and the job summary for the run.

---

## 5. Required run ids and evidence ids (typical)

Use these as **recordkeeping fields** in your change ticket. Replace placeholders with digits from the GitHub Actions URL `…/actions/runs/<ID>`.

| Id / label | What it points to | When it is required |
| --- | --- | --- |
| **CI** / merge | Merge commit on `develop` or `main` | Always |
| **Build run id** | **Build and Push Images** run that produced the **release-candidate** / image digests you deploy | Production deploy |
| **Security Release run id** | Run that published **`verdict: pass`** for those images | Production deploy |
| **Staging evidence run id** | **Staging Deployment Contract** run with **`staging-deploy-evidence`** | Strict production path (unless a documented, noisy bypass is used with reason) |
| **Backup evidence id** | Run id or id from backup drill JSON when `run_migration` and backup policy require it | Migrations with backup policy—see [production-backup-restore-drill.md](production-backup-restore-drill.md) and [backup-evidence-for-production-migrations.md](../runbooks/backup-evidence-for-production-migrations.md) if present |
| **App / goose image digest** | `...@sha256:<64-hex>` | Always for production; immutable coordinates |

**Never** paste GitHub **personal access tokens**, **SSH private keys**, or **webhook secrets** into tickets or these docs. Use run links and artifact names only.

---

## 6. Related documentation

- [ci-cd-enterprise-contract.md](ci-cd-enterprise-contract.md) — what is “enterprise” vs manual vs not guaranteed
- [production-release-checklist.md](production-release-checklist.md) — pre-flight checklist
- [field-rollout-checklist.md](field-rollout-checklist.md) — post-deploy, machine-facing validation
- [two-vps-rolling-production-deploy.md](two-vps-rolling-production-deploy.md) — topology, drain, rollback posture
- [production-smoke-tests.md](production-smoke-tests.md) — public smoke tiers
