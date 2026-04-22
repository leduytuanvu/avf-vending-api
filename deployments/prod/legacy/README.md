# Legacy single-host production assets

These files remain in `deployments/prod/` only for rollback compatibility and operator reference:

- `docker-compose.prod.yml`
- `.env.production.example`
- `scripts/deploy_prod.sh`
- `scripts/update_prod.sh`
- `scripts/rollback_prod.sh`
- `scripts/healthcheck_prod.sh`
- `scripts/release.sh`

Their status is explicit:

- legacy
- not primary production
- not recommended for enterprise target deployment

Use the split layout under `deployments/prod/app-node`, `deployments/prod/data-node`, and `deployments/prod/shared` for the current primary production path.
