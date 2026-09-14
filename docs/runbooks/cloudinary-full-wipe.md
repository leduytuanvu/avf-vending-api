# Cloudinary Full Wipe Runbook

Permanently delete **all** assets in AVF Cloudinary product environment(s). Does not delete Cloudinary account, API keys, or upload presets.

## Prerequisites

- `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY`, `CLOUDINARY_API_SECRET` in env or `--env-file`
- Quiesce API uploads: `MEDIA_UPLOAD_ENABLED=false` and/or stop `api` container
- Evidence directory: `.cloudinary-wipe-evidence/` (gitignored)

## 1. Discover targets

```bash
bash scripts/ops/cloudinary/discover-targets.sh \
  --production-env deployments/prod/app-node/.env.app-node \
  --staging-env deployments/staging/.env.staging \
  --local-env .env.local
```

On VPS: use `/opt/avf-vending-api/deployments/prod/app-node/.env.app-node`.

## 2. Inventory (read-only)

```bash
bash scripts/ops/cloudinary/inventory.sh --env-file deployments/prod/app-node/.env.app-node
```

## 3. Dry-run wipe

```bash
bash scripts/ops/cloudinary/wipe.sh --dry-run --env-file deployments/prod/app-node/.env.app-node
```

## 4. Live wipe

```bash
export CONFIRM_CLOUDINARY_LIVE_DELETE="DELETE-ALL-CLOUDINARY-ASSETS-<cloud_name>"
bash scripts/ops/cloudinary/wipe.sh --delete-live-assets --delete-empty-folders \
  --env-file deployments/prod/app-node/.env.app-node
```

## 5. Verify

```bash
bash scripts/ops/cloudinary/verify-empty.sh --env-file deployments/prod/app-node/.env.app-node
```

## Integration

`run-environment-data-wipe.sh` phase `media` calls `wipe.sh` when Cloudinary is configured.

## Security

Never commit API secrets or evidence files containing credentials.
