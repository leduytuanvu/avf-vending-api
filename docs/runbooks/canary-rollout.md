# Production canary rollout (optional, host-order only)

This repository supports an **optional** `ROLLOUT_MODE=canary` path in **Deploy Production** (`.github/workflows/deploy-prod.yml`). It does **not** implement cloud load-balancer traffic splitting or percentage-based routing. Canary here means **sequential host rollout**: deploy and validate on **one** app host first, then deploy the **other** app host.

## Default: single-host behavior unchanged

- **`ROLLOUT_MODE=single-host`** is the default (repo variable `ROLLOUT_MODE` unset or `single-host`, or workflow dispatch **rollout_mode** left at `single-host`).
- With a **single** production app node (no `APP_NODE_B_HOST` / node B skipped), behavior matches historical **node A → (optional) node B skipped** rollout and **smoke** gates.
- **Rollback** behavior and **automatic rollback** after failure are unchanged for this mode.

## When canary mode is allowed

Canary is **only** valid when **two app nodes** are in play (the workflow does **not** skip app node B for topology reasons):

- **`APP_NODE_B_HOST`** configured, and
- Node B is **not** auto-skipped (e.g. same host as data node without `ALLOW_APP_NODE_ON_DATA_NODE`).

If you set `ROLLOUT_MODE=canary` with only one effective app node, the workflow **fails validation** with a clear error — it does **not** pretend to run a multi-host canary.

## Canary sequence (deploy mode only)

1. **Canary wave** — `release_app_cluster.sh` runs against **one** host first (migrations run here when applicable), with **post-readiness smoke** to `deployment-evidence/smoke-canary-wave.json`.
2. **Optional observability** — If **`PRODUCTION_OBSERVABILITY_CHECK_URL`** (repo variable) is non-empty, a **read-only HTTP GET** runs via `scripts/deploy/observability_http_check.sh`. Failure **blocks** the rest of the rollout (no stable wave).
3. **Optional wait** — **`CANARY_OBSERVATION_WAIT_SEC`** (repo variable, seconds, default `0`) sleeps between canary and stable waves.
4. **Stable wave** — Same images on the **remaining** app host (`RUN_MIGRATION_ON_FIRST_NODE=0`).
5. **Final cluster smoke** and **post-deploy smoke** — Same as single-host mode; failures **fail** the job and trigger **existing** automatic rollback rules when applicable.

### Which host is canary first?

- **Default:** **App node A** is the canary host; **app node B** is stable second.
- **Flip:** Set repo variable **`PRODUCTION_CANARY_IS_APP_NODE_B=true`** so **B** is canary first and **A** is stable second.

There is **no** partial traffic shift: DNS/LB must still point where your operations model requires; this workflow only controls **SSH rollout order**.

## Observability hook (optional)

| Variable | Purpose |
|----------|---------|
| `PRODUCTION_OBSERVABILITY_CHECK_URL` | If set after canary deploy, **GET** this URL (read-only). |
| `PRODUCTION_OBSERVABILITY_CHECK_TIMEOUT_SEC` | Optional timeout (default **15** in workflow). |
| `PRODUCTION_OBSERVABILITY_HEALTHY_SUBSTRING` | If set, response body must contain this literal substring. |
| `PRODUCTION_OBSERVABILITY_UNHEALTHY_BODY_REGEX` | If set, exit **1** when body matches (e.g. unhealthy JSON). |
| `OBSERVABILITY_EXPECT_HTTP_STATUS` | Supported by script; default **200** (set in environment if you extend the step). |

No external SaaS is required: the URL can be your own **health** or **status** endpoint reachable from **GitHub-hosted** runners (or a self-hosted runner if you use one).

## Workflow dispatch

Manual runs can set input **`rollout_mode`** to `single-host` or `canary`. Automatic runs (after **Security Release**) use repo variable **`ROLLOUT_MODE`** only (`single-host` if unset).

## Rollback

- **Canary or observability failure** before the stable wave: only the **canary host** may have received the new images; automatic rollback (when previous digest-pinned refs exist) targets hosts that actually ran deploy steps, **canary then stable** in canary mode.
- **Stable wave or smoke failure** after partial success: rollback uses the same **existing** `rollback_app_node.sh` path and **does not** run `goose down`.
- For manual recovery, use the same commands as in the main production runbook: **`deployments/prod/app-node/scripts/rollback_app_node.sh`** (via SSH on each node) and workflow_dispatch **rollback** mode with digest-pinned refs.

## Infra requirements (summary)

| Requirement | single-host | canary |
|-------------|------------|--------|
| One app node | Supported | **Not supported** (validation error) |
| Two app nodes | A then B (or B skipped) | A→B or B→A per `PRODUCTION_CANARY_IS_APP_NODE_B` |
| Observability URL | Ignored if unset | Optional gate after canary |
| Traffic shifting | N/A | **Not implemented** — document LB/DNS separately |

See also: `scripts/deploy/observability_http_check.sh`, `deployments/prod/shared/scripts/release_app_cluster.sh`, and the **Production Deployment** job summary in Actions for per-run rollout mode and outcomes.
