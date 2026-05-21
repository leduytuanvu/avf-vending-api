# Product Catalog Flow — Production Postman Testing

Guide for catalog setup, product creation, pricing, and machine assignment using **AVF Production Full API Suite**.

## Concepts

- **Product creation is catalog-level.** Creating a product adds it to the admin catalog; it is **not** sold by any machine automatically.
- **Machine sales require assignment.** A machine sells a product only after planogram/slot/assortment assignment and publish.
- **Product images** use `POST /v1/admin/product-images` with **multipart/form-data**. Do **not** manually set `Content-Type`.
- **Vending app** should use online image URLs and cache locally for offline mode.

## Import

1. Import `avf-production-full.postman_collection.json`
2. Import `avf-production.postman_environment.json`
3. Select **AVF Production** environment

## Required environment variables

| Variable | Value |
|----------|-------|
| `baseUrl` | `https://api.ldtv.dev` |
| `adminEmail` | your admin email |
| `adminPassword` | `<set-in-postman>` |
| `allowGatedWrites` | `true` |
| `confirmProductionWrites` | `I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` |
| `accessToken` | `<auto-populated-after-login>` |

Flow variables (auto-populated by tests when successful):

| Variable | Set after |
|----------|-----------|
| `categoryId` | POST /v1/admin/categories |
| `brandId` | POST /v1/admin/brands |
| `tagId` | POST /v1/admin/tags (optional) |
| `mediaId` / `primaryMediaId` | POST /v1/admin/product-images |
| `productId` / `sku` | POST /v1/admin/products |

## Step-by-step flow

### 1. Login

- **POST** `/v1/auth/login`
- Body: `{ "email": "{{adminEmail}}", "password": "{{adminPassword}}" }`
- Saves `accessToken`, `refreshToken`

### 2. Create category

- **POST** `/v1/admin/categories`
- Pre-request builds JSON: `{ active, name, slug }` (unique suffix)
- Saves `categoryId`

### 3. Create brand

- **POST** `/v1/admin/brands`
- Saves `brandId`

### 4. Create tag (optional)

- **POST** `/v1/admin/tags`
- Saves `tagId`

### 5. Upload product image

- **POST** `/v1/admin/product-images`
- **form-data:** `file` (File), `purpose=product_image`, `altText=Coca Cola 330ml product image`
- **No manual Content-Type header**
- Saves `mediaId` and `primaryMediaId`

See also: `PRODUCT_IMAGE_UPLOAD_TESTING.md`

### 6. Create product

- **POST** `/v1/admin/products`
- Pre-request builds valid JSON matching backend `V1AdminProductMutationRequest` (camelCase):

```json
{
  "sku": "COCA-330ML-<timestamp>",
  "name": "Coca Cola Can 330ml",
  "description": "Production canary test product",
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "categoryId": "<from step 2>",
  "brandId": "<from step 3>",
  "primaryMediaId": "<from step 5>",
  "tagIds": ["<optional from step 4>"]
}
```

- Required when `active: true`: `primaryMediaId` (ready uploaded media)
- Saves `productId`, `sku`
- **Does not assign to machines**

### 7. Price book (optional)

- Create/update price book and items under **Promotions PriceBooks**
- Assign targets to machine/site as needed

### 8. Machine assignment

- Configure machine planogram draft, add slot with `productId`, publish planogram
- Sync machine catalog: **POST** `/v1/admin/machines/{{machineId}}/sync` or runtime catalog fetch

### 9. Verify catalog / image

- **GET** `/v1/admin/products/{{productId}}` — verify image metadata
- **GET** `/v1/machines/{{machineId}}/sale-catalog` — verify product appears after assignment

## Backend contract (product create)

Source: `internal/httpserver/openapi_types.go` → `V1AdminProductMutationRequest`

| JSON key | Required | Notes |
|----------|----------|-------|
| `sku` | yes | string |
| `name` | yes | string |
| `active` | yes | bool |
| `ageRestricted` | yes | bool |
| `description` | no | string |
| `categoryId` | no | UUID string |
| `brandId` | no | UUID string |
| `barcode` | no | string |
| `primaryMediaId` | conditional | required when `active: true` on create |
| `primaryImageUrl` | no | alternative to media upload |
| `tagIds` | no | UUID array |
| `allergenCodes` | no | string array |
| `attrs` | no | JSON object |

Use **camelCase** only. Do not send `company_id` on product create.

## Troubleshooting

| Error | Fix |
|-------|-----|
| `invalid_json` on product create | Re-import collection; pre-request must `JSON.stringify` body; ensure `Content-Type: application/json` |
| `GATED-WRITE blocked` | Set `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` |
| `missing_idempotency_key` | Re-send; pre-request sets `_runtimeIdempotencyKey` |
| `invalid_image_file` / unsupported content type | Do not set manual Content-Type on multipart; reselect png/jpg/webp/gif |
| `401 unauthorized` | Login first; check `accessToken` |
| `invalid_argument` / `company_id` | Remove `company_id` from requests — backend uses `MEDIA_COMPANY_ID` server-side for uploads |
| `active products require primaryMediaId` | Upload image first; ensure `primaryMediaId` env var is set |

## Re-import

After repo updates, re-import collection and environment so pre-request scripts and bodies stay in sync.
