# Production release checklist (operator)

Use this list **before** clicking **Run workflow** on **Deploy Production** (`deploy-prod.yml` on `main`). It does not replace GitHub environment approvals or your organization’s change-management policy.

**Production deployment is manual only** (`workflow_dispatch` on `main`). A green merge or a green **Security Release** alone does not deploy to production.

**Two-VPS note:** a successful rolling job in CI does **not** by itself prove global zero downtime. Confirm traffic drain / load-balancer behavior per [two-vps-rolling-production-deploy.md](two-vps-rolling-production-deploy.md).

**Priority tags (documentation only — aligns with `[production-release-readiness.md](../runbooks/production-release-readiness.md)` tiers):**

| Tag | Meaning |
| --- | ------- |
| **P0** | Must pass before deploy proceeds (safety gates). |
| **P1** | Required for normal release sign-off before production approval step. |

**Evidence & owner:** For **each** subsection below, the change ticket must list **Approver / owner**, **Evidence** (workflow run URL, backup id, staging run id, digest lines), and **UTC timestamp**. Do not paste secrets.

Record **run ids and names** in the change ticket; do **not** record secrets, tokens, or private keys.

---

## Governance and access

- [ ] **Branch protection** on `main` is confirmed (required reviews, required checks, and merge rules match policy).
- [ ] **`production` environment** has **required reviewers** (or equivalent rulesets) and is limited to the intended users.
- [ ] The operator running deploy has **authorization** for production change in your org (separate from GitHub **admin** rights).
- [ ] Client teams (Android + field) acknowledged **[`../architecture/production-final-contract.md`](../architecture/production-final-contract.md)** for this release (gRPC + MQTT + no legacy machine HTTP in prod unless waived).

## CI and build health

- [ ] **CI** is green for the commit you are promoting (or the merge commit you are deploying), per your policy.
- [ ] **Build and Push Images** has completed successfully for that line of commits and has produced the **digest-pinned** app and goose images you will reference.
- [ ] **Security Release** for the same build chain shows **`verdict: pass`** in **`security-verdict`**, with **`source_branch: main`** as required for production. Non-pass verdicts are **not** deployable.

## Staging and evidence

- [ ] **Staging evidence run id** is recorded: a successful **Staging Deployment Contract** run when using the **strict** gate (`promotion_eligible: true`, `deployment_mode: real_staging`, digest match). See [staging-preprod-gate.md](staging-preprod-gate.md).
- [ ] If your process allows **missing staging evidence** only in exception cases, the bypass inputs are set with a **non-empty business reason** and the exception is **approved** (not the default path).

## Migrations and data safety

- [ ] **`run_migration`** decision matches the change (image-only vs schema change).
- [ ] If migrations run, **backup evidence id** (or other required backup proof) is on hand per [production-backup-restore-drill.md](production-backup-restore-drill.md) and [backup-evidence-for-production-migrations.md](../runbooks/backup-evidence-for-production-migrations.md) if present.
- [ ] Data migration review has followed [production-data-migration-safety.md](production-data-migration-safety.md), including replay/idempotency checks for outbox, payment webhooks, and machine offline events.
- [ ] DR readiness context is known: `APP_REGION`, `APP_NODE_NAME`, and `APP_INSTANCE_ID` values for every production process match [multi-region-dr-readiness.md](../runbooks/multi-region-dr-readiness.md).

## Images and rollback

- [ ] **App image digest** and **goose image digest** are the immutable `...@sha256:…` values validated by **Security Release** and staging (when used)—not a floating tag.
- [ ] **Rollback target** (previous known-good app/goose digests) is **known** from a prior **production-deployment-manifest** or approved incident workflow, in case automatic or manual rollback is required. Image rollback does **not** reverse database migrations.

## Smoke and observability

- [ ] **Smoke** inputs for production are understood: [production-smoke-tests.md](production-smoke-tests.md) (tiering, read-only business probes, optional synthetic GET when enabled).
- [ ] **SLO / deploy evidence** expectations are understood: [deploy-monitoring-slo.md](deploy-monitoring-slo.md) (pre/post JSON artifacts; optional SSH snapshots).

## Final approval

- [ ] **Operator approval** (change ticket / CAB / on-call sign-off) is complete **before** the **`production` environment** approval in GitHub, or in the order your org requires.
- [ ] **Deploy Production** form inputs (build id, security release id, staging id, image refs, confirmation flags) are **double-checked** against the artifacts, not retyped from memory.

---

## After deploy

Use [field-rollout-checklist.md](field-rollout-checklist.md) for **machine- and field-level** checks. Populate **[`../testing/field-test-cases.md`](../testing/field-test-cases.md)** rows for pilots; use [field-pilot-checklist.md](field-pilot-checklist.md) for **before / during / after** pilot sequencing. **`P2` fleet readiness** (storm JSON, expanded field matrix for 100–1000) is governed by **[`../runbooks/production-release-readiness.md`](../runbooks/production-release-readiness.md)** — not by this checklist alone.

For rollback steps, see [../runbooks/production-rollback.md](../runbooks/production-rollback.md), [../runbooks/production-cutover-rollback.md](../runbooks/production-cutover-rollback.md) (2-VPS topology), and [two-vps-rolling-production-deploy.md](two-vps-rolling-production-deploy.md).
