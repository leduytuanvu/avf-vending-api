# Deployment layout by environment

| Environment | Path / artifact | Config on server |
|-------------|-----------------|------------------|
| **Local** | `deployments/docker/docker-compose.yml` | Copy [`.env.local.example`](../../.env.local.example) to `.`env locally (gitignored) |
| **Staging** | `deployments/staging/**` (compose, Caddy, EMQX, scripts) | `deployments/staging/.env.staging` (from secrets; not committed) — see [`.env.staging.example`](../../.env.staging.example) in repo root |
| **Production** | `deployments/prod/**` (legacy single-host, `app-node/`, `data-node/`, shared) | `deployments/prod/.env.production` and/or `deployments/prod/app-node/.env.app-node` + `.../data-node/.env.data-node` (see `*.example` files) |

Narrative runbooks: [environment-strategy.md](../runbooks/environment-strategy.md), [local-dev.md](../runbooks/local-dev.md), [staging-release.md](../runbooks/staging-release.md).
