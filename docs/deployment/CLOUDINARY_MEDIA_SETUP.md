# Cloudinary Media Setup (Production)

## 1. Create Cloudinary account

Sign up for the [Cloudinary free tier](https://cloudinary.com/).

## 2. Copy API credentials

In Cloudinary Console → **Settings → API Keys**, copy:

- Cloud name
- API key
- API secret (**server-side only**)

## 3. GitHub Actions secrets

Add repository secrets (Settings → Secrets and variables → Actions):

- `CLOUDINARY_CLOUD_NAME`
- `CLOUDINARY_API_KEY`
- `CLOUDINARY_API_SECRET`

## 4. Production environment

On the API app node (`.env.app-node` or deployment template):

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

## 5. Deploy

Deploy API as usual. Health endpoints must remain **200** even when Cloudinary env is absent (upload route returns 503 only).

## 6. Smoke test

1. `GET /health/live`, `GET /health/ready`
2. `POST /v1/auth/login`
3. `POST /v1/admin/product-images` (multipart `file`, `Idempotency-Key`)
4. Create product with `primaryMediaId`
5. Machine catalog sync — confirm `primaryImage` metadata and `cacheKey`

See [PRODUCT_IMAGE_CLOUDINARY_SMOKE_TEST.md](../testing/PRODUCT_IMAGE_CLOUDINARY_SMOKE_TEST.md).

## 7. Rollback

- Set `MEDIA_UPLOAD_ENABLED=false` or remove `MEDIA_PROVIDER=cloudinary`
- Redeploy API
- Existing product images (Cloudinary URLs already stored) remain valid

## 8. Security

- Never expose `CLOUDINARY_API_SECRET` to clients
- Rotate API secret in Cloudinary if leaked
- Upload validation rejects SVG and non-image types server-side
