# Production rollback (AVF vending API)

Production rollbacks are **image repins** only: they redeploy digest-pinned **app** and **goose** containers to a known-good pair. They **do not** run `goose down`, reverse SQL migrations, or otherwise mutate the database schema automatically.

## Sources of truth

| Artifact / doc | Purpose |
| --- | --- |
| Workflow **Deploy Production** (`.github/workflows/deploy-prod.yml`) | Canonical automation for deploy and rollback modes |
| Artifact **`production-deployment-manifest`** | Last-known-good (LKG) record after every production run (success or failure); used to resolve **previous** digest-pinned refs |
| This runbook | Operator procedure when automation or cluster state is unclear |

## Last-known-good manifest (`production-deployment-manifest.json`)

Written on **every** workflow conclusion (`if: always()`), so a failed deploy still records what was attempted, `rollback_attempted`, and `rollback_result`.

Fields operators rely on:

- **`app_image_ref`**, **`goose_image_ref`** — digest-pinned refs promoted in this run (when successful, this row becomes the next deploy’s “previous” LKG).
- **`source_commit_sha`**, **`release_tag`**, **`deployed_at_utc`** / **`completed_at_utc`**, **`run_id`**, **`run_url`** — audit trail.
- **`rollback_available_before_deploy`** — whether this run started with resolvable, digest-pinned **previous** refs (enables automatic rollback if deploy fails mid-flight).
- **`previous_*`** — snapshot of the LKG **before** this run (rollback target for auto-rollback).
- **`rollback_attempted`**, **`rollback_result`** — `not-attempted` \| `not-started` \| `no-previous-release` \| `nothing-to-rollback` \| `completed` \| `failed`.
- **`migration_rollback_policy`**: `never_automatic` — schema is not reverted by this workflow.
- **`auto_rollback_scope`**: `app_and_goose_images_only` — containers only; see **`auto_rollback_note`** in the JSON.

Manifest **`schema_version`** is incremented only when JSON shape changes in a breaking way.

## Before deploying

1. Open the **Resolve Previous Production Deployment** job on the run you are about to execute. It prints whether **`rollback_available`** is true and copies **manual rollback** workflow inputs (digest-pinned).
2. If **`rollback_available`** is false, a failed deploy **cannot** auto-revert live nodes. Mitigate before risky changes: complete a clean deploy to publish a manifest, or archive refs externally (registry + digests + `release_tag`).
3. All production image refs must contain **`@sha256:`** and must not use **`latest`** (enforced in workflow and `deployments/prod/shared/scripts/validate_digest_pinned_image_refs.sh`).

## Automatic rollback (failed deploy)

When **`action_mode=deploy`**, rollout has passed **Mark production release start**, and a later step fails (readiness, smoke, second node, etc.):

1. If **`PREVIOUS_APP_IMAGE_REF`** and **`PREVIOUS_GOOSE_IMAGE_REF`** were resolved from a prior successful manifest and are digest-pinned, the workflow runs **`rollback_app_node.sh`** on affected hosts with explicit refs and **`RUN_MIGRATION=0`**.
2. If those refs are missing, the rollback step **fails with a clear error** — it does **not** claim success.
3. If rollback commands fail (SSH, compose, healthcheck), **`rollback_result=failed`** and the step exits unsuccessfully; **verify live cluster state** before assuming safety.

**Healthchecks** after rollback use the same **`healthcheck_app_node.sh`** path as a normal release (no bypass).

## Manual rollback (workflow_dispatch)

1. In **Deploy Production**, set **`action_mode=rollback`**.
2. Set **`rollback_app_image_ref`** and **`rollback_goose_image_ref`** to the **exact** digest-pinned strings from the last good manifest (or the **Resolve Previous Production Deployment** summary).
3. Leave **`build_run_id`** empty; do not pass deploy-only confirmation fields.
4. Approve the **production** environment as usual.

## After a successful deploy

- Download and archive the **`production-deployment-manifest`** artifact (default retention **90** days in-repo).
- Optionally mirror digest + `release_tag` + `source_commit_sha` to your change-management system so rollback does not depend on GitHub artifact TTL alone.

## Operational warnings

- If **migrations already ran** on production before failure, rolling back **images** can leave schema ahead of old binaries — treat as an incident; do not rely on image rollback alone without compatibility analysis.
- **Data-node** rollback is separate from app-node rollback; automatic data-node rollback only runs when this workflow requested a data-node deploy in the same run.

## Related

- [production-2-vps.md](./production-2-vps.md) — topology and deploy root
- [production-cutover-rollback.md](./production-cutover-rollback.md) — broader cutover and rollback context
