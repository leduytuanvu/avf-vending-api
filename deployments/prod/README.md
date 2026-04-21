# Production Deploy Runbook

This production profile is for the existing `avf-vending-api` backend on a single Ubuntu VPS. The VPS does not build Go binaries or Docker images. It only pulls prebuilt immutable images and runs `docker compose`.

Current supported mode:
- GitHub Actions -> VPS sync -> SSH -> `release.sh`
- static SSH credentials remain the active production path today

## Release inputs

Required:
- `.env.production`
- `APP_IMAGE_REF`
- `GOOSE_IMAGE_REF`

Operator prerequisite:
- `EMQX_API_KEY` / `EMQX_API_SECRET` must already belong to a REST API key created in EMQX (for example via `System > API Key` in the dashboard). Production deploys use that pre-provisioned key for `/api/v5/*`; they do not create or sync EMQX API keys from files in this repo.

Preferred image format:
- `ghcr.io/<owner>/<repo>@sha256:<digest>`
- `ghcr.io/<owner>/<repo>-goose@sha256:<digest>`

Optional operator label:
- `RELEASE_LABEL=v1.2.3`

## Deploy flow

GitHub Actions production deploy:
1. A successful `Build and Push Images` run from `main` now auto-triggers `deploy-prod.yml` through `workflow_run`.
2. The workflow resolves digest-pinned image refs from the build artifact, records promotion metadata, then syncs deploy assets to the VPS and runs `release.sh deploy`.
3. The release path is: `validate -> backup -> migrate -> deploy -> verify`.
4. If post-deploy verify fails, the workflow attempts one automatic image rollback and then marks the run failed.
5. Manual `workflow_dispatch` remains available when an operator needs to promote explicit refs.
6. Manual runs still require `release_tag`, `app_image_ref`, and `goose_image_ref`; set `source_commit_sha` too when promoting a digest built from a different commit than the workflow ref.

Manual VPS deploy:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
RELEASE_LABEL=v1.2.3 bash scripts/release.sh deploy ghcr.io/<owner>/<repo>@sha256:<app-digest> ghcr.io/<owner>/<repo>-goose@sha256:<goose-digest>
```

Thin wrappers remain available:

```bash
bash scripts/deploy_prod.sh
bash scripts/update_prod.sh
```

Both wrappers now resolve `APP_IMAGE_REF` / `GOOSE_IMAGE_REF` first and only fall back to legacy tag fields when needed.

Important behavior:
- deploys consume immutable image refs
- `latest` is not a valid production source
- migrations run before the long-lived app stack is considered healthy
- image rollback does not roll back database schema

## Rollback flow

Preferred rollback:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/release.sh rollback
```

Explicit rollback:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/release.sh rollback ghcr.io/<owner>/<repo>@sha256:<known-good-app> ghcr.io/<owner>/<repo>-goose@sha256:<known-good-goose>
```

Rollback behavior:
- restores the last known good image refs from `.deploy/last_known_good_*` when no refs are provided
- re-pulls the selected images
- brings the stack back up
- runs `healthcheck_prod.sh` unless `SKIP_SMOKE=1`
- does not attempt database schema downgrade

## Post-deploy verification checklist

Run from `deployments/prod`:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml ps
bash scripts/healthcheck_prod.sh
```

Quick checks:
- confirm `api`, `worker`, `mqtt-ingest`, `reconciler`, `postgres`, `nats`, `emqx`, and `caddy` are running
- confirm `/health/live` and `/health/ready` pass
- confirm the workflow run summary shows the expected image refs and trigger
- confirm the workflow artifact contains the deployment manifest for audit/review

## Troubleshooting

Start here:
- `bash scripts/release.sh status`
- `bash scripts/release.sh logs`
- `docker compose --env-file .env.production -f docker-compose.prod.yml ps`

Common failure areas:
- `.env.production` missing or still contains placeholders
- invalid or unavailable GHCR image ref
- Postgres not ready, causing migrate or readiness failure
- EMQX API credentials do not match the pre-provisioned EMQX REST API key
- DNS / TLS / upstream firewall issues causing public HTTPS healthcheck failures

Recovery guidance:
- if deploy verify fails in GitHub Actions, inspect the job summary first, then the deploy step logs
- if rollback also fails, inspect `.deploy/` state plus `release.sh logs`
- if the problem is schema/data related, use backup/restore or a forward-fix migration; do not rely on image rollback alone

## Future cloud-ready notes

The current production path is intentionally still VPS/SSH based.

The workflow is now structured with a dedicated production promotion context before the active deploy steps:
- current deploy transport: `vps-ssh`
- current auth mode: `static-ssh-key`
- current provenance mode: report-only placeholder

Intended future extension point:
- replace or extend the promotion context in `deploy-prod.yml`
- add real provenance / attestation verification before `Sync production runtime assets`
- later swap static SSH or registry secrets for OIDC/cloud identity without rewriting the deploy/summary flow

What is not implemented yet:
- no cloud deploy target
- no live OIDC credential exchange
- no blocking provenance verification gate
