# Production: GHCR image-only deploy and rollback

This runbook matches `deployments/prod/docker-compose.prod.yml` (image-only, no `build:` on the VPS).

## Image tags and precedence

Compose pulls:

- Application services: `${IMAGE_REGISTRY}/${APP_IMAGE_REPOSITORY}:${APP_IMAGE_TAG}`
- Migrate: `${IMAGE_REGISTRY}/${GOOSE_IMAGE_REPOSITORY}:${GOOSE_IMAGE_TAG}`

`deployments/prod/scripts/release.sh`, `deploy_prod.sh`, and `update_prod.sh` resolve tags as follows:

1. Use explicit `APP_IMAGE_TAG` and `GOOSE_IMAGE_TAG` in `.env.production` when you run `release.sh deploy` with **no arguments** (re-apply whatever is in the file).
2. If `APP_IMAGE_TAG` is missing, fall back to legacy `IMAGE_TAG` for the app image tag.
3. If `GOOSE_IMAGE_TAG` is missing, fall back to `IMAGE_TAG`, then to the resolved app tag.

`IMAGE_TAG` does **not** have to equal `APP_IMAGE_TAG`; it is optional legacy input only.

## Typical rollout (VPS)

From `deployments/prod` (or with paths adjusted):

```bash
# Optional: GHCR login for private images
export GHCR_PULL_USERNAME=...
export GHCR_PULL_TOKEN=...

# One immutable tag for both app + goose (common)
bash scripts/release.sh deploy sha-abc123def456

# Or split tags (e.g. hotfix app only)
bash scripts/release.sh deploy sha-app sha-goose
```

Equivalent wrappers:

```bash
bash scripts/deploy_prod.sh sha-abc123def456
bash scripts/update_prod.sh
```

## What `release.sh deploy` does (order)

1. Validates compose env and writes registry + tag fields to `.env.production`.
2. `docker compose pull` for migrate + app images.
3. Starts Postgres, NATS, EMQX; runs **migrate** once; EMQX bootstrap.
4. `docker compose up -d` for the long-lived stack; polls container readiness (running; healthy when a Docker healthcheck exists — see `ROLLUP_HEALTH_*` in `release.sh`).
5. Runs `scripts/healthcheck_prod.sh` unless `SKIP_SMOKE=1`.
6. Records current/previous app and goose tags under `.deploy/` for rollback.

## Rollback

```bash
# Restore last recorded previous app/goose tags from .deploy/
bash scripts/release.sh rollback

# Or set explicit tags
bash scripts/release.sh rollback sha-good-app sha-good-goose
```

`rollback_prod.sh` delegates to `release.sh rollback`.

## Compose validation

```bash
docker compose --env-file deployments/prod/.env.production -f deployments/prod/docker-compose.prod.yml config
```

## Related

- Operator-facing detail: [deployments/prod/README.md](../../deployments/prod/README.md)
