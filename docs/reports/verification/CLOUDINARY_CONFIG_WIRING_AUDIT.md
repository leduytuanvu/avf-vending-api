# Cloudinary Config Wiring Audit

Date: 2026-05-20  
Branch: `fix/cloudinary-config-wiring`

## Config struct / env vars

| Env var | Config field | Notes |
|---------|--------------|-------|
| `MEDIA_PROVIDER` | `MediaUploadConfig.Provider` | Must be `cloudinary` |
| `MEDIA_UPLOAD_ENABLED` | `MediaUploadConfig.Enabled` | Auto-true when provider=cloudinary |
| `CLOUDINARY_CLOUD_NAME` | `CloudinaryConfig.CloudName` | Public |
| `CLOUDINARY_API_KEY` | `CloudinaryConfig.APIKey` | Server-side only |
| `CLOUDINARY_API_SECRET` | `CloudinaryConfig.APISecret` | **Never logged or exposed** |
| `CLOUDINARY_FOLDER` | `CloudinaryConfig.Folder` | Default `avf-vending/products` |
| `MEDIA_MAX_IMAGE_SIZE_MB` | `MediaUploadConfig.MaxBytes` | Default 5 |
| `MEDIA_ALLOWED_IMAGE_TYPES` | `MediaUploadConfig.AllowedTypes` | CSV MIME list |

`MediaUploadConfig.CloudinaryConfigured()` is true only when enabled, provider=cloudinary, and all three credentials are non-empty.

## API bootstrap

- `internal/bootstrap/api.go` creates `platformcloudinary.NewUploader` when `CloudinaryConfigured()`.
- `api.NewHTTPApplication` wires `MediaAdmin` when artifacts, external URLs, **or** Cloudinary is configured.
- Panics at startup if Cloudinary is configured but uploader is nil (mis-wiring).

## Routes

| Route | Mounted | Capability when configured |
|-------|---------|----------------------------|
| `POST /v1/admin/product-images` | Yes | 201 upload |
| `POST /v1/admin/media/product-images` | Yes (alias) | 201 upload |
| `POST /v1/admin/media/external-images` | Yes | Requires `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED=true` (no object storage) |
| `POST /v1/admin/media/uploads/init` | Yes | Requires S3/object storage |

When Cloudinary creds missing: routes stay registered; upload returns **503** `capability_not_configured` / `v1.admin.media`.

## Production env path

- Runtime: `deployments/prod/app-node/.env.app-node` on each app VPS (gitignored).
- Template: `deployments/prod/app-node/.env.app-node.example` (placeholders only).
- CI sync: `deployments/prod/shared/scripts/apply_cloudinary_app_node_env.sh` (SSH during deploy-prod when secrets set).
- Preflight: `deployments/prod/shared/scripts/validate_cloudinary_deploy_secrets.sh`.

## GitHub Actions

Repository secrets (no values in repo):

- `CLOUDINARY_CLOUD_NAME`
- `CLOUDINARY_API_KEY`
- `CLOUDINARY_API_SECRET`

Optional org/repo var: `PRODUCTION_SYNC_CLOUDINARY_ENV=1` to require secrets and sync on deploy.

## Local dev

- Ignored file: `.env.local` (cloud name + API key OK; secret pasted locally only).

## Missing pieces fixed in this branch

1. Deploy scripts to validate and apply Cloudinary env without logging secrets.
2. `deploy-prod.yml` steps: validate secrets + SSH apply to app nodes.
3. Tracked env examples use placeholders (`your-cloudinary-*`), not operator credentials.
4. `.env.local` helper with operator cloud name/key + secret placeholder.
5. Tests for config + HTTP capability when Cloudinary configured vs missing.
6. Postman generator saves `displayUrl` / `thumbnailUrl` from upload response.

## Test plan

- `go test ./internal/config/... ./internal/app/mediaadmin/... ./internal/httpserver/... ./internal/app/api/...`
- Secret scan excludes `.env.local`
- Postman JSON parse validation
- Manual: set `.env.local`, start API, upload via Postman, verify Cloudinary URL opens
