# MEDIA_COMPANY_ID Product Image Upload Audit

**Date:** 2026-05-20  
**Branch:** `fix/media-company-id-product-images`

## Problem

Production `POST /v1/admin/product-images` returned:

```json
400 invalid_argument
"mediaadmin: invalid argument: company_id"
```

Postman sent only `file`, `purpose=product_image`, `altText` — no `company_id`.

## Root cause

1. `requireCatalogPrincipalUUID` in single-company mode always returned `uuid.Nil`.
2. `UploadProductImageFile` rejected `CompanyID == uuid.Nil`.
3. `adminMediaOrgAllowed` denied non-platform admins when `scopeID != uuid.Nil` (would block after fix if not updated).

## Route

- `POST /v1/admin/product-images`
- Alias: `POST /v1/admin/media/product-images`
- Handler: `postAdminProductImageUploadHandler` in `internal/httpserver/admin_media_http.go`
- Service: `internal/app/mediaadmin/cloudinary_upload.go` → `UploadProductImageFile`

## Config (before fix)

- `MEDIA_PROVIDER`, `MEDIA_UPLOAD_ENABLED`, `CLOUDINARY_*` — present
- `MEDIA_COMPANY_ID` — **missing**
- `MEDIA_SCOPE_ID` — not used in repo

## Fix plan (implemented)

1. Add canonical `MEDIA_COMPANY_ID` to `MediaUploadConfig` with UUID validation.
2. Production startup fails if Cloudinary enabled and `MEDIA_COMPANY_ID` missing.
3. Handler resolves company id: explicit admin override → `MEDIA_COMPANY_ID` from config.
4. Update deploy scripts / GitHub workflow to sync `MEDIA_COMPANY_ID`.
5. Postman: no enabled `company_id` field; tests assert upload without client company id.
6. Docs and env examples use `MEDIA_COMPANY_ID` only.

## Deployment env rendering

- `deployments/prod/shared/scripts/apply_cloudinary_app_node_env.sh`
- `.github/workflows/deploy-prod.yml` — `vars.MEDIA_COMPANY_ID` or `secrets.MEDIA_COMPANY_ID`
- `deployments/prod/shared/scripts/validate_cloudinary_deploy_secrets.sh`

## Files changed

| Area | Files |
|------|-------|
| Config | `internal/config/config.go`, `media_upload_config_test.go` |
| Handler | `internal/httpserver/admin_scope.go`, `admin_media_http.go`, tests |
| Service | `internal/app/mediaadmin/cloudinary_upload.go` |
| Deploy | `apply_cloudinary_app_node_env.sh`, `validate_cloudinary_deploy_secrets.sh`, `deploy-prod.yml` |
| Examples | `.env*.example`, `deployments/prod/app-node/.env.app-node.example` |
| Postman | `scripts/postman/generate_production_full_suite.py` |
| Docs | `docs/deployment/CLOUDINARY_MEDIA_SETUP.md`, `docs/testing/PRODUCT_IMAGE_APP_FLOW_GUIDE.md` |
