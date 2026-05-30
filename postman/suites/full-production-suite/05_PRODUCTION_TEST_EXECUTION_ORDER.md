# 05 — Production test execution order (integrated REST + gRPC + MQTT)

Auto-generated inventory counts: **REST OpenAPI operations 329**, **gRPC methods 86**, **MQTT flows 28**.

Postman Collection v2.1 runs **HTTP only**. gRPC uses **grpcurl** (`grpc/run-grpc-postman-adjacent.sh`). MQTT uses **mosquitto** (`mqtt/run-mqtt-postman-adjacent.sh`).

| Step | Protocol | Request name | operationId / method / topic | Exact request | Expected | Variables | Evidence |
|------|----------|--------------|------------------------------|---------------|----------|-----------|----------|
| 1 | REST | GET /health/live + /version | HealthLive / VersionGet | `GET {{baseUrl}}/health/live` · `GET {{baseUrl}}/version` | 200 JSON | baseUrl | `postman/suites/full-production-suite/evidence/` |
| 2 | REST | POST /v1/auth/login | AuthLogin | `POST {{baseUrl}}/v1/auth/login` with adminEmail/adminPassword | 200 tokens pair | adminEmail; adminPassword | `evidence/03_login_platform.json` |
| 3 | REST | GET /v1/auth/me | AuthMeGet | `GET {{baseUrl}}/v1/auth/me` Bearer accessToken | 200 principal JSON | accessToken | `evidence/04_auth_me.json` |
| 4 | REST | POST /v1/admin/categories | CatalogCategoryCreate | Body with slug/name/active + timestamp | 200 row; capture categoryId | accessToken | `evidence/` |
| 5 | REST | POST /v1/admin/brands | CatalogBrandCreate | Body with slug/name/active + timestamp | 200 row; capture brandId | accessToken | `evidence/` |
| 6 | REST | POST /v1/admin/tags | CatalogTagCreate | Body with slug/name/active + timestamp | 200 row; capture tagId | accessToken | `evidence/` |
| 7 | REST | POST /v1/admin/media/uploads/init | MediaUploadInit | filename + contentType + purpose product_image | 200 mediaId + upload URL | accessToken | `evidence/` |
| 8 | REST | POST /v1/admin/media/uploads/{mediaId}/complete | MediaUploadComplete | sizeBytes + sha256 + contentType | 200 variants JSON | mediaId | `evidence/` |
| 9 | REST | POST /v1/admin/products | CatalogProductCreate | sku + primaryMediaId + tagIds + status active | 200 product JSON incl. media.primary.variants | categoryId; brandId; tagId; mediaId | `evidence/` |
| 10 | REST | GET /v1/admin/products/{productId} | CatalogProductGet | Verify primaryMediaId, sha256, version, downloadUrl, tags | 200 enriched payload | productId | `evidence/` |
| 11 | REST | POST /v1/admin/sites | SiteCreate | Folder machines admin / gate local writes | 201 site id | accessToken; siteId | `evidence/` |
| 12 | REST | POST /v1/admin/machines + activation-codes + claim | MachineProvision / ActivationClaim | Mint code then `POST /v1/setup/activation-codes/claim` | machine JWT + broker hints | siteId; machineId; machineToken; activationCode | `evidence/` |
| 13 | REST | Topology + planogram draft/publish + sync | PlanogramPublish / Sync | `PUT topology` · `PUT planograms/draft` · `POST publish` · `POST sync` | 2xx command envelopes | machineId; productId | `evidence/` |
| 14 | gRPC | Catalog + media manifest RPCs | MachineCatalogService / MachineMediaService | `grpc/run-grpc-postman-adjacent.sh` + matrix rows | OK + JSON templates | grpcAddr; grpcUseReflection; machineToken | `grpc/evidence/12_catalog/` |
| 15 | MQTT | catalog.refresh publish + ACK | Outbound commands topic matrix | `mqtt/run-mqtt-postman-adjacent.sh` + payloads JSON | broker ACK + backend logs | mqttHost; mqttPort; mqttTopicPrefix; machineId | `mqtt/evidence/` |
| 16 | REST | Commerce order + payment-session + vend | CheckoutOrder / VendSuccess | `POST /v1/commerce/orders` chain | 201/200 happy path | machineToken; orderId | `evidence/` |
| 17 | REST | Inventory decrement verification | InventoryByMachine | `GET /v1/admin/machines/{id}/inventory` before/after vend | quantity delta | machineId; accessToken | `evidence/` |
| 18 | REST | Reporting + audit reads | Reports / AuditEvents | `GET /v1/admin/reports/*` · `GET /v1/admin/audit/events` | 200 CSV/JSON lists | accessToken | `evidence/` |

## Final evidence summary

Archive console transcripts + HTTP `.response` bodies under `postman/suites/full-production-suite/evidence/` (create locally; not shipped with secrets).
