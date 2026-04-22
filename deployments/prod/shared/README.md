# Shared production deployment assets

This folder holds assets shared by the primary `deployments/prod/app-node` and `deployments/prod/data-node` production layouts.

Contents:

- `Caddyfile`: shared reverse-proxy config for the app-node stack.
- `env-matrix.md`: which variables belong on app nodes vs the fallback data node.

The legacy single-VPS stack stays at `deployments/prod/docker-compose.prod.yml` for rollback compatibility only. It is not the primary production path and is not recommended for the enterprise target deployment.
