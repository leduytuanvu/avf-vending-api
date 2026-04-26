# Production deploy SLO / monitoring evidence

This document describes what the production workflow records for **basic observability** during a release, how to interpret it, and how to extend monitoring later.

## What is collected

The script `scripts/monitoring/collect_deploy_slo_evidence.sh` runs on the GitHub Actions runner (not on paid APM) and writes **one JSON file per phase** under `deployment-evidence/`:

| File | Phase | When |
|------|--------|------|
| `slo-pre-deploy.json` | `pre_deploy` | After release start, before data-node / app-node rollouts |
| `slo-post-node-a.json` | `post_node_a` | After app node A rollout (pass or fail) |
| `slo-post-final.json` | `post_final` | After the final public smoke step (pass or fail) |

Each file is a schema-versioned JSON object that may include:

### Required vs optional

- **Critical (can fail the pre-deploy step)**  
  When `DEPLOY_SLO_CRITICAL=1` and a public `BASE_URL` is configured, **HTTP 2xx** is required for:
  - `/health/ready` (readiness)
  - `/health/live` (liveness)  
  Missing or non-2xx responses fail the **pre-deploy** SLO step so the release does not proceed on an already-unhealthy public endpoint.

- **Optional (never faked; marked unavailable if missing)**  
  - **Response-time sample**: second `curl` to the same public URLs (seconds, best effort).  
  - **`/version`**: Truncated body for build metadata when exposed.  
  - **Smoke JSON references**: Paths and parsed `overall_status` from:
    - `deployment-evidence/smoke-app-node-a.json`
    - `deployment-evidence/smoke-cluster-final.json`  
    Phase rules avoid claiming a final public smoke “pass” before that smoke has run; missing files are `unavailable` / `not_run` with a note, not a fake pass.
  - **Per-VPS (SSH)**: When `PRODUCTION_DEPLOY_ROOT`, `SSH_USER`, and hosts are set, the script may SSH to each app node and record (best effort, capped):
    - `docker compose -f docker-compose.app-node.yml ps` (head)
    - `df` / `free` (or `/proc/meminfo` fallback)
    - Count of `ERROR`/`FATAL` lines in the last 30 minutes of `api` service logs (grep count only — **no log body** in JSON, to avoid leaking secrets)

If SSH is not configured or a command errors, the JSON records `status: unavailable` and a short note — the workflow **does not** treat that as success.

## Manual checks during a release

Use this as an operator checklist alongside `docs/operations/two-vps-rolling-production-deploy.md` and `docs/operations/production-smoke-tests.md`:

1. Confirm **Security Release** and **staging evidence** gates are green before production.
2. Watch the **pre-deploy SLO** step: if it fails, the public API was not healthy/ready; investigate traffic, DNS, TLS, and edge (Caddy) before retrying.
3. After each app node, confirm **readiness** and **per-node smoke** in the job log; open `slo-post-node-a.json` in the **production-deploy-evidence** artifact for container and host snapshots.
4. After **final public smoke**, review `slo-post-final.json` and `smoke-cluster-final.json` together.

## Wiring external monitoring later

This mechanism is **evidence in the release artifact**; it does not replace a full observability stack. To integrate APM, uptime SaaS, or log analytics:

- **Keep** this script for portable, no-account evidence in CI.
- **Add** synthetic checks or dashboards in your provider that hit the same public `/health/ready` and smokes, and **link** the provider’s incident/alert ID in the release ticket or in a future manifest field (behind a repo convention).
- **Avoid** embedding API keys in the repo; use GitHub **secrets** for any authenticated probes and **never** print them in workflow logs.
- For **Datadog / Grafana Cloud / GKE**-style paid offerings, the workflow stays unchanged; attach those systems’ screenshots or **permalink exports** to the change record.

## Run locally

```bash
export BASE_URL="https://api.example.com"
export DEPLOY_SLO_CRITICAL=0
export PRODUCTION_DEPLOY_ROOT="/opt/avf-vending-api"
export SSH_USER="deploy"
export APP_NODE_A_HOST="10.0.0.1"
# Optional: export SSH_PORT / SSH_OPTS for non-default SSH
bash scripts/monitoring/collect_deploy_slo_evidence.sh --json --phase pre_deploy | jq .
```

## Evidence manifest

`production-deploy-evidence.json` (artifact **production-deploy-evidence**) includes a `deploy_slo_evidence` object with the three relative paths above so audit packages can find them in one place.
