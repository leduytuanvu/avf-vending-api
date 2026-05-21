# Cloudinary Media Setup (Production)

## 1. Create Cloudinary account

Sign up for the [Cloudinary free tier](https://cloudinary.com/).

## 2. Copy API credentials

In Cloudinary Console → **Settings → API Keys**, copy:

- Cloud name
- API key
- API secret (**server-side only — never commit**)

## 3. GitHub Actions secrets

### GitHub CLI

```bash
gh secret set CLOUDINARY_CLOUD_NAME --body "dz4qz0tk9"
gh secret set CLOUDINARY_API_KEY --body "462269529388452"
gh secret set CLOUDINARY_API_SECRET
```

For `CLOUDINARY_API_SECRET`, run the command **without** `--body`. Your shell prompts for the value; paste the real secret from Cloudinary Console and confirm. The secret is stored in GitHub only — it is not echoed to the terminal or written to the repo.

### GitHub UI

Repository → **Settings → Secrets and variables → Actions → New repository secret**

| Secret name | Value |
|-------------|-------|
| `CLOUDINARY_CLOUD_NAME` | Your cloud name (e.g. `dz4qz0tk9`) |
| `CLOUDINARY_API_KEY` | Your API key |
| `CLOUDINARY_API_SECRET` | Your API secret (paste once; never document the value) |

Optional repository variable:

- `PRODUCTION_SYNC_CLOUDINARY_ENV=1` — fail deploy preflight if Cloudinary secrets are missing (recommended once secrets are set).

## 4. Production runtime env

During `deploy-prod`, when all three GitHub secrets are present, CI runs `apply_cloudinary_app_node_env.sh` over SSH on each app node and patches `deployments/prod/app-node/.env.app-node` (gitignored on the VPS).

Manual equivalent on the app VPS:

```env
MEDIA_PROVIDER=cloudinary
MEDIA_UPLOAD_ENABLED=true
CLOUDINARY_CLOUD_NAME=<from secret>
CLOUDINARY_API_KEY=<from secret>
CLOUDINARY_API_SECRET=<from secret>
CLOUDINARY_FOLDER=avf-vending/products
MEDIA_MAX_IMAGE_SIZE_MB=5
MEDIA_ALLOWED_IMAGE_TYPES=image/jpeg,image/png,image/webp,image/gif
```

Do **not** set `CLOUDINARY_API_SECRET` in Postman, mobile app, or public docs.

## 5. Local development

Copy `.env.local.example` to `.env.local` (already gitignored) or use the repo’s local helper:

```env
MEDIA_PROVIDER=cloudinary
MEDIA_UPLOAD_ENABLED=true
CLOUDINARY_CLOUD_NAME=dz4qz0tk9
CLOUDINARY_API_KEY=462269529388452
CLOUDINARY_API_SECRET=<paste real secret locally only>
CLOUDINARY_FOLDER=avf-vending/products
```

Never commit `.env.local` or the real API secret.

## 6. Deploy

Deploy API as usual. Health endpoints remain **200** when Cloudinary env is absent; upload returns **503** `capability_not_configured` only.

## 7. Smoke test

1. `GET /health/live`, `GET /health/ready`
2. `POST /v1/auth/login`
3. `POST /v1/admin/product-images` (multipart `file`, `Idempotency-Key`)
4. Create product with `primaryMediaId`
5. Machine catalog sync — confirm `primaryImage` metadata and `cacheKey`

See [PRODUCT_IMAGE_CLOUDINARY_SMOKE_TEST.md](../testing/PRODUCT_IMAGE_CLOUDINARY_SMOKE_TEST.md).

## 8. Rollback

- Set `MEDIA_UPLOAD_ENABLED=false` or remove `MEDIA_PROVIDER=cloudinary` on app nodes
- Redeploy API
- Existing product images (Cloudinary URLs already stored) remain valid

## 9. Security

- Never expose `CLOUDINARY_API_SECRET` to clients
- Rotate API secret in Cloudinary if leaked
- Upload validation rejects SVG and non-image types server-side
- Deploy scripts log cloud name only; API secret is always `[REDACTED]`
