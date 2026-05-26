# Postman enterprise API inventory

Generated: 2026-05-26T20:49:39Z
Repository SHA: `local`

## Summary

| Surface | Count |
|---------|------:|
| REST (manifest + coverage) | 114 |
| gRPC flows | 15 |
| MQTT flows | 12 |

## REST classifications

- **EXCLUDED_UNHAPPY**: 152
- **ONLINE_PAYMENT_EXCLUDED**: 6
- **OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT**: 67
- **REST_TOTAL**: 115
- **RUNNABLE**: 41

## REST endpoints

| Flow ID | Method | Path | Auth | Folder | Class |
|---------|--------|------|------|--------|-------|
| REST-AUDIT-001 | GET | `/v1/admin/audit/events?limit=10` | bearer_admin | Audit Logs Diagnostics/Admin Audit Logs | RUNNABLE |
| REST-AUTH-001 | POST | `/v1/auth/login` | none | Auth/Admin Login | RUNNABLE |
| REST-AUTH-002 | GET | `/v1/auth/me` | bearer_admin | Auth/Current User | RUNNABLE |
| REST-AUTH-LOGOUT | POST | `/v1/auth/logout` | bearer_admin | Auth/Logout | RUNNABLE |
| REST-AUTH-REFRESH | POST | `/v1/auth/refresh` | none | Auth/Refresh Token | RUNNABLE |
| REST-CATALOG-001 | POST | `/v1/admin/categories` | bearer_admin | Category/Create Category | RUNNABLE |
| REST-CATALOG-002 | POST | `/v1/admin/brands` | bearer_admin | Brand/Create Brand | RUNNABLE |
| REST-CATALOG-003 | POST | `/v1/admin/tags` | bearer_admin | Tag/Create Tag | RUNNABLE |
| REST-CATALOG-004 | POST | `/v1/admin/products` | bearer_admin | Product/Create Product | RUNNABLE |
| REST-CATALOG-005 | GET | `/v1/admin/products/{{productId}}` | bearer_admin | Product/Get Product | RUNNABLE |
| REST-COMMERCE-001 | POST | `/v1/commerce/orders` | bearer_machine | Online Payment Happy Case Guarded/Payment Reconciliation | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-002 | POST | `/v1/commerce/orders/{{orderId}}/payment-session?slot_index=1` | bearer_machine | Online Payment Happy Case Guarded/Payment Reconciliation | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-003 | POST | `/v1/commerce/orders/{{orderId}}/payments/{{paymentId}}/webhooks` | webhook_hmac | Online Payment Happy Case Guarded/Receive Successful Payment Webhook | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-004 | GET | `/v1/commerce/orders/{{orderId}}` | bearer_machine | Online Payment Happy Case Guarded/Payment Reconciliation | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-005 | POST | `/v1/commerce/orders/{{orderId}}/vend/start` | bearer_machine | Online Payment Happy Case Guarded/Payment Reconciliation | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-006 | POST | `/v1/commerce/orders/{{orderId}}/vend/success` | bearer_machine | Online Payment Happy Case Guarded/Payment Reconciliation | ONLINE_PAYMENT_EXCLUDED |
| REST-COV-CAT-LIST | GET | `/v1/admin/categories?limit=50` | bearer_admin | Category/List Categorys | RUNNABLE |
| REST-COV-GET-0021 | GET | `/v1/admin/activation-codes?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0022 | GET | `/v1/admin/anomalies?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0023 | GET | `/v1/admin/artifacts?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0024 | GET | `/v1/admin/assignments?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0025 | GET | `/v1/admin/brands?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0026 | GET | `/v1/admin/commands?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0027 | GET | `/v1/admin/commands/{{commandId}}` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0028 | GET | `/v1/admin/commerce/reconciliation?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0029 | GET | `/v1/admin/feature-flags?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0030 | GET | `/v1/admin/finance/daily-close?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0031 | GET | `/v1/admin/inventory/anomalies?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0032 | GET | `/v1/admin/inventory/low-stock?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0033 | GET | `/v1/admin/inventory/refill-suggestions?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0034 | GET | `/v1/admin/machine-config/rollouts?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0035 | GET | `/v1/admin/machines?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0036 | GET | `/v1/admin/machines/{{machineId}}` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0037 | GET | `/v1/admin/machines/{{machineId}}/activation-codes` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0038 | GET | `/v1/admin/machines/{{machineId}}/cash-collections` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0039 | GET | `/v1/admin/machines/{{machineId}}/cashbox` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0040 | GET | `/v1/admin/machines/{{machineId}}/diagnostics/bundles` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0041 | GET | `/v1/admin/machines/{{machineId}}/health` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0042 | GET | `/v1/admin/machines/{{machineId}}/inventory-events` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0043 | GET | `/v1/admin/machines/{{machineId}}/inventory/anomalies` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0044 | GET | `/v1/admin/machines/{{machineId}}/refill-suggestions` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0045 | GET | `/v1/admin/machines/{{machineId}}/technicians` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0046 | GET | `/v1/admin/machines/{{machineId}}/timeline` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0047 | GET | `/v1/admin/media?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0048 | GET | `/v1/admin/media/assets?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0049 | GET | `/v1/admin/media/assets/{{mediaId}}` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0050 | GET | `/v1/admin/media/{{mediaId}}` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0051 | GET | `/v1/admin/operations/machines/health?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0052 | GET | `/v1/admin/ops/outbox?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0053 | GET | `/v1/admin/ops/retention?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0054 | GET | `/v1/admin/orders/{{orderId}}/timeline?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0055 | GET | `/v1/admin/ota?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0056 | GET | `/v1/admin/ota/campaigns?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0057 | GET | `/v1/admin/price-books?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0058 | GET | `/v1/admin/products?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0059 | GET | `/v1/admin/promotions?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0060 | GET | `/v1/admin/refunds?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0061 | GET | `/v1/admin/reports/cash?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0062 | GET | `/v1/admin/reports/commands?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0063 | GET | `/v1/admin/reports/inventory?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0064 | GET | `/v1/admin/reports/inventory-low-stock?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0065 | GET | `/v1/admin/reports/machine-health?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0066 | GET | `/v1/admin/reports/machines?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0067 | GET | `/v1/admin/reports/payments?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0068 | GET | `/v1/admin/reports/reconciliation?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0069 | GET | `/v1/admin/reports/refunds?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0070 | GET | `/v1/admin/reports/sales?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0071 | GET | `/v1/admin/reports/vends?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0072 | GET | `/v1/admin/restock/suggestions?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0073 | GET | `/v1/admin/rollouts?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0074 | GET | `/v1/admin/sites?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0075 | GET | `/v1/admin/sites/{{siteId}}` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0076 | GET | `/v1/admin/system/outbox?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0077 | GET | `/v1/admin/system/outbox/stats?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0078 | GET | `/v1/admin/system/retention/stats?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0079 | GET | `/v1/admin/tags?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0080 | GET | `/v1/admin/technician-assignments?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0081 | GET | `/v1/admin/technicians?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0082 | GET | `/v1/auth/sessions?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0083 | GET | `/v1/machines/{{machineId}}/commands/receipts` | bearer_admin | Optional Contract Disabled/Legacy Machine HTTP | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0085 | GET | `/v1/operator-insights/users/action-attributions?user_principal={{adminUserId}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0086 | GET | `/v1/orders?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0087 | GET | `/v1/payments?limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0088 | GET | `/v1/reports/payments-summary?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | Optional Contract Disabled/Config Required Features | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-INV-001 | GET | `/v1/admin/machines/{{machineId}}/inventory` | bearer_admin | Machine/Get Machine | RUNNABLE |
| REST-MACHINE-001 | POST | `/v1/admin/machines` | bearer_admin | Machine/Create Machine | RUNNABLE |
| REST-MACHINE-002 | PATCH | `/v1/admin/machines/{{machineId}}` | bearer_admin | Machine/Update Machine | RUNNABLE |
| REST-MACHINE-003 | POST | `/v1/admin/machines/{{machineId}}/activation-codes` | bearer_admin | Activation/Create Activation Code | RUNNABLE |
| REST-MACHINE-004 | POST | `/v1/setup/activation-codes/claim` | none | Activation/Claim Activation | RUNNABLE |
| REST-MACHINE-005 | GET | `/v1/setup/machines/{{machineId}}/bootstrap` | bearer_machine | Route Coverage Happy Case/All Readonly Happy Case Routes | RUNNABLE |
| REST-MACHINE-006 | GET | `/v1/machines/{{machineId}}/sale-catalog?include_unavailable=true&include_images=true` | bearer_machine | Route Coverage Happy Case/All Readonly Happy Case Routes | RUNNABLE |
| REST-MEDIA-001 | POST | `/v1/admin/media/uploads/init` | bearer_admin | Media/Presigned Upload | RUNNABLE |
| REST-MEDIA-002 | POST | `/v1/admin/media/uploads/{{mediaId}}/complete` | bearer_admin | Media/Presigned Upload | RUNNABLE |
| REST-MEDIA-COMPLETE | POST | `/v1/admin/media/uploads/{{mediaId}}/complete` | bearer_admin | Media/Presigned Upload | RUNNABLE |
| REST-MEDIA-INIT | POST | `/v1/admin/media/uploads/init` | bearer_admin | Media/Presigned Upload | RUNNABLE |
| REST-MEDIA-INIT | POST | `/v1/admin/product-images` | bearer_admin | Media/Upload Product Image | RUNNABLE |
| REST-OP-001 | POST | `/v1/admin/machines/{{machineId}}/stock-adjustments` | bearer_admin | Stock Inventory/Restock Slot | RUNNABLE |
| REST-OP-002 | POST | `/v1/machines/{{machineId}}/operator-sessions/logout` | bearer_admin | Route Coverage Happy Case/All Readonly Happy Case Routes | RUNNABLE |
| REST-PLANO-000 | GET | `/v1/admin/planograms?limit=20` | bearer_admin | Route Coverage Happy Case/All Readonly Happy Case Routes | RUNNABLE |
| REST-PLANO-001 | POST | `/v1/admin/machines/{{machineId}}/operator-sessions/start` | bearer_admin | Operator Technician/Start Operator Session | RUNNABLE |
| REST-PLANO-002 | PUT | `/v1/admin/machines/{{machineId}}/topology` | bearer_admin | Topology/Create Or Update Topology | RUNNABLE |
| REST-PLANO-003 | PUT | `/v1/admin/machines/{{machineId}}/planograms/draft` | bearer_admin | Planogram/Create Planogram Draft | RUNNABLE |
| REST-PLANO-004 | POST | `/v1/admin/machines/{{machineId}}/planograms/publish` | bearer_admin | Planogram/Publish Planogram | RUNNABLE |
| REST-PLANO-005 | POST | `/v1/admin/machines/{{machineId}}/stock-adjustments` | bearer_admin | Stock Inventory/Restock Slot | RUNNABLE |
| REST-PLANO-006 | GET | `/v1/admin/machines/{{machineId}}/slots` | bearer_admin | Stock Inventory/Get Inventory | RUNNABLE |
| REST-PREFLIGHT-001 | GET | `/health/live` | none | Health Version/Health Live | RUNNABLE |
| REST-PREFLIGHT-002 | GET | `/health/ready` | none | Health Version/Health Ready | RUNNABLE |
| REST-PREFLIGHT-003 | GET | `/version` | none | Health Version/Version | RUNNABLE |
| REST-REPORT-001 | GET | `/v1/reports/inventory-exceptions?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z&exception_kind=low_stock&limit=5` | bearer_admin | Reports/Inventory Report | RUNNABLE |
| REST-REPORT-002 | GET | `/v1/reports/fleet-health?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z` | bearer_admin | Reports/Fleet Health Report | RUNNABLE |
| REST-REPORT-003 | GET | `/v1/admin/reports/failed-vends?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z&limit=5` | bearer_admin | Reports/Machine Activity Report | RUNNABLE |
| REST-REPORT-004 | GET | `/v1/admin/reports/reconciliation-queue?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z&limit=5` | bearer_admin | Reports/Machine Activity Report | RUNNABLE |
| REST-REPORT-005 | GET | `/v1/reports/sales-summary?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z&group_by=none` | bearer_admin | Reports/Sales Report | RUNNABLE |
| REST-SITE-001 | POST | `/v1/admin/sites` | bearer_admin | Site/Create Site | RUNNABLE |

## gRPC methods

| Flow ID | Service.RPC | Phase | Source |
|---------|-------------|-------|--------|
| GRPC-TOKEN-001 | MachineTokenService/RefreshMachineToken | grpc-auth | e2e-manifest-grpc.yaml |
| GRPC-BOOT-001 | MachineBootstrapService/GetBootstrap | grpc-bootstrap | e2e-manifest-grpc.yaml |
| GRPC-BOOT-002 | MachineBootstrapService/CheckIn | grpc-bootstrap | e2e-manifest-grpc.yaml |
| GRPC-CAT-001 | catalog_sync_assert/ | grpc-catalog | e2e-manifest-grpc.yaml |
| GRPC-CAT-002 | MachineCatalogService/GetCatalogDelta | grpc-catalog | e2e-manifest-grpc.yaml |
| GRPC-CAT-003 | MachineCatalogService/AckCatalogVersion | grpc-catalog | e2e-manifest-grpc.yaml |
| GRPC-MED-001 | media_download_cache/ | grpc-media | e2e-manifest-grpc.yaml |
| GRPC-INV-001 | MachineInventoryService/GetInventorySnapshot | grpc-inventory | e2e-manifest-grpc.yaml |
| GRPC-INV-002 | MachineInventoryService/AckInventorySync | grpc-inventory | e2e-manifest-grpc.yaml |
| GRPC-COMM-CASH-001 | commerce_cash/ | grpc-commerce | e2e-manifest-grpc.yaml |
| GRPC-COMM-QR-001 | commerce_qr/ | grpc-commerce | e2e-manifest-grpc.yaml |
| GRPC-COMM-FAIL-001 | commerce_vend_failure/ | grpc-commerce | e2e-manifest-grpc.yaml |
| GRPC-COMM-CANCEL-001 | commerce_cancel/ | grpc-commerce | e2e-manifest-grpc.yaml |
| GRPC-IDEM-001 | commerce_idempotency/ | grpc-idempotency | e2e-manifest-grpc.yaml |
| GRPC-OFFLINE-001 | offline_replay/ | grpc-offline | e2e-manifest-grpc.yaml |

## MQTT flows

| Flow ID | Phase | Handler | Topic key |
|---------|-------|---------|-----------|
| MQTT-CONN-001 | mqtt-connect | connect_valid |  |
| MQTT-CONN-002 | mqtt-connect | connect_invalid |  |
| MQTT-CMD-001 | mqtt-command | command_pipeline |  |
| MQTT-TEL-001 | mqtt-telemetry | telemetry_publish | heartbeat |
| MQTT-TEL-002 | mqtt-telemetry | telemetry_publish | presence |
| MQTT-TEL-003 | mqtt-telemetry | telemetry_publish | snapshot |
| MQTT-TEL-004 | mqtt-telemetry | telemetry_publish | inventory |
| MQTT-READ-001 | mqtt-readback | readback_reports |  |
| MQTT-NEG-001 | mqtt-negative | neg_wrong_machine_ack |  |
| MQTT-NEG-002 | mqtt-negative | neg_stale_ack |  |
| MQTT-NEG-003 | mqtt-negative | neg_malformed_telemetry |  |
| MQTT-NEG-004 | mqtt-negative | neg_duplicate_ack |  |

## Excluded / skipped

- Online payment / PSP / VietQR / refunds (suite `all-no-online-payment`)
- `GRPC-COMM-QR-001` unless `SKIP_GRPC_QR_WEBHOOK` unset and payment explicitly enabled
- Legacy machine HTTP when `PROD_E2E_SKIP_LEGACY_MACHINE_HTTP=1`
- Presigned media path when production uses Cloudinary direct upload

