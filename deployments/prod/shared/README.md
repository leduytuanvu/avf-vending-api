# Shared production deployment assets

This folder holds assets shared by the primary `deployments/prod/app-node` and `deployments/prod/data-node` production layouts.

Contents:

- `Caddyfile`: shared reverse-proxy config for the app-node stack.
- `env-matrix.md`: which variables belong on app nodes vs the fallback data node.
- `scripts/render_rollout_env.sh`: copy `app-node/.env.app-node.example` to a new local env file (refuses to overwrite).
- `scripts/validate_production_deploy_inputs.sh`: topology / DB pool guardrails (`COLOCATE_APP_WITH_DATA_NODE`, `ALLOW_APP_NODE_ON_DATA_NODE`, `ENABLE_APP_NODE_B`).

The legacy single-VPS stack stays at `deployments/prod/docker-compose.prod.yml` for rollback compatibility only. It is not the primary production path and is not recommended for the enterprise target deployment.
