# Two-VPS rolling production deploy

This document describes the **two application VPS** production topology, how the GitHub Actions `Deploy Production` workflow rolls out changes, what “zero downtime” means in evidence, and how to operate rollbacks.

## Required secrets and variables

Configure these in the GitHub **environment** (or org/repo secrets/variables) as referenced by [`.github/workflows/deploy-prod.yml`](../../.github/workflows/deploy-prod.yml).

| Name | Role |
| --- | --- |
| `PRODUCTION_APP_NODE_A_HOST` (or `PROD_APP_A_HOST` / `APP_A_HOST` via vars) | SSH target for **node A** (first in rolling order). |
| `PRODUCTION_APP_NODE_B_HOST` (or equivalent) | SSH target for **node B** (second; skipped only with `PRODUCTION_ALLOW_SINGLE_APP_NODE=1`). |
| `PRODUCTION_SSH_USER`, `PRODUCTION_SSH_PRIVATE_KEY`, `PRODUCTION_SSH_KNOWN_HOSTS` (or fallbacks) | Non-interactive SSH; host keys are validated before deploy. |
| `PRODUCTION_DEPLOY_ROOT` (or `DEPLOY_PATH` / `PROD_DEPLOY_PATH`) | Remote checkout path (default `/opt/avf-vending-api`). |
| `PRODUCTION_PUBLIC_BASE_URL` / `PRODUCTION_API_DOMAIN` | Used for public **final** smoke after both nodes. |
| `TRAFFIC_DRAIN_MODE` (repo var, default `none`) | `none` \| `caddy` \| `external-lb` — see [Traffic drain](#traffic-drain). |
| `TRAFFIC_DRAIN_WAIT_SECONDS` | Optional post-hook wait (integer seconds). |
| `PRODUCTION_ALLOW_SINGLE_APP_NODE` | Set to `1` only if you **intentionally** run a single app node (not two-VPS). |
| `ALLOW_IDENTICAL_PRODUCTION_APP_NODES` | Set to `1` only for **documented** dry-runs when A and B point at the same host. |

Build/security inputs (`security_release_run_id`, `build_run_id`, staging evidence, `DEPLOY_PRODUCTION` confirmation) and digest-pinned images are unchanged; see the workflow and [staging pre-prod gate](staging-preprod-gate.md).

## Traffic topology assumptions

- **CI runs steps in order:** sync → deploy **A** → smoke A → (only if A succeeded) deploy **B** → smoke B → final public smoke. There is **no** matrix/parallel job that restarts both app nodes at once.
- **Node-local edge:** If the stack uses Caddy on each VPS, stopping Caddy on one node is a **local** traffic pause; the other node can still serve the **public** name **if** DNS/LB send traffic to both backends. The workflow does not assume a specific cloud load balancer unless you use `TRAFFIC_DRAIN_MODE=external-lb` and provide a host hook.
- **Machines and sessions:** Vending clients should use the public API hostname (or pool), not a hardcoded single-node IP, so one node can be drained while the other serves. Validate this periodically (see [Validating that machines are not pinned to one node](#validating-that-machines-are-not-pinned-to-one-node)).

## What “zero downtime” means (and does not mean)

- **`TRAFFIC_DRAIN_MODE=none`:** Evidence records **`zero_downtime_claim: false`**. The deploy may still be successful; it does **not** assert global or LB-level draining.
- **`TRAFFIC_DRAIN_MODE=caddy`:** The hook documents **node-local** behavior: the real drain step is the subsequent `docker compose stop caddy` in `release_app_node.sh`. Evidence may set **`zero_downtime_claim: true`** only in the **node-scope** sense used by the release scripts (stopping the edge on that node in a controlled order), **not** a guarantee that every client worldwide sees no blip.
- **`TRAFFIC_DRAIN_MODE=external-lb`:** You must set **`TRAFFIC_DRAIN_EXTERNAL_SCRIPT`** on the host to an executable operator script. The hook **fails fast** if the hook is missing; the system does not fake a successful drain.

The workflow summary and `production-deploy-evidence.json` **must** be taken as the source of truth for whether a rolling deploy completed, was partial, or rolled back (`rollout_outcome_summary`, per-node results, `final_public_smoke_result`).

## Traffic drain

| Mode | Behavior |
| --- | --- |
| `none` | No pre-stop LB hook; `zero_downtime_claim` remains false for global/LB scope. |
| `caddy` | Documents that the in-compose caddy stop is the node-local drain; see [`traffic_drain_hook.sh`](../../deployments/prod/shared/scripts/traffic_drain_hook.sh). |
| `external-lb` | Runs `TRAFFIC_DRAIN_EXTERNAL_SCRIPT` on the app node; exits non-zero if not configured. |

## Validating that vending machines are not pinned to one node

1. **Configuration review:** Client configs should resolve the same **API base URL** that matches your public endpoint (or pool), not a raw IP of a single VPS.
2. **During a controlled maintenance window:** With both nodes up, **stop Caddy (or the app) on node A only** and confirm a sample of devices still connect via the public URL (served from B). Repeat for B. If clients fail when only one node is up, the fleet may be pinned or DNS/LB is not distributing.
3. **Observability:** Compare access logs or connection counts per node around deploy windows; a healthy split (modulo short transitions) is expected for two healthy backends.

## Rollback playbook

Automatic rollback in the workflow is **selective** to limit blast radius (digest-pinned previous images; no `goose down`):

- **Node A deploy fails:** Roll back **A only**; **B is not deployed** in that run.
- **Node A OK, node B deploy fails:** Roll back **B**; node A may remain on the new version; evidence records `failed_at_node_B_node_A_on_new_rollback_B_eligible`.
- **Final public smoke fails** after a successful B step: the workflow’s auto-rollback job decides scope from outcomes (e.g. both nodes or the minimal set per policy); see `release-events.jsonl` and `rollout_outcome_summary`.

**Manual recovery:** If automatic rollback does not complete, use the same digest-pinned images and [`rollback_prod.sh`](../../deployments/prod/scripts/rollback_prod.sh) / remote `rollback_app_node.sh` as in [production backup/restore](production-backup-restore-drill.md) runbooks, and do **not** assume DB state reverted (migrations are not auto-reversed).

## Related files

- [`deployments/prod/shared/scripts/traffic_drain_hook.sh`](../../deployments/prod/shared/scripts/traffic_drain_hook.sh)
- [`deployments/prod/shared/scripts/release_app_cluster.sh`](../../deployments/prod/shared/scripts/release_app_cluster.sh)
- [`production-smoke-tests.md`](production-smoke-tests.md) (tiered health + business-readonly + optional synthetic)
- [`production-backup-restore-drill.md`](production-backup-restore-drill.md) when `run_migration=true`
