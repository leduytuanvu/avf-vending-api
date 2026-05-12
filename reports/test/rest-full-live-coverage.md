# REST Full Live Coverage

- Generated At: `2026-05-12T08:23:35.359631+00:00`
- Mode: `local`
- Base Url: `http://127.0.0.1:18080`
- Total Operations: `365`
- Passed: `6`
- Failed: `0`
- Partial: `0`
- Blocked: `359`
- Evidence directory: `D:\admin\development\avf\avf-vending-system\avf-vending-api\reports\test\rest-full-live-evidence`

| Method | Path | Priority | Class | Status | HTTP | Reason |
|---|---|---|---|---|---:|---|
| GET | `/health/live` | P0 | safe-read | **pass** | 200 | HTTP evidence captured |
| GET | `/health/ready` | P0 | safe-read | **pass** | 200 | HTTP evidence captured |
| GET | `/metrics` | P2 | safe-read | **pass** | 200 | HTTP evidence captured |
| GET | `/swagger/doc.json` | P2 | safe-read | **pass** | 200 | HTTP evidence captured |
| GET | `/swagger/index.html` | P2 | safe-read | **pass** | 200 | HTTP evidence captured |
| GET | `/v1/admin/assignments` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/audit/events` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/auth/users` | P1 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/auth/users` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/auth/users/{accountId}` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/auth/users/{accountId}` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/auth/users/{accountId}/activate` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/auth/users/{accountId}/deactivate` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/auth/users/{accountId}/reset-password` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/auth/users/{accountId}/revoke-sessions` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/auth/users/{accountId}/roles` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/auth/users/{accountId}/roles` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/auth/users/{accountId}/roles` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/auth/users/{accountId}/sessions` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/auth/users/{accountId}/status` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/brands` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/brands` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| DELETE | `/v1/admin/brands/{brandId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/brands/{brandId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/brands/{brandId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/categories` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/categories` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| DELETE | `/v1/admin/categories/{categoryId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/categories/{categoryId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/categories/{categoryId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/commands` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/feature-flags` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/feature-flags` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/feature-flags/{flagId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/feature-flags/{flagId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/feature-flags/{flagId}/disable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/feature-flags/{flagId}/enable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/feature-flags/{flagId}/targets` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/finance/daily-close` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/finance/daily-close` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/finance/daily-close/{closeId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/inventory/low-stock` | P1 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/inventory/refill-suggestions` | P1 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/machine-config/rollouts` | P1 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/machine-config/rollouts` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/machine-config/rollouts/{rolloutId}` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines` | P1 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/machines` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/machines/{machineId}` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/machines/{machineId}` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines/{machineId}/activation-codes` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/activation-codes` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines/{machineId}/cash-collections` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/cash-collections` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines/{machineId}/cash-collections/{collectionId}` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/cash-collections/{collectionId}/close` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines/{machineId}/cashbox` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines/{machineId}/diagnostics/bundles` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/diagnostics/requests` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/disable` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/enable` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines/{machineId}/inventory` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines/{machineId}/inventory-events` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/machines/{machineId}/planograms/draft` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/planograms/publish` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines/{machineId}/refill-suggestions` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/retire` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/rotate-credential` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/machines/{machineId}/slots` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/stock-adjustments` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/machines/{machineId}/sync` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/machines/{machineId}/topology` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/media` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/media/assets` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/media/assets` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| DELETE | `/v1/admin/media/assets/{mediaId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/media/assets/{mediaId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/media/uploads` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| DELETE | `/v1/admin/media/{mediaId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/media/{mediaId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/media/{mediaId}/complete` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/ops/outbox` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/ops/outbox/{outboxId}/retry` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/ops/retention` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/organizations/{orgId}/artifacts` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{orgId}/artifacts` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/organizations/{orgId}/artifacts/{artifactId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{orgId}/artifacts/{artifactId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/organizations/{orgId}/artifacts/{artifactId}/content` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{orgId}/artifacts/{artifactId}/download` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/activation-codes` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/activation-codes` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/activation-codes/{codeId}/revoke` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/anomalies` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/anomalies/{anomalyId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/ignore` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/resolve` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/assignments` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/assignments` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/organizations/{organizationId}/assignments/{assignmentId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/assignments/{assignmentId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/audit-events` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/audit-events/{auditEventId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/commands` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/commands/{commandId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/commands/{commandId}/cancel` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/commands/{commandId}/retry` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/commerce/reconciliation` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}/ignore` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}/resolve` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/inventory/anomalies` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/inventory/anomalies/{anomalyId}/resolve` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/machines` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/machines/{machineId}` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/organizations/{organizationId}/machines/{machineId}` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/archive` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/commands` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/machines/{machineId}/health` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/machines/{machineId}/inventory/anomalies` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/inventory/reconcile` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/mark-compromised` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/resume` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/revoke-credentials` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/revoke-token` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/rotate-credentials` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/rotate-token-version` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/suspend` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians/{userId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/machines/{machineId}/timeline` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/machines/{machineId}/transfer-site` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/media/assets` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/organizations/{organizationId}/media/assets/{assetId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/media/assets/{assetId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/media/product-images` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/media/uploads/complete` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/media/uploads/init` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/operations/machines/health` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/orders/{orderId}/refunds` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/orders/{orderId}/timeline` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/products/{productId}/images` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/products/{productId}/images` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/products/{productId}/media` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/organizations/{organizationId}/products/{productId}/media/{mediaId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/provisioning/batches/{batchId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/provisioning/machines/bulk` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/refunds` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/refunds/{refundId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/cash` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/commands` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/export` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/failed-vends` | P0 | hardware-required | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/fills` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/inventory` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/inventory-low-stock` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/machine-health` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/machines` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/payments` | P0 | provider-required | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/products` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/reconciliation` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/reconciliation-queue` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/refunds` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/sales` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/reports/vends` | P0 | hardware-required | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/restock/suggestions` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/rollouts` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/rollouts` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/cancel` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/pause` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/resume` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/rollback` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/start` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/sites` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/sites` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/organizations/{organizationId}/sites/{siteId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/sites/{siteId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/organizations/{organizationId}/sites/{siteId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/sites/{siteId}/archive` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/technicians` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/technicians` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/technicians/{technicianId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/organizations/{organizationId}/technicians/{technicianId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/technicians/{technicianId}/disable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/technicians/{technicianId}/enable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/users` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/users` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/users/{userId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/organizations/{organizationId}/users/{userId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/users/{userId}/disable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/users/{userId}/enable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/users/{userId}/reset-password` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/users/{userId}/revoke-sessions` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/organizations/{organizationId}/users/{userId}/roles` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/organizations/{organizationId}/users/{userId}/roles` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/organizations/{organizationId}/users/{userId}/roles/{role}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/organizations/{organizationId}/users/{userId}/sessions` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/organizations/{organizationId}/users/{userId}/status` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/ota` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/ota/campaigns` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/ota/campaigns` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/ota/campaigns/{campaignId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/ota/campaigns/{campaignId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/ota/campaigns/{campaignId}/approve` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/ota/campaigns/{campaignId}/cancel` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/ota/campaigns/{campaignId}/pause` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/ota/campaigns/{campaignId}/publish` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/ota/campaigns/{campaignId}/results` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/ota/campaigns/{campaignId}/resume` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/ota/campaigns/{campaignId}/rollback` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/ota/campaigns/{campaignId}/start` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/ota/campaigns/{campaignId}/targets` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/ota/campaigns/{campaignId}/targets` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/planograms` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/planograms/{planogramId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/price-books` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/price-books` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/price-books/{priceBookId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/price-books/{priceBookId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/price-books/{priceBookId}/activate` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/price-books/{priceBookId}/archive` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/price-books/{priceBookId}/assign-target` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/price-books/{priceBookId}/deactivate` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/price-books/{priceBookId}/items` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/price-books/{priceBookId}/items` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/price-books/{priceBookId}/items/{productId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/price-books/{priceBookId}/items/{productId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/price-books/{priceBookId}/targets/{targetId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/pricing/preview` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/products` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/products` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| DELETE | `/v1/admin/products/{productId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/products/{productId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/products/{productId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/products/{productId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/products/{productId}/image` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/products/{productId}/image` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/products/{productId}/image` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/products/{productId}/media` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/products/{productId}/media` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/products/{productId}/media/{mediaId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/promotions` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/promotions` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/admin/promotions/preview` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/promotions/{promotionId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/promotions/{promotionId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/promotions/{promotionId}/activate` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/promotions/{promotionId}/archive` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/promotions/{promotionId}/assign-target` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/promotions/{promotionId}/deactivate` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/promotions/{promotionId}/pause` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| DELETE | `/v1/admin/promotions/{promotionId}/targets/{targetId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/reports/cash-collections/export.csv` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/reports/payments-summary/export.csv` | P0 | provider-required | **blocked-provider** |  | blocked-provider: external dependency required |
| GET | `/v1/admin/reports/sales-summary/export.csv` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/sites` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/sites` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| DELETE | `/v1/admin/sites/{siteId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/sites/{siteId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/sites/{siteId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/sites/{siteId}/disable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/system/outbox` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/system/outbox/stats` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/system/outbox/{eventId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/system/outbox/{eventId}/mark-dlq` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/system/outbox/{eventId}/replay` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/system/retention/dry-run` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/admin/system/retention/run` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/system/retention/stats` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/admin/tags` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/tags` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| DELETE | `/v1/admin/tags/{tagId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/tags/{tagId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/tags/{tagId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/technician-assignments` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/technician-assignments` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| DELETE | `/v1/admin/technician-assignments/{assignmentId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/technician-assignments/{assignmentId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/technician-assignments/{assignmentId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/technician-assignments/{assignmentId}/cancel` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/technicians` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/technicians` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/technicians/{technicianId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/technicians/{technicianId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/technicians/{technicianId}/disable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/technicians/{technicianId}/enable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/users` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/admin/users` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/admin/users/{userId}` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/users/{userId}` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/users/{userId}/disable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/users/{userId}/enable` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/users/{userId}/reset-password` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/users/{userId}/revoke-sessions` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/users/{userId}/roles` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/admin/users/{userId}/roles` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PUT | `/v1/admin/users/{userId}/roles` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/admin/users/{userId}/sessions` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| PATCH | `/v1/admin/users/{userId}/status` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/auth/change-password` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/auth/login` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/auth/logout` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/auth/me` | P1 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/auth/mfa/totp/disable` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/auth/mfa/totp/enroll` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/auth/mfa/totp/verify` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/auth/password/change` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/auth/password/reset/confirm` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/auth/password/reset/request` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/auth/refresh` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| DELETE | `/v1/auth/sessions` | P0 | destructive | **blocked-hardware** |  | blocked-hardware: destructive route requires explicit scenario guard |
| GET | `/v1/auth/sessions` | P1 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| DELETE | `/v1/auth/sessions/{sessionId}` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/commerce/cash-checkout` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| POST | `/v1/commerce/orders` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/commerce/orders/{orderId}` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/commerce/orders/{orderId}/cancel` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/commerce/orders/{orderId}/payment-session` | P0 | provider-required | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks` | P0 | provider-required | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/commerce/orders/{orderId}/reconciliation` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/commerce/orders/{orderId}/refunds` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/commerce/orders/{orderId}/refunds` | P0 | destructive | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/commerce/orders/{orderId}/refunds/{refundId}` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/commerce/orders/{orderId}/vend/failure` | P0 | hardware-required | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/commerce/orders/{orderId}/vend/start` | P0 | hardware-required | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/commerce/orders/{orderId}/vend/success` | P0 | hardware-required | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/device/machines/{machineId}/commands/poll` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/device/machines/{machineId}/events/reconcile` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/device/machines/{machineId}/events/{idempotencyKey}/status` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/device/machines/{machineId}/vend-results` | P0 | hardware-required | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/machines/{machineId}/check-ins` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/machines/{machineId}/commands/dispatch` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/commands/receipts` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/commands/{sequence}/status` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/machines/{machineId}/config-applies` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/operator-sessions/action-attributions` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/operator-sessions/auth-events` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/operator-sessions/current` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/operator-sessions/history` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/machines/{machineId}/operator-sessions/login` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/machines/{machineId}/operator-sessions/logout` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/operator-sessions/timeline` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| POST | `/v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat` | P0 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/sale-catalog` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/shadow` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/telemetry/incidents` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/telemetry/rollups` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/machines/{machineId}/telemetry/snapshot` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/operator-insights/technicians/{technicianId}/action-attributions` | P2 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/v1/operator-insights/users/action-attributions` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/orders` | P1 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/payments` | P0 | provider-required | **blocked-provider** |  | blocked-provider: external dependency required |
| GET | `/v1/reports/fleet-health` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/reports/inventory-exceptions` | P1 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| GET | `/v1/reports/payments-summary` | P0 | provider-required | **blocked-provider** |  | blocked-provider: external dependency required |
| GET | `/v1/reports/sales-summary` | P2 | safe-read | **blocked-missing-seed** | 401 | auth/role credentials required |
| POST | `/v1/setup/activation-codes/claim` | P2 | local-write | **blocked-missing-seed** |  | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| GET | `/v1/setup/machines/{machineId}/bootstrap` | P1 | safe-read | **blocked-missing-seed** |  | blocked-missing-seed: templated path requires seeded resource IDs |
| GET | `/version` | P0 | safe-read | **pass** | 200 | HTTP evidence captured |
