# AVF Production Full API Testing Guide

Generated: 2026-05-20T08:37:41.525090+00:00

## 1. Generated files

This folder contains exactly three files:

1. `avf-production-full.postman_collection.json`
2. `avf-production.postman_environment.json`
3. `AVF_PRODUCTION_FULL_TESTING_GUIDE.md` (this file)

## 2. Import into Postman

1. Postman → **Import** → select `avf-production-full.postman_collection.json`
2. Import `avf-production.postman_environment.json`
3. Select **AVF Production** environment in the top-right dropdown
4. Fill `adminEmail`, `adminPassword`, `mqttUsername`, `mqttPassword` locally — **never commit** filled values
5. Run `00_Health_System` → REST → health/live first

## 3. Required environment variables

| Variable | REST | gRPC | MQTT | Placeholder | Description |
| --- | --- | --- | --- | --- | --- |
| baseUrl | yes | — | — | https://api.ldtv.dev | REST API host |
| accessToken | yes | yes | — | (empty) | Admin JWT after login |
| machineToken | — | yes | — | (empty) | Machine JWT for machine.* gRPC |
| grpcHost / grpcPort / grpcTls | — | yes | — | api.ldtv.dev:443 | gRPC endpoint |
| mqttHost / mqttPort / mqttTls | — | — | yes | api.ldtv.dev:8883 | MQTT broker |
| mqttTopicPrefix | — | — | yes | avf/prod | Topic prefix per ACL |
| machineId | yes | yes | yes | (empty) | Target machine UUID |
| requestId / idempotencyKey | yes | yes | — | auto-generated | Correlation / idempotency |

## 4. Test REST APIs individually first

Grouped by **module/domain** folders in the collection (not by business flow).

### 00_Health_System

- `GET /health/live` — Liveness probe
  - Expected 200: `{}`
- `GET /health/ready` — Readiness probe
  - Expected 200: `{}`
- `GET /metrics` — Prometheus metrics scrape (public listener; optional)
  - Expected 200: `{}`
- `GET /swagger/doc.json` — OpenAPI 3.0 document (embedded)
  - Expected 200: `{"info": {"title": "AVF Vending HTTP API", "version": "1.0"}, "openapi": "3.0.3"}`
- `GET /swagger/index.html` — Swagger UI (HTML)
  - Expected 200: `{}`
- `GET /version` — Build and runtime version
  - Expected 200: `{"app_env": "development", "build_time": "2026-04-19T12:00:00Z", "git_sha": "abc123", "name": "avf-vending-api", "proces`

### 01_Auth

- `POST /v1/auth/change-password` — Change password (self-service)
  - Request: `{"currentPassword": "{{adminPassword}}", "newPassword": "{{adminPassword}}"}`
  - Expected 204: `{}`
- `POST /v1/auth/login` — Exchange email/password for JWT session tokens
  - Request: `{"email": "{{adminEmail}}", "password": "{{adminPassword}}"}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "email": "operator@example.com", "roles": ["admin"], "tokens": {"a`
- `POST /v1/auth/logout` — Revoke refresh token(s)
  - Request: `{"revokeAll": false}`
  - Expected 204: `{}`
- `GET /v1/auth/me` — Current authenticated principal
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "email": "operator@example.com", "roles": ["admin"]}`
- `POST /v1/auth/mfa/totp/disable` — Disable TOTP for the current user
  - Request: `{"currentPassword": "{{adminPassword}}", "totpCode": "123456"}`
  - Expected 204: `{}`
- `POST /v1/auth/mfa/totp/enroll` — Start TOTP MFA enrollment
  - Expected 200: `{"otpauthUri": "otpauth://totp/AVF%20Admin:operator%40example.com?secret=ABCDABCDABCDABCD&issuer=AVF%20Admin", "secret":`
- `POST /v1/auth/mfa/totp/verify` — Verify TOTP (enrollment or login)
  - Request: `{"code": "canary-code-{{$guid}}"}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "email": "operator@example.com", "roles": ["admin"], "tokens": {"a`
- `POST /v1/auth/password/change` — Change password (self-service)
  - Request: `{"currentPassword": "{{adminPassword}}", "newPassword": "{{adminPassword}}"}`
  - Expected 204: `{}`
- `POST /v1/auth/password/reset/confirm` — Confirm password reset
  - Request: `{"newPassword": "{{adminPassword}}", "token": "opaque-reset-token"}`
  - Expected 204: `{}`
- `POST /v1/auth/password/reset/request` — Request password reset
  - Request: `{"email": "{{adminEmail}}"}`
  - Expected 202: `{"accepted": true}`
- `POST /v1/auth/refresh` — Rotate access token using a refresh token
  - Request: `{"refreshToken": "{{refreshToken}}"}`
  - Expected 200: `{"tokens": {"accessExpiresAt": "2026-04-19T13:00:00Z", "accessToken": "stub-access-token", "refreshExpiresAt": "2026-05-`
- `DELETE /v1/auth/sessions` — Revoke other sessions
  - Request: `{"exceptRefreshToken": "stub-refresh-token"}`
  - Expected 204: `{}`
- `GET /v1/auth/sessions` — List current admin sessions
  - Expected 200: `{"sessions": [{"createdAt": "2026-04-19T10:00:00Z", "expiresAt": "2026-05-19T12:00:00Z", "sessionId": "bbbbbbbb-bbbb-bbb`
- `DELETE /v1/auth/sessions/{sessionId}` — Revoke one session
  - Expected 204: `{}`

### 02_Admin_Accounts_RBAC

- `GET /v1/admin/users` — List API accounts (admin) — alternate path
  - Expected 200: `{"items": [{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator`
- `POST /v1/admin/users` — Create API account — alternate path
  - Request: `{"email": "{{adminEmail}}", "password": "{{adminPassword}}", "roles": ["support"], "status": "active"}`
  - Expected 201: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `GET /v1/admin/users/{userId}` — Get API account — alternate path
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `PATCH /v1/admin/users/{userId}` — Patch API account — alternate path
  - Request: `{"roles": ["support"]}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/users/{userId}/disable` — Disable API account — alternate path
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/users/{userId}/enable` — Enable API account — alternate path
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/users/{userId}/reset-password` — Reset password — alternate path
  - Request: `{"password": "{{adminPassword}}"}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/users/{userId}/revoke-sessions` — Revoke user sessions — alternate path
  - Expected 204: `{}`
- `PATCH /v1/admin/users/{userId}/roles` — Replace roles — PATCH alias
  - Request: `{"roles": ["support"]}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/users/{userId}/roles` — Replace roles — alternate path
  - Request: `{"roles": ["support"]}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `PUT /v1/admin/users/{userId}/roles` — Replace roles — alternate path
  - Request: `{"roles": ["catalog_manager"]}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `DELETE /v1/admin/users/{userId}/roles/{role}` — Remove one role from user
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `GET /v1/admin/users/{userId}/sessions` — List user sessions — alternate path
  - Expected 200: `{"sessions": [{"createdAt": "2026-04-19T10:00:00Z", "expiresAt": "2026-05-19T12:00:00Z", "sessionId": "bbbbbbbb-bbbb-bbb`
- `PATCH /v1/admin/users/{userId}/status` — Patch account status only — alternate path
  - Request: `{"status": "disabled"}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`

### 04_Brands

- `GET /v1/admin/brands` — List brands
  - Expected 200: `{"items": [{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "`
- `POST /v1/admin/brands` — Create brand
  - Request: `{"active": true, "name": "Coca {{$timestamp}}", "slug": "coca-{{$timestamp}}"}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "Coca exampl`
- `DELETE /v1/admin/brands/{brandId}` — Deactivate brand
  - Expected 200: `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "Coca examp`
- `PATCH /v1/admin/brands/{brandId}` — Update brand (PATCH)
  - Request: `{"active": true, "name": "Coca {{$timestamp}}", "slug": "coca-{{$timestamp}}"}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "Coca exampl`
- `PUT /v1/admin/brands/{brandId}` — Update brand
  - Request: `{"active": true, "name": "Coca {{$timestamp}}", "slug": "coca-{{$timestamp}}"}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "Coca exampl`

### 05_Categories

- `GET /v1/admin/categories` — List categories
  - Expected 200: `{"items": [{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "`
- `POST /v1/admin/categories` — Create category
  - Request: `{"active": true, "name": "Drinks {{$timestamp}}", "slug": "drinks-{{$timestamp}}"}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "Drinks exam`
- `DELETE /v1/admin/categories/{categoryId}` — Deactivate category
  - Expected 200: `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "Drinks exa`
- `PATCH /v1/admin/categories/{categoryId}` — Update category (PATCH)
  - Request: `{"active": true, "name": "Drinks {{$timestamp}}", "slug": "drinks-{{$timestamp}}"}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "Drinks exam`
- `PUT /v1/admin/categories/{categoryId}` — Update category
  - Request: `{"active": true, "name": "Drinks {{$timestamp}}", "slug": "drinks-{{$timestamp}}"}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "Drinks exam`

### 06_Tags

- `GET /v1/admin/tags` — List tags
  - Expected 200: `{"items": [{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "`
- `POST /v1/admin/tags` — Create tag
  - Request: `{"active": true, "name": "Cold Drink {{$timestamp}}", "slug": "cold-drink-{{$timestamp}}"}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "Cold drink `
- `DELETE /v1/admin/tags/{tagId}` — Deactivate tag
  - Expected 200: `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "Cold drink`
- `PATCH /v1/admin/tags/{tagId}` — Update tag (PATCH)
  - Request: `{"active": true, "name": "Cold Drink {{$timestamp}}", "slug": "cold-drink-{{$timestamp}}"}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "Cold drink `
- `PUT /v1/admin/tags/{tagId}` — Update tag
  - Request: `{"active": true, "name": "Cold Drink {{$timestamp}}", "slug": "cold-drink-{{$timestamp}}"}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "Cold drink `

### 07_Product_Media

- `GET /v1/admin/media` — List media assets (alias path)
  - Expected 200: `{"items": [{"created_at": "2026-01-01T00:00:00Z", "etag": "W/\"etag1\"", "id": "11111111-2222-3333-4444-555555555555", "`
- `GET /v1/admin/media/assets` — List media assets (admin)
  - Expected 200: `{"items": [{"created_at": "2026-01-01T00:00:00Z", "etag": "W/\"etag1\"", "id": "11111111-2222-3333-4444-555555555555", "`
- `POST /v1/admin/media/assets` — Start enterprise media asset upload
  - Request: `{"content_type": "image/jpeg"}`
  - Expected 200: `{"complete_path": "/v1/admin/media/11111111-2222-3333-4444-555555555555/complete", "expires_at": "2026-04-19T13:00:00Z",`
- `DELETE /v1/admin/media/assets/{mediaId}` — Delete media asset (admin)
  - Expected 204: `{}`
- `GET /v1/admin/media/assets/{mediaId}` — Get one media asset by id (admin)
  - Expected 200: `{"created_at": "2026-01-01T00:00:00Z", "etag": "W/\"etag1\"", "id": "11111111-2222-3333-4444-555555555555", "kind": "pro`
- `POST /v1/admin/media/uploads` — Start enterprise media upload (presigned PUT)
  - Request: `{"content_type": "image/jpeg"}`
  - Expected 200: `{"complete_path": "/v1/admin/media/11111111-2222-3333-4444-555555555555/complete", "expires_at": "2026-04-19T13:00:00Z",`
- `POST /v1/admin/media/uploads/init` — Start enterprise media upload (camelCase contract)
  - Request: `{"contentType": "image/png", "filename": "coca-330ml.png", "purpose": "product_image"}`
  - Expected 200: `{"completePath": "/v1/admin/media/uploads/11111111-2222-3333-4444-555555555555/complete", "mediaId": "11111111-2222-3333`
- `POST /v1/admin/media/uploads/{mediaId}/complete` — Finalize media upload (uploads/{mediaId}/complete alias)
  - Request: `{"contentType": "image/png", "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "sizeBytes": `
  - Expected 200: `{"id": "11111111-2222-3333-4444-555555555555", "status": "ready", "variants": [{"downloadUrl": "https://cdn.example.com/`
- `DELETE /v1/admin/media/{mediaId}` — Delete media asset (alias path)
  - Expected 204: `""`
- `GET /v1/admin/media/{mediaId}` — Get media asset (alias path)
  - Expected 200: `{"created_at": "2026-01-01T00:00:00Z", "etag": "W/\"etag1\"", "id": "11111111-2222-3333-4444-555555555555", "kind": "pro`
- `POST /v1/admin/media/{mediaId}/complete` — Finalize media upload (variants + ready)
  - Request: `{"contentType": "{{$guid}}", "sha256": "{{$guid}}", "sizeBytes": 0}`
  - Expected 200: `{"id": "11111111-2222-3333-4444-555555555555", "status": "ready", "variants": [{"downloadUrl": "https://cdn.example.com/`
- `POST /v1/admin/products/{productId}/media` — Bind media to product (POST)
  - Request: `{"media_id": "11111111-2222-3333-4444-555555555555"}`
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`
- `PUT /v1/admin/products/{productId}/media` — Bind or replace product media (PUT)
  - Request: `{"media_id": "11111111-2222-3333-4444-555555555555"}`
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`
- `DELETE /v1/admin/products/{productId}/media/{mediaId}` — Remove bound media from product
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`

### 08_Products

- `GET /v1/admin/products` — List products (admin catalog)
  - Expected 200: `{"items": [{"active": true, "barcode": "8850123456789", "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "categoryId":`
- `POST /v1/admin/products` — Create product (admin catalog)
  - Request: `{"active": true, "ageRestricted": false, "allergenCodes": [], "barcode": "8850123456789", "brandId": "aaaaaaaa-bbbb-cccc`
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`
- `DELETE /v1/admin/products/{productId}` — Deactivate product
  - Expected 200: `{"active": false, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaa`
- `GET /v1/admin/products/{productId}` — Get product by id (admin catalog)
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`
- `PATCH /v1/admin/products/{productId}` — Update product (PATCH)
  - Request: `{"active": true, "ageRestricted": false, "allergenCodes": [], "barcode": "8850123456789", "brandId": "aaaaaaaa-bbbb-cccc`
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`
- `PUT /v1/admin/products/{productId}` — Update product (PUT/PATCH)
  - Request: `{"active": true, "ageRestricted": false, "allergenCodes": [], "barcode": "8850123456789", "brandId": "aaaaaaaa-bbbb-cccc`
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`
- `DELETE /v1/admin/products/{productId}/image` — Remove primary product image
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`
- `POST /v1/admin/products/{productId}/image` — Bind primary product image (alias)
  - Request: `{"artifactId": "11111111-2222-3333-4444-555555555555", "contentHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`
- `PUT /v1/admin/products/{productId}/image` — Bind primary product image
  - Request: `{"artifactId": "11111111-2222-3333-4444-555555555555", "contentHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`
  - Expected 200: `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa`
- `GET /v1/admin/reports/products` — Product performance report
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`

### 09_Sites_Regions

- `GET /v1/admin/sites` — List sites (admin)
  - Expected 200: `{"items": [{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f6`
- `POST /v1/admin/sites` — Create site (admin)
  - Request: `{"address": {"line1": "1 Main St"}, "code": "canary-code-{{$guid}}", "name": "canary-name-{{$guid}}", "timezone": "UTC"}`
  - Expected 201: `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na`
- `DELETE /v1/admin/sites/{siteId}` — Deactivate site (admin)
  - Expected 200: `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na`
- `GET /v1/admin/sites/{siteId}` — Get site by ID (admin)
  - Expected 200: `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na`
- `PATCH /v1/admin/sites/{siteId}` — Patch site (admin)
  - Request: `{"name": "canary-name-{{$guid}}", "status": "active"}`
  - Expected 200: `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na`
- `POST /v1/admin/sites/{siteId}/archive` — Archive site (admin alias)
  - Expected 200: `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na`
- `POST /v1/admin/sites/{siteId}/disable` — Disable site (admin)
  - Expected 200: `{"address": {}, "code": "HQ-01", "created_at": "2026-04-01T00:00:00.000000000Z", "id": "aaaaaaaa-bbbb-cccc-dddd-11111111`

### 10_Machines

- `GET /v1/admin/commands` — List machine commands (admin)
  - Expected 200: `{"items": [{"attemptCount": 1, "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "commandType": "SET_TEMPERATURE", "c`
- `GET /v1/admin/commands/{commandId}` — Get command ledger row by id
  - Expected 200: `{"attempts": [{"attemptNo": 1, "dispatchState": "failed", "id": "cccccccc-dddd-eeee-ffff-000000000001", "sentAt": "2026-`
- `POST /v1/admin/commands/{commandId}/cancel` — Cancel pending command
  - Expected 200: `{"attemptsCancelled": 1}`
- `POST /v1/admin/commands/{commandId}/retry` — Retry failed command dispatch
  - Expected 200: `{"attemptId": "dddddddd-eeee-ffff-0000-111111111111", "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "dispatchStat`
- `GET /v1/admin/feature-flags` — List company feature flags
  - Expected 200: `{"items": [{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": `
- `POST /v1/admin/feature-flags` — Create a feature flag
  - Request: `{"description": "Experimental UI", "displayName": "Beta UI", "enabled": false, "flagKey": "kiosk.beta_ui", "metadata": {`
  - Expected 201: `{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": false, "fla`
- `GET /v1/admin/feature-flags/{flagId}` — Get feature flag and scoped targets
  - Expected 200: `{"flag": {"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": fa`
- `PATCH /v1/admin/feature-flags/{flagId}` — Patch feature flag metadata / master enabled bit
  - Request: `{"displayName": "Beta UI v2", "enabled": true}`
  - Expected 200: `{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": false, "fla`
- `POST /v1/admin/feature-flags/{flagId}/disable` — Disable feature flag (master switch)
  - Expected 200: `{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": false, "fla`
- `POST /v1/admin/feature-flags/{flagId}/enable` — Enable feature flag (master switch)
  - Expected 200: `{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": false, "fla`
- `PUT /v1/admin/feature-flags/{flagId}/targets` — Replace scoped targets for a feature flag
  - Request: `{"targets": [{"enabled": true, "machineId": "{{machineId}}", "metadata": {}, "priority": 10, "targetType": "machine"}]}`
  - Expected 200: `{"targets": []}`
- `GET /v1/admin/machines` — List machines (admin)
  - Expected 200: `{"items": [{"assignedTechnicians": [], "commandSequence": 12, "createdAt": "2026-01-01T00:00:00.000000000Z", "effectiveT`
- `POST /v1/admin/machines` — Create machine (admin)
  - Request: `{"cabinetType": "ambient", "code": "canary-code-{{$guid}}", "model": "AVF-1", "name": "canary-name-{{$guid}}", "serialNu`
  - Expected 201: `{"code": "M001", "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7`
- `GET /v1/admin/machines/{machineId}` — Get machine (admin)
  - Expected 200: `{"assignedTechnicians": [], "commandSequence": 12, "createdAt": "2026-01-01T00:00:00.000000000Z", "effectiveTimezone": "`
- `PATCH /v1/admin/machines/{machineId}` — Patch machine metadata (admin)
  - Request: `{"name": "canary-name-{{$guid}}", "status": "active"}`
  - Expected 200: `{"code": "M001", "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7`
- `POST /v1/admin/machines/{machineId}/archive` — Retire machine (archive alias)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7425-40de-944b-e0`
- `POST /v1/admin/machines/{machineId}/commands` — Dispatch new command to machine (MQTT/device pipeline)
  - Request: `{"commandType": "REQUEST_DIAGNOSTICS", "payload": {"bundle": "logs"}}`
  - Expected 200: `{"status": "ok"}`
- `POST /v1/admin/machines/{machineId}/disable` — Disable machine (admin)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-01T00:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "n`
- `POST /v1/admin/machines/{machineId}/enable` — Enable machine (admin)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-01T00:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "n`
- `GET /v1/admin/machines/{machineId}/health` — Machine health detail
  - Expected 200: `{"failedCommandCount": 0, "inventoryAnomalyCount": 0, "lastSeenAt": "2026-04-29T12:00:00.000000000Z", "machineId": "7c9e`
- `POST /v1/admin/machines/{machineId}/mark-compromised` — Mark machine compromised (admin)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 1, "id": "7c9e6679-7425-40de-944b-e0`
- `POST /v1/admin/machines/{machineId}/resume` — Resume suspended machine (admin)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7425-40de-944b-e0`
- `POST /v1/admin/machines/{machineId}/retire` — Retire machine (admin)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-01T00:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "n`
- `POST /v1/admin/machines/{machineId}/revoke-credentials` — Revoke machine credential material (admin)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 3, "id": "7c9e6679-7425-40de-944b-e0`
- `POST /v1/admin/machines/{machineId}/revoke-sessions` — Revoke interactive sessions for machine technicians/operators (admin)
  - Expected 204: `{}`
- `POST /v1/admin/machines/{machineId}/revoke-token` — Revoke machine tokens (alias path)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 3, "id": "7c9e6679-7425-40de-944b-e0`
- `GET /v1/admin/machines/{machineId}/slots` — List live slot inventory for a machine (restock / cycle-count UI)
  - Expected 200: `{"cabinets": [], "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "slots": [{"cabinetCode": "A", "currentQuantity": `
- `POST /v1/admin/machines/{machineId}/suspend` — Suspend machine (admin)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7425-40de-944b-e0`
- `POST /v1/admin/machines/{machineId}/sync` — Queue a machine setup / inventory sync command
  - Request: `{"operator_session_id": "{{operatorSessionId}}", "reason": "post_restock_verify"}`
  - Expected 200: `{"command": {"commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "dispatchState": "published", "replay": false, "sequen`
- `GET /v1/admin/machines/{machineId}/technicians` — List technicians assigned to a machine
  - Expected 200: `{"items": [{"assignmentId": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "createdAt": "2026-04-29T00:00:00Z", "machineId": "7`
- `POST /v1/admin/machines/{machineId}/technicians` — Assign technician user to machine
  - Request: `{"role": "field_service", "scope": "maintenance", "userId": "{{$guid}}"}`
  - Expected 201: `{"created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "machine_id": "7c9e6679-7425-40de-9`
- `DELETE /v1/admin/machines/{machineId}/technicians/{userId}` — Remove technician assignment from machine by user id
  - Expected 204: `""`
- `GET /v1/admin/machines/{machineId}/timeline` — Machine operational timeline
  - Expected 200: `{"items": [{"eventKind": "command_attempt", "occurredAt": "2026-04-29T12:00:00.000000000Z", "payload": {"status": "sent"`
- `POST /v1/admin/machines/{machineId}/transfer-site` — Move machine to another site (admin)
  - Request: `{"site_id": "{{siteId}}"}`
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7425-40de-944b-e0`
- `POST /v1/machines/{machineId}/commands/dispatch` — Dispatch remote MQTT command
  - Request: `{"command_type": "SET_TEMPERATURE", "desired_state": {}, "payload": {"celsius": 4}}`
  - Expected 200: `{"attempt_id": "cccccccc-dddd-eeee-ffff-000000000001", "command_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "dispatch_s`
- `GET /v1/machines/{machineId}/commands/receipts` — List recent command receipts
  - Expected 200: `{"items": [], "meta": {"limit": 50, "returned": 0}}`
- `GET /v1/machines/{machineId}/commands/{sequence}/status` — Get command dispatch status by sequence
  - Expected 200: `{"attempt": {"ack_deadline_at": "2026-04-19T12:00:40Z", "attempt_no": 1, "id": "cccccccc-dddd-eeee-ffff-000000000001", "`
- `GET /v1/machines/{machineId}/operator-sessions/action-attributions` — List action attributions for machine
  - Expected 200: `{"items": [], "meta": {"limit": 50, "returned": 0}}`
- `GET /v1/machines/{machineId}/operator-sessions/auth-events` — List auth events
  - Expected 200: `{"items": [], "meta": {"limit": 50, "returned": 0}}`
- `GET /v1/machines/{machineId}/operator-sessions/current` — Get current operator session
  - Expected 200: `{"active_session": null, "technician_display_name": ""}`
- `GET /v1/machines/{machineId}/operator-sessions/history` — List session history
  - Expected 200: `{"items": [], "meta": {"limit": 50, "returned": 0}}`
- `POST /v1/machines/{machineId}/operator-sessions/login` — Start or resume operator session
  - Request: `{"auth_method": "oidc", "client_metadata": {"kiosk": "A12"}}`
  - Expected 200: `{"session": {"actor_type": "TECHNICIAN", "client_metadata": {}, "created_at": "2026-04-19T12:10:00Z", "id": "dddddddd-ee`
- `POST /v1/machines/{machineId}/operator-sessions/logout` — End operator session
  - Request: `{"auth_method": "oidc", "ended_reason": "user_logout", "session_id": "{{operatorSessionId}}"}`
  - Expected 200: `{"session": {"actor_type": "TECHNICIAN", "client_metadata": {}, "created_at": "2026-04-19T12:10:00Z", "id": "dddddddd-ee`
- `GET /v1/machines/{machineId}/operator-sessions/timeline` — Combined operator timeline
  - Expected 200: `{"items": [], "meta": {"limit": 50, "returned": 0}}`
- `POST /v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat` — Session activity heartbeat
  - Expected 200: `{"session": {"session": {"actor_type": "TECHNICIAN", "client_metadata": {}, "created_at": "2026-04-19T12:10:00Z", "id": `
- `GET /v1/machines/{machineId}/sale-catalog` — Runtime sale catalog (planogram, price, stock, images)
  - Expected 200: `{"configVersion": 7, "currency": "VND", "generatedAt": "2026-04-24T00:00:00Z", "items": [{"availableQuantity": 8, "cabin`
- `GET /v1/operator-insights/technicians/{technicianId}/action-attributions` — List action attributions for a technician
  - Expected 200: `{"items": [], "meta": {"limit": 50, "returned": 0}}`
- `GET /v1/operator-insights/users/action-attributions` — List action attributions for a user principal
  - Expected 200: `{"items": [], "meta": {"limit": 50, "returned": 0}}`

### 11_Machine_Provisioning

- `GET /v1/admin/activation-codes` — List activation codes across machines (admin catalog)
  - Expected 200: `{"items": [{"activationCodeId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "createdAt": "2026-04-29T00:00:00Z", "expiresAt"`
- `POST /v1/admin/activation-codes` — Create activation code (catalog path; targets machine in body)
  - Request: `{"expiresInMinutes": 1440, "machineId": "{{machineId}}", "maxUses": 1, "notes": "pilot"}`
  - Expected 201: `{"activationCode": "AVF-123456", "activationCodeId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "expiresAt": "2026-04-30T00`
- `POST /v1/admin/activation-codes/{codeId}/revoke` — Revoke activation code by id (catalog path)
  - Expected 200: `{"status": "ok"}`
- `GET /v1/admin/machines/{machineId}/activation-codes` — List activation codes for a machine
  - Expected 200: `{"items": [{"activationCodeId": "11111111-2222-3333-4444-555555555555", "createdAt": "2026-04-23T00:00:00Z", "expiresAt"`
- `POST /v1/admin/machines/{machineId}/activation-codes` — Create machine activation code
  - Request: `{"expiresInMinutes": 1440, "maxUses": 1, "notes": "Field install at site A"}`
  - Expected 201: `{"activationCode": "AVF-123456-ABCDEF", "activationCodeId": "11111111-2222-3333-4444-555555555555", "expiresAt": "2026-0`
- `DELETE /v1/admin/machines/{machineId}/activation-codes/{activationCodeId}` — Revoke an activation code
  - Expected 204: `""`
- `POST /v1/setup/activation-codes/claim` — Claim an activation code (public pre-auth)
  - Request: `{"activationCode": "{{activationCode}}", "deviceFingerprint": {"androidId": "android-123", "manufacturer": "SUNMI", "mod`
  - Expected 200: `{"bootstrapUrl": "/v1/setup/machines/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/bootstrap", "machineId": "7c9e6679-7425-40de-9`
- `GET /v1/setup/machines/{machineId}/bootstrap` — Machine setup bootstrap (topology + catalog)
  - Expected 200: `{"catalog": {"products": [{"assortmentId": "dddddddd-eeee-ffff-0000-111111111111", "assortmentName": "Standard", "name":`

### 12_Machine_Runtime_Config

- `POST /v1/admin/machines/{machineId}/rotate-credential` — Rotate machine credential (admin)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-01T00:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "n`
- `POST /v1/admin/machines/{machineId}/rotate-credentials` — Rotate machine credential (plural alias)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 2, "id": "7c9e6679-7425-40de-944b-e0`
- `POST /v1/admin/machines/{machineId}/rotate-token-version` — Bump credential version / rotate token (admin)
  - Expected 200: `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 2, "id": "7c9e6679-7425-40de-944b-e0`
- `POST /v1/machines/{machineId}/config-applies` — Acknowledge config applied on device
  - Request: `{"android_id": "device-android-1", "app_version": "1.0.0", "applied_at": "2026-04-19T12:05:00Z", "config_payload": {"app`
  - Expected 201: `{"applied_at": "2026-04-19T12:05:00.000000000Z", "config_revision": 7, "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "ma`
- `GET /v1/machines/{machineId}/shadow` — Get machine shadow JSON
  - Expected 200: `{"desired": {"temperature_c": 4.0}, "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "metadata": {"version": 12}, "`

### 13_Telemetry

- `POST /v1/device/machines/{machineId}/commands/poll` — Poll pending remote commands over HTTP (MQTT fallback)
  - Request: `{"limit": 10}`
  - Expected 200: `{"items": [{"command_type": "machine_planogram_publish", "correlation_id": "11111111-2222-3333-4444-555555555555", "idem`
- `POST /v1/device/machines/{machineId}/events/reconcile` — Batch reconcile critical telemetry idempotency keys
  - Request: `{"idempotencyKeys": ["machine-001:boot-20260424:seq-100:events.vend"]}`
  - Expected 200: `{"items": [{"acceptedAt": "2026-04-24T00:00:00Z", "eventType": "events.vend", "idempotencyKey": "machine-001:boot-202604`
- `GET /v1/device/machines/{machineId}/events/{idempotencyKey}/status` — Single critical telemetry idempotency status
  - Expected 200: `{"acceptedAt": null, "eventType": null, "idempotencyKey": "machine-001:boot-20260424:seq-100:events.vend", "processedAt"`
- `POST /v1/device/machines/{machineId}/vend-results` — Report vend outcome for an order (HTTP bridge)
  - Request: `{"correlation_id": "11111111-2222-3333-4444-555555555555", "order_id": "{{orderId}}", "outcome": "success", "slot_index"`
  - Expected 200: `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "completed", "replay": false, "vend_state": "succes`
- `POST /v1/machines/{machineId}/check-ins` — Record Android check-in
  - Request: `{"android_release": "14", "boot_id": "boot-session-1", "manufacturer": "Example", "metadata": {}, "model": "Kiosk-1", "n`
  - Expected 201: `{"id": "12001", "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "occurred_at": "2026-04-19T12:00:00.000000000Z"}`
- `GET /v1/machines/{machineId}/telemetry/incidents` — Recent persisted machine incidents
  - Expected 200: `{"items": [{"code": "TEMP_HIGH", "dedupeKey": "TEMP_HIGH:slot3", "detail": {"threshold_c": 8}, "id": "aaaaaaaa-bbbb-cccc`
- `GET /v1/machines/{machineId}/telemetry/rollups` — Telemetry rollup buckets (1m / 1h)
  - Expected 200: `{"items": [{"bucketStart": "2026-04-19T12:00:00.000000000Z", "extra": {}, "granularity": "1m", "last": 7.1, "max": 8.2, `
- `GET /v1/machines/{machineId}/telemetry/snapshot` — Current machine telemetry snapshot (projected)
  - Expected 200: `{"androidId": "dev123", "appVersion": "1.2.3", "deviceModel": "Pixel", "effectiveTimezone": "America/Los_Angeles", "firm`

### 14_Inventory

- `GET /v1/admin/inventory/anomalies` — List inventory anomalies (ledger-backed)
  - Expected 200: `{"items": [{"anomalyType": "negative_stock", "createdAt": "2026-04-29T12:00:00.000000000Z", "detectedAt": "2026-04-29T12`
- `POST /v1/admin/inventory/anomalies/{anomalyId}/resolve` — Resolve inventory anomaly
  - Expected 200: `{"anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "resolved"}`
- `GET /v1/admin/inventory/low-stock` — List slots estimated to need refill soon (low stock)
  - Expected 200: `{"items": [{"currentQuantity": 3, "dailyVelocity": 1.0, "daysToEmpty": 3.0, "fillRatio": 0.3, "machineId": "7c9e6679-742`
- `GET /v1/admin/inventory/refill-suggestions` — List refill suggestions across machines (all slots)
  - Expected 200: `{"items": [{"currentQuantity": 3, "dailyVelocity": 1.0, "daysToEmpty": 3.0, "fillRatio": 0.3, "machineId": "7c9e6679-742`
- `GET /v1/admin/machines/{machineId}/inventory` — Aggregate inventory by product for a machine
  - Expected 200: `{"items": [{"cabinetCode": "CAB-A", "cabinetIndex": 0, "lowStock": false, "machineId": "7c9e6679-7425-40de-944b-e07fc1f9`
- `GET /v1/admin/machines/{machineId}/inventory-events` — List append-only inventory ledger events for a machine
  - Expected 200: `{"items": [{"cabinetCode": "CAB-A", "currency": "USD", "eventType": "adjustment", "id": 1001, "machineId": "7c9e6679-742`
- `GET /v1/admin/machines/{machineId}/inventory/anomalies` — List inventory anomalies for one machine
  - Expected 200: `{"items": [{"anomalyType": "stale_inventory_sync", "createdAt": "2026-04-29T12:00:00.000000000Z", "detectedAt": "2026-04`
- `POST /v1/admin/machines/{machineId}/inventory/reconcile` — Post machine inventory reconcile adjustment
  - Expected 200: `{"status": "ok"}`
- `GET /v1/admin/machines/{machineId}/refill-suggestions` — Refill suggestions for one machine
  - Expected 200: `{"items": [{"currentQuantity": 3, "dailyVelocity": 1.0, "daysToEmpty": 3.0, "fillRatio": 0.3, "machineId": "7c9e6679-742`
- `POST /v1/admin/machines/{machineId}/stock-adjustments` — Apply stock adjustments (restock, cycle count, manual, reconcile)
  - Request: `{"items": [{"cabinetCode": "A", "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", "productId": "{{productId}}", "qu`
  - Expected 200: `{"eventIds": [1001, 1002], "replay": false}`
- `GET /v1/admin/reports/fills` — Technician and fill / restock inventory operations
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/admin/reports/inventory` — Inventory BI (low stock or movement ledger)
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/admin/reports/inventory-low-stock` — Inventory low-stock report
  - Expected 200: `{"exceptionKind": "low_stock", "from": "2026-04-01T00:00:00.000000000Z", "items": [], "meta": {"limit": 50, "offset": 0,`
- `GET /v1/admin/restock/suggestions` — Restock suggestions (admin)
  - Expected 200: `{"items": [{"currentQuantity": 3, "dailyVelocity": 1.0, "daysToEmpty": 3.0, "fillRatio": 0.3, "machineId": "7c9e6679-742`
- `GET /v1/reports/inventory-exceptions` — Slots needing refill or restock attention
  - Expected 200: `{"exceptionKind": "low_stock", "from": "2026-04-01T00:00:00.000000000Z", "items": [], "meta": {"limit": 50, "offset": 0,`

### 15_Planogram_Assortment

- `PUT /v1/admin/machines/{machineId}/planograms/draft` — Save draft cabinet slot planogram assignments
  - Request: `{"items": [{"cabinetCode": "A", "layoutKey": "grid-4x6", "layoutRevision": 1, "legacySlotIndex": 3, "maxQuantity": 12, "`
  - Expected 204: `{}`
- `POST /v1/admin/machines/{machineId}/planograms/publish` — Publish draft planogram as current and dispatch device command
  - Request: `{"items": [{"cabinetCode": "A", "layoutKey": "grid-4x6", "layoutRevision": 1, "legacySlotIndex": 3, "maxQuantity": 12, "`
  - Expected 200: `{"command": {"commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "dispatchState": "published", "replay": false, "sequen`
- `PUT /v1/admin/machines/{machineId}/topology` — Upsert machine cabinet topology and slot layouts
  - Request: `{"cabinets": [{"code": "canary-code-{{$guid}}", "metadata": {}, "sortOrder": 1, "title": "canary-title-{{$guid}}"}], "la`
  - Expected 204: `{}`
- `GET /v1/admin/planograms` — List planograms (admin catalog)
  - Expected 200: `{"items": [{"createdAt": "2026-04-01T00:00:00Z", "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", "name": "Lobby spring", "`
- `GET /v1/admin/planograms/{planogramId}` — Get planogram detail with slots
  - Expected 200: `{"planogram": {"createdAt": "2026-04-01T00:00:00Z", "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", "name": "Lobby spring"`

### 16_Orders

- `GET /v1/admin/commerce/reconciliation` — List commerce reconciliation cases
  - Expected 200: `{"items": [{"caseType": "payment_paid_vend_failed", "firstDetectedAt": "2026-04-19T12:10:00Z", "id": "99999999-8888-7777`
- `GET /v1/admin/commerce/reconciliation/{caseId}` — Get commerce reconciliation case
  - Expected 200: `{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"}`
- `POST /v1/admin/commerce/reconciliation/{caseId}/ignore` — Ignore commerce reconciliation case
  - Request: `{"id": "{{resource_uuid}}"}`
  - Expected 200: `{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"}`
- `POST /v1/admin/commerce/reconciliation/{caseId}/request-refund` — Request refund from reconciliation case
  - Request: `{"status": "ok"}`
  - Expected 200: `{"status": "ok"}`
- `POST /v1/admin/commerce/reconciliation/{caseId}/resolve` — Resolve commerce reconciliation case
  - Request: `{"id": "{{resource_uuid}}"}`
  - Expected 200: `{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"}`
- `GET /v1/admin/orders/{orderId}/timeline` — List commerce order timeline events
  - Expected 200: `{"items": [], "meta": {"limit": 50, "offset": 0, "returned": 1, "total": 42}}`
- `POST /v1/commerce/cash-checkout` — Create order, record captured cash payment, mark paid
  - Request: `{"currency": "USD", "machine_id": "{{machineId}}", "product_id": "{{productId}}", "slot_index": 3, "subtotal_minor": 125`
  - Expected 200: `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "paid", "payment_id": "aaaaaaaa-bbbb-cccc-dddd-eeee`
- `POST /v1/commerce/orders` — Create order and initial vend session
  - Request: `{"currency": "USD", "machine_id": "{{machineId}}", "product_id": "{{productId}}", "slot_index": 3, "subtotal_minor": 125`
  - Expected 201: `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "created", "replay": false, "vend_session_id": "8d3`
- `GET /v1/commerce/orders/{orderId}` — Checkout status for order
  - Expected 200: `{"order": {"created_at": "2026-04-19T12:00:00Z", "currency": "USD", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "machi`
- `POST /v1/commerce/orders/{orderId}/cancel` — Cancel order before payment capture
  - Request: `{"reason": "user_cancelled", "slot_index": 3}`
  - Expected 200: `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "cancelled", "payment_state": "none", "refund_state`
- `POST /v1/commerce/orders/{orderId}/payment-session` — Start payment with outbox row
  - Request: `{"amount_minor": 135, "currency": "USD", "outbox_payload_json": {"source": "http_api"}, "payment_state": "created", "pro`
  - Expected 200: `{"outbox_event_id": 9001, "payment_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "payment_state": "created", "replay": fa`
- `POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks` — Apply provider webhook
  - Request: `{"event_type": "payment_intent.succeeded", "normalized_payment_state": "captured", "payload_json": {"id": "pi_example_12`
  - Expected 200: `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "payment_state": "captured", "replay": false}`
- `GET /v1/commerce/orders/{orderId}/reconciliation` — Reconciliation snapshot wrapper
  - Expected 200: `{"kind": "commerce.reconciliation_snapshot", "status": {"order": {"created_at": "2026-04-19T12:00:00Z", "currency": "USD`
- `POST /v1/commerce/orders/{orderId}/vend/start` — Advance vend to in_progress
  - Request: `{"slot_index": 3}`
  - Expected 200: `{"slot_index": 3, "vend_state": "in_progress"}`
- `POST /v1/commerce/orders/{orderId}/vend/success` — Finalize vend success
  - Request: `{"slot_index": 3}`
  - Expected 200: `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "completed", "vend_state": "success"}`
- `GET /v1/orders` — List orders for company
  - Expected 200: `{"items": [{"createdAt": "2026-04-19T12:00:00Z", "currency": "USD", "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",`
- `GET /v1/payments` — List payments for company
  - Expected 200: `{"items": [{"amountMinor": 100, "createdAt": "2026-04-19T12:04:00Z", "currency": "USD", "machineId": "7c9e6679-7425-40de`

### 17_Payments

- `GET /v1/admin/machines/{machineId}/cash-collections` — List cash collections for machine
  - Expected 200: `{"items": [{"close_request_hash_hex": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "closed_at": "`
- `POST /v1/admin/machines/{machineId}/cash-collections` — Start cash collection session
  - Request: `{"currency": "USD", "notes": "Field collection \u2014 tray A", "operator_session_id": "{{operatorSessionId}}", "startedA`
  - Expected 200: `{"close_request_hash_hex": null, "closed_at": null, "collected_at": "2026-04-19T14:00:00.000000000Z", "countedPhysicalCa`
- `GET /v1/admin/machines/{machineId}/cash-collections/{collectionId}` — Get one cash collection
  - Expected 200: `{"close_request_hash_hex": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "closed_at": "2026-04-19T`
- `POST /v1/admin/machines/{machineId}/cash-collections/{collectionId}/close` — Close cash collection with counted cash
  - Request: `{"closedAt": "2026-04-24T00:10:00Z", "countedCashboxMinor": 995000, "countedRecyclerMinor": 200000, "currency": "VND", "`
  - Expected 200: `{"close_request_hash_hex": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "closed_at": "2026-04-19T`
- `GET /v1/admin/machines/{machineId}/cashbox` — Cashbox summary (expected vault from commerce)
  - Expected 200: `{"currency": "VND", "denominations": [], "disclosure": "Accounting-only: cloud ledger expectation only; does not sense o`
- `GET /v1/admin/reports/cash` — Cash collection report
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/admin/reports/cash-collections/export.csv` — Export cash collection sessions as CSV (UTF-8)
  - Expected 200: `{}`
- `GET /v1/admin/reports/payments` — Payment settlement report
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"amountMinor": 12500, "bucketStart": "2026-04-01T00:00:00Z", "paymentCount":`
- `GET /v1/admin/reports/payments-summary/export.csv` — Export payments summary as CSV (UTF-8)
  - Expected 200: `{}`
- `GET /v1/reports/payments-summary` — Payment outcomes and method/status breakdown
  - Expected 200: `{"breakdown": [], "from": "2026-04-01T00:00:00Z", "groupBy": "day", "summary": {"authorizedAmountMinor": 10200, "authori`

### 18_Refunds_Disputes

- `POST /v1/admin/orders/{orderId}/refunds` — Create refund request + ledger refund (admin scoped)
  - Request: `{"amountMinor": 100, "reason": "customer courtesy"}`
  - Expected 200: `{"ledgerAmountMinor": 100, "ledgerCurrency": "USD", "ledgerRefundID": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "ledgerSta`
- `GET /v1/admin/refunds` — List refund requests for company
  - Expected 200: `{"items": [], "meta": {"limit": 50, "offset": 0, "returned": 1, "total": 42}}`
- `GET /v1/admin/refunds/{refundId}` — Get refund request by id
  - Expected 200: `{}`
- `GET /v1/admin/reports/refunds` — Refund report
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/commerce/orders/{orderId}/refunds` — List refunds for an order
  - Expected 200: `{"items": [{"amount_minor": 15000, "created_at": "2026-04-24T00:00:00Z", "currency": "VND", "order_id": "3fa85f64-5717-4`
- `POST /v1/commerce/orders/{orderId}/refunds` — Create or replay a refund (idempotent)
  - Request: `{"amount_minor": 15000, "currency": "VND", "metadata": {"slot_index": 3, "vend_failure_reason": "motor_timeout"}, "reaso`
  - Expected 200: `{"amount_minor": 15000, "currency": "VND", "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "payment_id": "11111111-2`
- `GET /v1/commerce/orders/{orderId}/refunds/{refundId}` — Get one refund on an order
  - Expected 200: `{"amount_minor": 15000, "created_at": "2026-04-24T00:00:00Z", "currency": "VND", "order_id": "3fa85f64-5717-4562-b3fc-2c`
- `POST /v1/commerce/orders/{orderId}/vend/failure` — Finalize vend failure
  - Request: `{"failure_reason": "motor_timeout", "slot_index": 3}`
  - Expected 200: `{"local_cash_refund_required": false, "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "failed", "ref`

### 19_Promotions_PriceBooks

- `GET /v1/admin/price-books` — List price books (admin catalog)
  - Expected 200: `{"items": [{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:0`
- `POST /v1/admin/price-books` — Create price book
  - Request: `{"currency": "USD", "effectiveFrom": "2026-04-01T00:00:00Z", "isDefault": false, "name": "canary-name-{{$guid}}", "prior`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id": `
- `GET /v1/admin/price-books/{priceBookId}` — Get price book by ID
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id": `
- `PATCH /v1/admin/price-books/{priceBookId}` — Patch price book
  - Request: `{"priority": 20}`
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id": `
- `POST /v1/admin/price-books/{priceBookId}/activate` — Activate price book
  - Expected 200: `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id": `
- `POST /v1/admin/price-books/{priceBookId}/archive` — Archive price book (deactivate)
  - Expected 200: `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id":`
- `POST /v1/admin/price-books/{priceBookId}/assign-target` — Assign company price book to machine or site
  - Request: `{"machineId": "{{machineId}}"}`
  - Expected 200: `{"createdAt": "2026-04-24T12:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "machineId": "7c9e6679-7425-40de-944`
- `POST /v1/admin/price-books/{priceBookId}/deactivate` — Deactivate price book
  - Expected 200: `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id":`
- `GET /v1/admin/price-books/{priceBookId}/items` — List price book items
  - Expected 200: `{"items": [{"priceBookId": "11111111-2222-3333-4444-555555555555", "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", `
- `PUT /v1/admin/price-books/{priceBookId}/items` — Replace price book items
  - Request: `{"items": [{"productId": "{{productId}}", "unitPriceMinor": 150}]}`
  - Expected 204: `{}`
- `DELETE /v1/admin/price-books/{priceBookId}/items/{productId}` — Delete price book item
  - Expected 204: `""`
- `PATCH /v1/admin/price-books/{priceBookId}/items/{productId}` — Upsert one price book item
  - Request: `{"unitPriceMinor": 175}`
  - Expected 200: `{"priceBookId": "11111111-2222-3333-4444-555555555555", "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", "unitPriceM`
- `DELETE /v1/admin/price-books/{priceBookId}/targets/{targetId}` — Remove price book target assignment
  - Expected 204: `""`
- `GET /v1/admin/promotions` — List promotions (admin catalog)
  - Expected 200: `{"items": [{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "`
- `POST /v1/admin/promotions` — Create promotion (draft lifecycle)
  - Request: `{"endsAt": "2026-09-01T00:00:00Z", "name": "canary-name-{{$guid}}", "priority": 10, "rules": [{"payload": {"percent": 10`
  - Expected 200: `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10`
- `POST /v1/admin/promotions/preview` — Preview promotion discounts on top of catalog pricing
  - Request: `{"machineId": "{{machineId}}", "productIds": ["9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff"]}`
  - Expected 200: `{"at": "2026-04-24T12:00:00.000000000Z", "lines": [{"appliedPromotionIds": ["10101010-1010-1010-1010-101010101010"], "ap`
- `GET /v1/admin/promotions/{promotionId}` — Get promotion detail with rules and targets
  - Expected 200: `{"promotion": {"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id"`
- `PATCH /v1/admin/promotions/{promotionId}` — Patch promotion fields or replace rules
  - Request: `{"name": "canary-name-{{$guid}}", "priority": 11}`
  - Expected 200: `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10`
- `POST /v1/admin/promotions/{promotionId}/activate` — Activate promotion (lifecycle active)
  - Expected 200: `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10`
- `POST /v1/admin/promotions/{promotionId}/archive` — Archive promotion (deactivate with audit trail)
  - Expected 200: `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10`
- `POST /v1/admin/promotions/{promotionId}/assign-target` — Assign a promotion target (company, site, machine, product, category, tag)
  - Request: `{"productId": "{{productId}}", "targetType": "product"}`
  - Expected 200: `{"createdAt": "2026-04-01T00:00:00Z", "id": "30303030-3030-3030-3030-303030303030", "productId": "9f1e2d3c-aaaa-bbbb-ccc`
- `POST /v1/admin/promotions/{promotionId}/deactivate` — Deactivate promotion
  - Expected 200: `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10`
- `POST /v1/admin/promotions/{promotionId}/pause` — Pause promotion
  - Expected 200: `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10`
- `DELETE /v1/admin/promotions/{promotionId}/targets/{targetId}` — Remove a promotion target assignment
  - Expected 204: `""`

### 20_Finance_Reconciliation

- `GET /v1/admin/finance/daily-close` — List finance daily closes
  - Expected 200: `{"items": [{"cashMinor": 60000, "closeDate": "2026-04-27", "createdAt": "2026-04-27T18:00:00.000000000Z", "discountMinor`
- `POST /v1/admin/finance/daily-close` — Create immutable finance daily close (requires Idempotency-Key)
  - Request: `{"closeDate": "2026-04-27", "timezone": "Asia/Bangkok"}`
  - Expected 201: `{"cashMinor": 60000, "closeDate": "2026-04-27", "createdAt": "2026-04-27T18:00:00.000000000Z", "discountMinor": 0, "fail`
- `GET /v1/admin/finance/daily-close/{closeId}` — Get one finance daily close by id
  - Expected 200: `{"cashMinor": 60000, "closeDate": "2026-04-27", "createdAt": "2026-04-27T18:00:00.000000000Z", "discountMinor": 0, "fail`
- `GET /v1/admin/reports/commands` — Machine command failure report (terminal attempts only)
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/admin/reports/export` — Unified CSV export dispatcher
  - Expected 200: `{}`
- `GET /v1/admin/reports/failed-vends` — Failed vend report
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/admin/reports/machine-health` — Machine health and offline report
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/admin/reports/machines` — Machine uptime / last-seen report (alias naming)
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/admin/reports/reconciliation` — Reconciliation BI (open/closed summaries + scoped cases)
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/admin/reports/reconciliation-queue` — Reconciliation queue report
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/admin/reports/sales` — Company sales report
  - Expected 200: `{"breakdown": [], "from": "2026-04-01T00:00:00Z", "groupBy": "day", "summary": {"avgOrderValueMinor": 200, "grossTotalMi`
- `GET /v1/admin/reports/sales-summary/export.csv` — Export sales summary as CSV (UTF-8)
  - Expected 200: `{}`
- `GET /v1/admin/reports/vends` — Company vend lifecycle summary
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"`
- `GET /v1/reports/fleet-health` — Machine posture and incident rollups
  - Expected 200: `{"from": "2026-04-01T00:00:00Z", "incidentsByStatus": [], "machineIncidentsBySeverity": [], "machineSummary": {"fault": `
- `GET /v1/reports/sales-summary` — Sales rollup and trend breakdown
  - Expected 200: `{"breakdown": [], "from": "2026-04-01T00:00:00Z", "groupBy": "day", "summary": {"avgOrderValueMinor": 200, "grossTotalMi`

### 21_Incidents_Diagnostics

- `GET /v1/admin/anomalies` — List operational anomalies
  - Expected 200: `{"items": [{"anomalyType": "machine_offline_too_long", "createdAt": "2026-04-29T12:00:00.000000000Z", "detectedAt": "202`
- `GET /v1/admin/anomalies/{anomalyId}` — Get operational anomaly
  - Expected 200: `{"anomalyType": "repeated_vend_failure", "createdAt": "2026-04-29T12:00:00.000000000Z", "detectedAt": "2026-04-29T12:00:`
- `POST /v1/admin/anomalies/{anomalyId}/ignore` — Ignore operational anomaly
  - Expected 200: `{"anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "ignored"}`
- `POST /v1/admin/anomalies/{anomalyId}/resolve` — Resolve operational anomaly
  - Expected 200: `{"anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "resolved"}`
- `GET /v1/admin/machines/{machineId}/diagnostics/bundles` — List machine diagnostic bundles
  - Expected 200: `{"items": [{"bundleId": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "commandId": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "co`
- `POST /v1/admin/machines/{machineId}/diagnostics/requests` — Request machine diagnostic bundle
  - Expected 202: `{"commandId": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "dispatchState": "published", "machineId": "7c9e6679-7425-40de-944`

### 22_OTA_Rollout

- `GET /v1/admin/machine-config/rollouts` — List machine config rollouts
  - Expected 200: `{"items": [{"createdAt": "2026-04-19T12:00:00.000000000Z", "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb", "scopeType": "c`
- `POST /v1/admin/machine-config/rollouts` — Create machine config rollout (or rollback)
  - Request: `{"scopeType": "company", "targetVersionId": "11111111-2222-3333-4444-555555555555"}`
  - Expected 201: `{"createdAt": "2026-04-19T12:00:00.000000000Z", "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb", "scopeType": "company", "s`
- `GET /v1/admin/machine-config/rollouts/{rolloutId}` — Get one machine config rollout
  - Expected 200: `{"createdAt": "2026-04-19T12:00:00.000000000Z", "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb", "scopeType": "company", "s`
- `GET /v1/admin/ota` — List OTA campaigns (admin)
  - Expected 200: `{"items": [{"artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactStorageKey": "org/acme/ota/fw.bin", "campaign`
- `GET /v1/admin/ota/campaigns` — List OTA campaigns (lifecycle admin)
  - Expected 200: `{"items": [{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver"`
- `POST /v1/admin/ota/campaigns` — Create OTA campaign (draft)
  - Request: `{"artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactVersion": "1.2.3", "campaignType": "firmware", "canaryPe`
  - Expected 201: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `GET /v1/admin/ota/campaigns/{campaignId}` — Get OTA campaign detail
  - Expected 200: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `PATCH /v1/admin/ota/campaigns/{campaignId}` — Patch draft/approved OTA campaign
  - Request: `{"name": "canary-name-{{$guid}}"}`
  - Expected 200: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `POST /v1/admin/ota/campaigns/{campaignId}/approve` — Approve OTA campaign
  - Expected 200: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `POST /v1/admin/ota/campaigns/{campaignId}/cancel` — Cancel OTA campaign
  - Expected 200: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `POST /v1/admin/ota/campaigns/{campaignId}/pause` — Pause active rollout
  - Expected 200: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `POST /v1/admin/ota/campaigns/{campaignId}/publish` — Publish OTA campaign (approve + start when needed)
  - Expected 200: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `GET /v1/admin/ota/campaigns/{campaignId}/results` — List campaign machine rollout results
  - Expected 200: `{"items": [{"commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "createdAt": "2026-04-19T12:00:00.000000000Z", "machine`
- `POST /v1/admin/ota/campaigns/{campaignId}/resume` — Resume paused rollout (remaining machines)
  - Expected 200: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `POST /v1/admin/ota/campaigns/{campaignId}/rollback` — Rollback OTA campaign (dispatch rollback commands)
  - Request: `{"rollbackArtifactId": "dddddddd-eeee-ffff-0000-333333333333"}`
  - Expected 200: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `POST /v1/admin/ota/campaigns/{campaignId}/start` — Start OTA rollout (canary first wave)
  - Expected 200: `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", `
- `GET /v1/admin/ota/campaigns/{campaignId}/targets` — List campaign machine targets
  - Expected 200: `{"items": [{"machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "state": "pending", "updatedAt": "2026-04-19T12:00:00.0`
- `PUT /v1/admin/ota/campaigns/{campaignId}/targets` — Replace campaign machine targets (draft/approved only)
  - Request: `{"machineIds": ["{{$guid}}"]}`
  - Expected 204: `{}`
- `GET /v1/admin/rollouts` — List rollout campaigns
  - Expected 200: `{"items": [], "meta": {"limit": 50, "offset": 0, "returned": 0}}`
- `POST /v1/admin/rollouts` — Create rollout campaign
  - Request: `{"rolloutType": "config_version", "strategy": {"canary_percent": 10, "confirm_full_rollout": false}, "targetVersion": "2`
  - Expected 201: `{"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType": "config_ver`
- `GET /v1/admin/rollouts/{rolloutId}` — Get rollout campaign
  - Expected 200: `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"`
- `POST /v1/admin/rollouts/{rolloutId}/cancel` — Cancel rollout
  - Expected 200: `{"campaign": {"cancelledAt": "2026-04-29T12:01:00.000000000Z", "createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9`
- `POST /v1/admin/rollouts/{rolloutId}/pause` — Pause rollout
  - Expected 200: `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"`
- `POST /v1/admin/rollouts/{rolloutId}/resume` — Resume rollout
  - Expected 200: `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"`
- `POST /v1/admin/rollouts/{rolloutId}/rollback` — Roll back rollout
  - Expected 200: `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"`
- `POST /v1/admin/rollouts/{rolloutId}/start` — Start rollout
  - Expected 200: `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"`

### 23_Audit_Logs

- `GET /v1/admin/audit/events` — List enterprise audit events
  - Expected 200: `{"items": [{"action": "catalog.product.update", "actorId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "actorType": "user", `
- `GET /v1/admin/audit/events/{auditEventId}` — Get one enterprise audit event by id
  - Expected 200: `{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"}`

### 99_Utilities

- `GET /v1/admin/artifacts` — List artifacts
  - Expected 200: `{"items": [], "meta": {"limit": 50, "offset": 0, "returned": 0, "totalCount": 0}}`
- `POST /v1/admin/artifacts` — Reserve artifact id
  - Expected 201: `{"artifact_id": "ffffffff-0000-1111-2222-333333333333", "upload_path": "org/acme/artifacts/ff/..."}`
- `DELETE /v1/admin/artifacts/{artifactId}` — Delete artifact
  - Expected 200: `{"artifact_id": "ffffffff-0000-1111-2222-333333333333", "status": "deleted"}`
- `GET /v1/admin/artifacts/{artifactId}` — Get artifact metadata
  - Expected 200: `{"artifact_id": "ffffffff-0000-1111-2222-333333333333", "status": "uploaded"}`
- `PUT /v1/admin/artifacts/{artifactId}/content` — Upload artifact bytes
  - Expected 200: `{"artifact_id": "11111111-2222-3333-4444-555555555555", "status": "uploaded"}`
- `GET /v1/admin/artifacts/{artifactId}/download` — Presigned download URL
  - Expected 200: `{"expires_at": "2026-04-19T13:00:00Z", "headers": {}, "method": "GET", "url": "https://storage.example/presigned-read"}`
- `GET /v1/admin/assignments` — List technician assignments (admin)
  - Expected 200: `{"items": [{"assignmentId": "dddddddd-eeee-ffff-0000-111111111111", "createdAt": "2026-04-01T00:00:00Z", "machineId": "7`
- `POST /v1/admin/assignments` — Create technician assignment
  - Request: `{"machine_id": "{{machineId}}", "role": "field_service", "technician_id": "{{$guid}}"}`
  - Expected 201: `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7`
- `DELETE /v1/admin/assignments/{assignmentId}` — Release assignment (delete)
  - Expected 200: `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7`
- `GET /v1/admin/assignments/{assignmentId}` — Get technician–machine assignment by id
  - Expected 200: `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7`
- `GET /v1/admin/auth/users` — List API accounts for an company (admin)
  - Expected 200: `{"items": [{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator`
- `POST /v1/admin/auth/users` — Create API account (admin)
  - Request: `{"email": "{{adminEmail}}", "password": "{{adminPassword}}", "roles": ["viewer"], "status": "active"}`
  - Expected 201: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `GET /v1/admin/auth/users/{accountId}` — Get API account by id (admin)
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `PATCH /v1/admin/auth/users/{accountId}` — Patch API account (admin)
  - Request: `{"status": "disabled"}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/auth/users/{accountId}/activate` — Activate API account (admin)
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/auth/users/{accountId}/deactivate` — Deactivate API account (admin)
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/auth/users/{accountId}/reset-password` — Reset password (admin)
  - Request: `{"password": "{{adminPassword}}"}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/auth/users/{accountId}/revoke-sessions` — Revoke API account sessions (admin)
  - Expected 204: `{}`
- `PATCH /v1/admin/auth/users/{accountId}/roles` — Replace API account roles — PATCH alias
  - Request: `{"roles": ["viewer"]}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `POST /v1/admin/auth/users/{accountId}/roles` — Replace API account roles (admin)
  - Request: `{"roles": ["viewer"]}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `PUT /v1/admin/auth/users/{accountId}/roles` — Replace API account roles (admin)
  - Request: `{"roles": ["viewer"]}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `GET /v1/admin/auth/users/{accountId}/sessions` — List sessions for an API account (admin)
  - Expected 200: `{"sessions": [{"createdAt": "2026-04-19T10:00:00Z", "expiresAt": "2026-05-19T12:00:00Z", "sessionId": "bbbbbbbb-bbbb-bbb`
- `PATCH /v1/admin/auth/users/{accountId}/status` — Patch API account status only
  - Request: `{"status": "disabled"}`
  - Expected 200: `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co`
- `GET /v1/admin/operations/machines/health` — Fleet machine health snapshot list
  - Expected 200: `{"items": [{"appVersion": "1.4.2", "catalogVersion": "2026-04-29T00:00:00Z", "configVersion": "7", "failedCommandCount":`
- `GET /v1/admin/ops/outbox` — List transactional outbox rows and pipeline stats
  - Expected 200: `{"meta": {"limit": 50, "offset": 0, "returned": 1, "total": 42}, "rows": [{"aggregateId": "7c9e6679-7425-40de-944b-e07fc`
- `POST /v1/admin/ops/outbox/{outboxId}/retry` — Reset a dead-lettered outbox row for retry
  - Expected 200: `{"retried": true}`
- `GET /v1/admin/ops/retention` — Show retention table visibility
  - Expected 200: `{"tables": [{"oldestRecordAgeDays": 28, "oldestRecordAt": "2026-04-01T00:00:00.000000000Z", "tableName": "outbox_events"`
- `POST /v1/admin/pricing/preview` — Preview effective prices for products
  - Request: `{"machineId": "{{machineId}}", "productIds": ["9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff"]}`
  - Expected 200: `{"at": "2026-04-24T12:00:00.000000000Z", "currency": "USD", "lines": [{"appliedRuleIds": ["price_book:11111111-2222-3333`
- `GET /v1/admin/provisioning/batches/{batchId}` — Get provisioning batch status
  - Expected 200: `{"batch": {"cabinetType": "ambient", "createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc`
- `POST /v1/admin/provisioning/machines/bulk` — Bulk provision machines at a site
  - Request: `{"cabinetType": "ambient", "generateActivationCodes": false, "machines": [{"model": "AVF-1", "name": "canary-name-{{$gui`
  - Expected 200: `{"status": "ok"}`
- `GET /v1/admin/system/outbox` — List transactional outbox rows (system alias)
  - Expected 200: `{"meta": {"limit": 50, "offset": 0, "returned": 1, "total": 42}, "rows": [{"aggregateId": "7c9e6679-7425-40de-944b-e07fc`
- `GET /v1/admin/system/outbox/stats` — Outbox pipeline statistics (system alias)
  - Expected 200: `{"stats": {"deadLetteredTotal": 1, "maxPendingAttempts": 5, "oldestPendingCreatedAt": "2026-04-19T12:00:00.000000000Z", `
- `GET /v1/admin/system/outbox/{eventId}` — Get one outbox row by id
  - Expected 200: `{"aggregateId": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "aggregateType": "payment", "attempts": 0, "createdAt": "2026-04`
- `POST /v1/admin/system/outbox/{eventId}/mark-dlq` — Manually move an outbox row to Postgres DLQ
  - Request: `{"note": "Operator confirmed upstream outage before manual DLQ"}`
  - Expected 200: `{"marked": true}`
- `POST /v1/admin/system/outbox/{eventId}/replay` — Replay a dead-lettered outbox row
  - Expected 200: `{"retried": true}`
- `POST /v1/admin/system/retention/dry-run` — Preview retention candidates (dry-run)
  - Expected 200: `{"enterprise": {"candidates": {"inventory_events": 9000, "outbox_events_published": 12}, "dryRun": true, "enabled": true`
- `POST /v1/admin/system/retention/run` — Run bounded Postgres retention
  - Expected 200: `{"enterprise": {"candidates": {"inventory_events": 9000, "outbox_events_published": 12}, "dryRun": true, "enabled": true`
- `GET /v1/admin/system/retention/stats` — Data retention policy + table footprints (system)
  - Expected 200: `{"policy": {"auditRetentionDays": 2555, "commandReceiptRetentionDays": 180, "commandRetentionDays": 180, "inventoryEvent`
- `GET /v1/admin/technician-assignments` — List technician assignments (alternate path)
  - Expected 200: `{"items": [{"assignmentId": "dddddddd-eeee-ffff-0000-111111111111", "createdAt": "2026-04-01T00:00:00Z", "machineId": "7`
- `POST /v1/admin/technician-assignments` — Create technician–machine assignment (admin)
  - Request: `{"machine_id": "{{machineId}}", "role": "maintainer", "technician_id": "eeeeeeee-ffff-0000-1111-222222222222"}`
  - Expected 201: `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7`
- `DELETE /v1/admin/technician-assignments/{assignmentId}` — Release technician assignment (admin)
  - Expected 200: `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7`
- `GET /v1/admin/technician-assignments/{assignmentId}` — Get technician assignment by ID (admin)
  - Expected 200: `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7`
- `PATCH /v1/admin/technician-assignments/{assignmentId}` — Patch technician assignment (admin)
  - Request: `{"role": "lead"}`
  - Expected 200: `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7`
- `POST /v1/admin/technician-assignments/{assignmentId}/cancel` — Cancel technician assignment (admin)
  - Expected 200: `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7`
- `GET /v1/admin/technicians` — List technicians (admin)
  - Expected 200: `{"items": [{"createdAt": "2026-03-01T00:00:00Z", "displayName": "Alex Tech", "technicianId": "eeeeeeee-ffff-0000-1111-22`
- `POST /v1/admin/technicians` — Create technician (admin)
  - Request: `{"display_name": "canary-display-name-{{$guid}}", "email": "{{adminEmail}}"}`
  - Expected 201: `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Tech", "id": "eeeeeeee-ffff-0000-1111-22222222222`
- `GET /v1/admin/technicians/{technicianId}` — Get technician by ID (admin)
  - Expected 200: `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Tech", "id": "eeeeeeee-ffff-0000-1111-22222222222`
- `PATCH /v1/admin/technicians/{technicianId}` — Patch technician (admin)
  - Request: `{"display_name": "canary-display-name-{{$guid}}"}`
  - Expected 200: `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Field", "id": "eeeeeeee-ffff-0000-1111-2222222222`
- `POST /v1/admin/technicians/{technicianId}/disable` — Disable technician (admin)
  - Expected 200: `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Tech", "id": "eeeeeeee-ffff-0000-1111-22222222222`
- `POST /v1/admin/technicians/{technicianId}/enable` — Enable technician (admin)
  - Expected 200: `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Tech", "id": "eeeeeeee-ffff-0000-1111-22222222222`

## 5. Test gRPC APIs individually

gRPC items are **manual test folders** under each domain's `gRPC` subfolder.

### 01_Auth

- `/avf.machine.v1.MachineAuthService/ActivateMachine` — metadata: Bearer token + x-request-id
  - Request: `{"claim": {}}`
- `/avf.machine.v1.MachineAuthService/ClaimActivation` — metadata: Bearer token + x-request-id
  - Request: `{"claim": {}}`
- `/avf.machine.v1.MachineAuthService/RefreshMachineToken` — metadata: Bearer token + x-request-id
  - Request: `{"refresh": {}}`
- `/avf.machine.v1.MachineTokenService/RefreshMachineToken` — metadata: Bearer token + x-request-id
  - Request: `{"refresh_token": ""}`

### 03_Catalog

- `/avf.internal.v1.InternalCatalogQueryService/GetSaleCatalogSnapshot` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}", "include_unavailable": false, "include_images": false, "if_none_matc`
- `/avf.machine.v1.MachineCatalogService/AckCatalogVersion` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "acknowledged_catalog_version": ""}`
- `/avf.machine.v1.MachineCatalogService/GetCatalogDelta` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}", "basis_catalog_version": "", "meta": {}}`
- `/avf.machine.v1.MachineCatalogService/GetCatalogSnapshot` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "machine_id": "{{machineId}}", "include_unavailable": false, "include_images": false, "`
- `/avf.machine.v1.MachineCatalogService/GetMediaManifest` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}", "include_unavailable": false, "meta": {}}`
- `/avf.machine.v1.MachineCatalogService/GetSaleCatalog` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "machine_id": "{{machineId}}", "include_unavailable": false, "include_images": false, "`
- `/avf.machine.v1.MachineCatalogService/SyncCatalogBundle` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "machine_id": "{{machineId}}", "include_unavailable": false, "include_images": false, "`
- `/avf.machine.v1.MachineCatalogService/SyncSaleCatalog` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "machine_id": "{{machineId}}", "include_unavailable": false, "include_images": false, "`

### 07_Product_Media

- `/avf.machine.v1.MachineMediaService/AckMediaVersion` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "acknowledged_media_fingerprint": ""}`
- `/avf.machine.v1.MachineMediaService/GetMediaDelta` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}", "basis_media_fingerprint": "", "meta": {}, "include_unavailable": fa`
- `/avf.machine.v1.MachineMediaService/GetMediaManifest` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}", "include_unavailable": false, "meta": {}}`
- `/avf.machine.v1.MachineOfflineSyncService/GetSyncCursor` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}}`
- `/avf.machine.v1.MachineOfflineSyncService/PushOfflineEvents` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "events": []}`

### 10_Machines

- `/avf.internal.v1.InternalMachineQueryService/GetMachineCabinetSlotSummary` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.internal.v1.InternalMachineQueryService/GetMachineState` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.internal.v1.InternalMachineQueryService/GetMachineSummary` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.machine.v1.MachineCommandService/AckCommand` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "command_id": "{{$guid}}", "command_sequence": 0, "receipt_id": "{{$guid}}"}`
- `/avf.machine.v1.MachineCommandService/GetAssignedUpdate` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}}`
- `/avf.machine.v1.MachineCommandService/GetPendingCommands` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "after_command_sequence": 0, "limit": 0}`
- `/avf.machine.v1.MachineCommandService/RejectCommand` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "command_id": "{{$guid}}", "command_sequence": 0, "reason": ""}`
- `/avf.machine.v1.MachineCommandService/ReportDiagnosticBundleResult` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "request_id": "{{$guid}}", "storage_key": "", "storage_provider": "", "content_type": "`
- `/avf.machine.v1.MachineCommandService/ReportUpdateStatus` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "campaign_id": "{{$guid}}", "status": "", "error_message": ""}`
- `/avf.machine.v1.MachineOperatorService/CloseOperatorSession` — metadata: Bearer token + x-request-id
  - Request: `{"session_id": "{{$guid}}"}`
- `/avf.machine.v1.MachineOperatorService/HeartbeatOperatorSession` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "session_id": "{{$guid}}"}`
- `/avf.machine.v1.MachineOperatorService/LoginOperator` — metadata: Bearer token + x-request-id
  - Request: `{}`
- `/avf.machine.v1.MachineOperatorService/LogoutOperator` — metadata: Bearer token + x-request-id
  - Request: `{}`
- `/avf.machine.v1.MachineOperatorService/OpenOperatorSession` — metadata: Bearer token + x-request-id
  - Request: `{}`
- `/avf.machine.v1.MachineOperatorService/SubmitFillReport` — metadata: Bearer token + x-request-id
  - Request: `{"fill": {}}`
- `/avf.machine.v1.MachineOperatorService/SubmitStockAdjustment` — metadata: Bearer token + x-request-id
  - Request: `{"adjustment": {}}`

### 11_Machine_Provisioning

- `/avf.machine.v1.MachineActivationService/ClaimActivation` — metadata: Bearer token + x-request-id
  - Request: `{"activation_code": "", "device_fingerprint": {}}`
- `/avf.machine.v1.MachineBootstrapService/AckConfigVersion` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "acknowledged_config_version": 0, "acknowledged_planogram_version_id": "{{$guid}}"}`
- `/avf.machine.v1.MachineBootstrapService/CheckForUpdates` — metadata: Bearer token + x-request-id
  - Request: `{"catalog_fingerprint": "", "pricing_fingerprint": "", "planogram_fingerprint": "", "media_fingerpri`
- `/avf.machine.v1.MachineBootstrapService/CheckIn` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "boot_id": "{{$guid}}", "network_state": ""}`
- `/avf.machine.v1.MachineBootstrapService/GetBootstrap` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}}`

### 13_Telemetry

- `/avf.internal.v1.InternalTelemetryQueryService/GetLatestMachineTelemetry` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.machine.v1.MachineTelemetryService/CheckIn` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "machine_id": "{{machineId}}", "android_id": "{{$guid}}", "sim_serial": "", "package`
- `/avf.machine.v1.MachineTelemetryService/GetEventStatus` — metadata: Bearer token + x-request-id
  - Request: `{"idempotency_key": ""}`
- `/avf.machine.v1.MachineTelemetryService/PushCriticalEvent` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "meta": {}, "event": {}, "severity": ""}`
- `/avf.machine.v1.MachineTelemetryService/PushTelemetryBatch` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "meta": {}, "events": []}`
- `/avf.machine.v1.MachineTelemetryService/ReconcileEvents` — metadata: Bearer token + x-request-id
  - Request: `{"idempotency_keys": []}`
- `/avf.machine.v1.MachineTelemetryService/SubmitTelemetryBatch` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "events": []}`

### 14_Inventory

- `/avf.internal.v1.InternalInventoryQueryService/GetMachineSlotInventory` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.machine.v1.MachineInventoryService/AckInventorySync` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}, "sync_cursor": ""}`
- `/avf.machine.v1.MachineInventoryService/GetInventorySnapshot` — metadata: Bearer token + x-request-id
  - Request: `{"meta": {}}`
- `/avf.machine.v1.MachineInventoryService/GetPlanogram` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.machine.v1.MachineInventoryService/PushInventoryDelta` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "meta": {}, "reason": "", "lines": []}`
- `/avf.machine.v1.MachineInventoryService/SubmitFillReport` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "lines": []}`
- `/avf.machine.v1.MachineInventoryService/SubmitFillResult` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "lines": []}`
- `/avf.machine.v1.MachineInventoryService/SubmitInventoryAdjustment` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "reason": "", "lines": []}`
- `/avf.machine.v1.MachineInventoryService/SubmitRestock` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "lines": []}`
- `/avf.machine.v1.MachineInventoryService/SubmitStockAdjustment` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "reason": "", "lines": []}`
- `/avf.machine.v1.MachineInventoryService/SubmitStockSnapshot` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "lines": []}`

### 16_Orders

- `/avf.internal.v1.InternalCommerceQueryService/GetOrderPaymentVendState` — metadata: Bearer token + x-request-id
  - Request: `{"order_id": "{{$guid}}", "slot_index": 0}`
- `/avf.machine.v1.MachineCommerceService/AttachPaymentResult` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "provider": "", "payment_state": "", "amount_minor": 0, "cu`
- `/avf.machine.v1.MachineCommerceService/CancelOrder` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "reason": ""}`
- `/avf.machine.v1.MachineCommerceService/ConfirmCashPayment` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}"}`
- `/avf.machine.v1.MachineCommerceService/ConfirmVendSuccess` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "slot_index": 0, "correlation_id": "{{$guid}}"}`
- `/avf.machine.v1.MachineCommerceService/CreateCashCheckout` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}"}`
- `/avf.machine.v1.MachineCommerceService/CreateOrder` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "machine_id": "{{machineId}}", "product_id": "{{$guid}}", "slot": {}, "currency": ""`
- `/avf.machine.v1.MachineCommerceService/CreatePaymentSession` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "provider": "", "payment_state": "", "amount_minor": 0, "cu`
- `/avf.machine.v1.MachineCommerceService/GetOrder` — metadata: Bearer token + x-request-id
  - Request: `{"order_id": "{{$guid}}", "slot_index": 0}`
- `/avf.machine.v1.MachineCommerceService/GetOrderStatus` — metadata: Bearer token + x-request-id
  - Request: `{"order_id": "{{$guid}}", "slot_index": 0}`
- `/avf.machine.v1.MachineCommerceService/ReportVendFailure` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "slot_index": 0, "failure_reason": "", "correlation_id": "{`
- `/avf.machine.v1.MachineCommerceService/ReportVendSuccess` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "slot_index": 0, "correlation_id": "{{$guid}}"}`
- `/avf.machine.v1.MachineCommerceService/StartVend` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "slot_index": 0}`
- `/avf.machine.v1.MachineSaleService/AttachPayment` — metadata: Bearer token + x-request-id
  - Request: `{"payment_session": {}}`
- `/avf.machine.v1.MachineSaleService/CancelSale` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "reason": ""}`
- `/avf.machine.v1.MachineSaleService/CompleteVend` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "slot_index": 0, "correlation_id": "{{$guid}}"}`
- `/avf.machine.v1.MachineSaleService/ConfirmCashReceived` — metadata: Bearer token + x-request-id
  - Request: `{"payment": {}}`
- `/avf.machine.v1.MachineSaleService/CreateSale` — metadata: Bearer token + x-request-id
  - Request: `{"order": {}}`
- `/avf.machine.v1.MachineSaleService/FailVend` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "slot_index": 0, "failure_reason": "", "correlation_id": "{`
- `/avf.machine.v1.MachineSaleService/StartVend` — metadata: Bearer token + x-request-id
  - Request: `{"context": {}, "order_id": "{{$guid}}", "slot_index": 0}`

### 17_Payments

- `/avf.internal.v1.InternalPaymentQueryService/GetLatestPaymentForOrder` — metadata: Bearer token + x-request-id
  - Request: `{"order_id": "{{$guid}}"}`
- `/avf.internal.v1.InternalPaymentQueryService/GetPaymentById` — metadata: Bearer token + x-request-id
  - Request: `{"payment_id": "{{$guid}}"}`

### 20_Finance_Reconciliation

- `/avf.internal.v1.InternalReportingQueryService/GetSalesSummary` — metadata: Bearer token + x-request-id
  - Request: `{"from_rfc3339": "", "to_rfc3339": "", "group_by": ""}`

### 21_Incidents_Diagnostics

- `/avf.internal.v1.InternalTelemetryQueryService/GetMachineIncidentSummary` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}", "limit": 0}`

### 99_Utilities

- `/avf.v1.InternalCommerceQueryService/GetOrderPaymentVendState` — metadata: Bearer token + x-request-id
  - Request: `{"order_id": "{{$guid}}", "slot_index": 0}`
- `/avf.v1.InternalMachineQueryService/GetMachineCabinetSlotSummary` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.v1.InternalMachineQueryService/GetMachineState` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.v1.InternalMachineQueryService/GetMachineSummary` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.v1.InternalTelemetryQueryService/GetLatestMachineTelemetry` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}"}`
- `/avf.v1.InternalTelemetryQueryService/GetMachineIncidentSummary` — metadata: Bearer token + x-request-id
  - Request: `{"machine_id": "{{machineId}}", "limit": 0}`

**Postman gRPC UI:** New → gRPC Request → import `proto/avf/**/*.proto` → select service/method → paste JSON from item description.

## 6. Test MQTT topics individually

MQTT items are **manual test folders** under each domain's `MQTT` subfolder.

### 10_Machines

- `{{mqttTopicPrefix}}/+/presence` (publish)
- `{{mqttTopicPrefix}}/+/state/heartbeat` (publish)
- `{{mqttTopicPrefix}}/+/commands/receipt` (publish)
- `{{mqttTopicPrefix}}/+/commands/ack` (publish)
- `{{mqttTopicPrefix}}/machines/+/presence` (publish)
- `{{mqttTopicPrefix}}/machines/+/state/heartbeat` (publish)
- `{{mqttTopicPrefix}}/machines/+/commands/receipt` (publish)
- `{{mqttTopicPrefix}}/machines/+/commands/ack` (publish)
- `{{mqttTopicPrefix}}/{{machineId}}/commands/dispatch` (publish)
- `{{mqttTopicPrefix}}/{{machineId}}/commands/down` (publish)
- `{{mqttTopicPrefix}}/machines/{{machineId}}/commands` (publish)

### 12_Machine_Runtime_Config

- `{{mqttTopicPrefix}}/+/shadow/reported` (publish)
- `{{mqttTopicPrefix}}/+/shadow/desired` (publish)
- `{{mqttTopicPrefix}}/machines/+/shadow/reported` (publish)
- `{{mqttTopicPrefix}}/machines/+/shadow/desired` (publish)

### 13_Telemetry

- `{{mqttTopicPrefix}}/+/telemetry` (publish)
- `{{mqttTopicPrefix}}/+/telemetry/snapshot` (publish)
- `{{mqttTopicPrefix}}/+/telemetry/incident` (publish)
- `{{mqttTopicPrefix}}/machines/+/telemetry` (publish)
- `{{mqttTopicPrefix}}/machines/+/telemetry/snapshot` (publish)
- `{{mqttTopicPrefix}}/machines/+/telemetry/incident` (publish)
- `{{mqttTopicPrefix}}/machines/+/events` (publish)

### 14_Inventory

- `{{mqttTopicPrefix}}/+/events/inventory` (publish)
- `{{mqttTopicPrefix}}/machines/+/events/inventory` (publish)

### 16_Orders

- `{{mqttTopicPrefix}}/+/events/vend` (publish)
- `{{mqttTopicPrefix}}/+/events/cash` (publish)
- `{{mqttTopicPrefix}}/machines/+/events/vend` (publish)
- `{{mqttTopicPrefix}}/machines/+/events/cash` (publish)

## 7. Full flow tests after individual API tests

After verifying each module individually, run these business flows in order:

1. **Health/System** — `00_Health_System/REST` health/live, health/ready, version
2. **Auth** — login → capture accessToken → me → refresh
3. **Admin/RBAC** — companies, users, roles (canary only for writes)
4. **Catalog** — brands → categories → tags
5. **Product media** — media manifest / offline cache endpoints
6. **Products** — CRUD product
7. **Sites/Regions** — create site, assign region
8. **Machine provisioning** — create machine, activation code, claim
9. **Machine runtime** — config sync, check-in
10. **MQTT telemetry** — publish telemetry topic (canary machine)
11. **Commands** — REST dispatch → MQTT command topic → commands/ack
12. **Inventory** — stock levels, restock
13. **Planogram** — layout publish
14. **Orders** — commerce checkout / vend session
15. **Payments** — payment session, webhook mock
16. **Refunds** — refund request flow
17. **Promotions/Price books** — if enabled in tenant
18. **Finance** — reconciliation reports
19. **Incidents** — diagnostic bundle
20. **OTA** — rollout campaign (canary)
21. **Audit** — verify audit log entries

For each flow: set `canaryMode=true` and `readiness=true` before gated writes.

## 8. Coverage summary

| Protocol | Total (source) | Generated | Missing | Extra |
| --- | --- | --- | --- | --- |
| REST | 327 | 327 | 0 | 0 |
| gRPC | 86 | 86 | 0 | 0 |
| MQTT | 28 | 28 | 0 | 0 |

## 9. Accuracy evidence

- REST: `docs/swagger/swagger.json` (OpenAPI 3), examples from `schema_to_example()`
- gRPC: `proto/avf/**/*.proto`, request templates from `build_grpc_templates()`
- MQTT: `fix_mqtt_rows()` aligned with `internal/platform/mqtt/topics.go`, `docs/api/mqtt-contract.md`

## 10. Blockers or limitations

- **gRPC/MQTT native Postman import:** Collection uses **manual test item folders** (not native gRPC/MQTT request types). Use Postman Desktop gRPC/MQTT clients per item description.
- **gRPC response examples:** Shape notes from proto; live responses require running against listener.
- **Production writes:** Gated REST requests disabled by default; enable per-request after setting canary flags.

## 11. Troubleshooting

- **401/403:** Refresh token; verify RBAC role for admin endpoints
- **Missing variable:** Check environment; run login first for accessToken
- **Invalid UUID:** Use valid v4/v7 UUIDs in path params
- **Invalid JSON body:** Copy examples from item description; validate in JSON linter
- **gRPC TLS:** Set grpcTls=true and port 443 for production
- **MQTT auth failure:** Verify broker credentials and topic prefix ACL
- **MQTT no ACK:** Subscribe to ack topic before publishing command
- **Optional query params:** Disabled by default in collection; enable in Postman URL tab if needed

## 12. Final verdict

**COMPLETE_WITH_POSTMAN_GRPC_MQTT_MANUAL_STEPS**
