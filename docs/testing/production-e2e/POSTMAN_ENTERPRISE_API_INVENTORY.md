# Postman enterprise API inventory

Generated: 2026-05-26T15:15:44Z
Repository SHA: `local`

## Summary

| Surface | Count |
|---------|------:|
| REST (manifest + coverage) | 263 |
| gRPC flows | 15 |
| MQTT flows | 12 |

## REST classifications

- **CONFIG_REQUIRED**: 17
- **ONLINE_PAYMENT_EXCLUDED**: 9
- **OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT**: 67
- **REST_TOTAL**: 264
- **RUNNABLE**: 170

## REST endpoints

| Flow ID | Method | Path | Auth | Folder | Class |
|---------|--------|------|------|--------|-------|
| REST-AUDIT-001 | GET | `/v1/admin/audit/events?limit=10` | bearer_admin | 09 - Route Coverage Smoke/verify | RUNNABLE |
| REST-AUTH-001 | POST | `/v1/auth/login` | none | 02 - Auth/02.01 Login | CONFIG_REQUIRED |
| REST-AUTH-002 | GET | `/v1/auth/me` | bearer_admin | 02 - Auth/02.02 Current User | RUNNABLE |
| REST-AUTH-003 | POST | `/v1/auth/login` | none | 02 - Auth/02.99 Negative Auth | RUNNABLE |
| REST-AUTH-004 | GET | `/v1/auth/me` | none | 02 - Auth/02.99 Negative Auth | RUNNABLE |
| REST-AUTH-005 | GET | `/v1/admin/categories?limit=1` | bearer_machine | 09 - Route Coverage Smoke/provisioning | CONFIG_REQUIRED |
| REST-CATALOG-001 | POST | `/v1/admin/categories` | bearer_admin | 03 - Admin Catalog/Categories | CONFIG_REQUIRED |
| REST-CATALOG-002 | POST | `/v1/admin/brands` | bearer_admin | 03 - Admin Catalog/Brands | CONFIG_REQUIRED |
| REST-CATALOG-003 | POST | `/v1/admin/tags` | bearer_admin | 03 - Admin Catalog/Tags | CONFIG_REQUIRED |
| REST-CATALOG-004 | POST | `/v1/admin/products` | bearer_admin | 03 - Admin Catalog/Products | CONFIG_REQUIRED |
| REST-CATALOG-005 | GET | `/v1/admin/products/{{productId}}` | bearer_admin | 03 - Admin Catalog/Products | RUNNABLE |
| REST-COMMERCE-001 | POST | `/v1/commerce/orders` | bearer_machine | 08 - Commerce No-Online-Payment/Payment Excluded Contract | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-002 | POST | `/v1/commerce/orders/{{orderId}}/payment-session?slot_index=1` | bearer_machine | 08 - Commerce No-Online-Payment/Payment Excluded Contract | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-003 | POST | `/v1/commerce/orders/{{orderId}}/payments/{{paymentId}}/webhooks` | webhook_hmac | 08 - Commerce No-Online-Payment/Payment Excluded Contract | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-003-DUP | POST | `/v1/commerce/orders/{{orderId}}/payments/{{paymentId}}/webhooks` | webhook_hmac | 08 - Commerce No-Online-Payment/Payment Excluded Contract | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-004 | GET | `/v1/commerce/orders/{{orderId}}` | bearer_machine | 08 - Commerce No-Online-Payment/Payment Excluded Contract | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-005 | POST | `/v1/commerce/orders/{{orderId}}/vend/start` | bearer_machine | 08 - Commerce No-Online-Payment/Payment Excluded Contract | ONLINE_PAYMENT_EXCLUDED |
| REST-COMMERCE-006 | POST | `/v1/commerce/orders/{{orderId}}/vend/success` | bearer_machine | 08 - Commerce No-Online-Payment/Payment Excluded Contract | ONLINE_PAYMENT_EXCLUDED |
| REST-COV-DEL-0001 | DELETE | `/v1/admin/artifacts/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0002 | DELETE | `/v1/admin/assignments/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0003 | DELETE | `/v1/admin/brands/{{brandId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0004 | DELETE | `/v1/admin/categories/{{categoryId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0005 | DELETE | `/v1/admin/machines/{{machineId}}/activation-codes/{{activationCode}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0006 | DELETE | `/v1/admin/machines/{{machineId}}/technicians/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0007 | DELETE | `/v1/admin/media/assets/{{mediaId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0008 | DELETE | `/v1/admin/media/{{mediaId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0009 | DELETE | `/v1/admin/price-books/00000000-0000-4000-8000-000000000099/items/{{productId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0010 | DELETE | `/v1/admin/price-books/00000000-0000-4000-8000-000000000099/targets/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0011 | DELETE | `/v1/admin/products/{{productId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0012 | DELETE | `/v1/admin/products/{{productId}}/image` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0013 | DELETE | `/v1/admin/products/{{productId}}/media/{{mediaId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0014 | DELETE | `/v1/admin/promotions/00000000-0000-4000-8000-000000000099/targets/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0015 | DELETE | `/v1/admin/sites/{{siteId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0016 | DELETE | `/v1/admin/tags/{{tagId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0017 | DELETE | `/v1/admin/technician-assignments/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0018 | DELETE | `/v1/admin/users/00000000-0000-4000-8000-000000000099/roles/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0019 | DELETE | `/v1/auth/sessions` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-DEL-0020 | DELETE | `/v1/auth/sessions/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-GET-0021 | GET | `/v1/admin/activation-codes?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0022 | GET | `/v1/admin/anomalies?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0023 | GET | `/v1/admin/artifacts?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0024 | GET | `/v1/admin/assignments?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0025 | GET | `/v1/admin/brands?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0026 | GET | `/v1/admin/commands?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0027 | GET | `/v1/admin/commands/{{commandId}}` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0028 | GET | `/v1/admin/commerce/reconciliation?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0029 | GET | `/v1/admin/feature-flags?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0030 | GET | `/v1/admin/finance/daily-close?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0031 | GET | `/v1/admin/inventory/anomalies?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0032 | GET | `/v1/admin/inventory/low-stock?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0033 | GET | `/v1/admin/inventory/refill-suggestions?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0034 | GET | `/v1/admin/machine-config/rollouts?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0035 | GET | `/v1/admin/machines?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0036 | GET | `/v1/admin/machines/{{machineId}}` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0037 | GET | `/v1/admin/machines/{{machineId}}/activation-codes` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0038 | GET | `/v1/admin/machines/{{machineId}}/cash-collections` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0039 | GET | `/v1/admin/machines/{{machineId}}/cashbox` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0040 | GET | `/v1/admin/machines/{{machineId}}/diagnostics/bundles` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0041 | GET | `/v1/admin/machines/{{machineId}}/health` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0042 | GET | `/v1/admin/machines/{{machineId}}/inventory-events` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0043 | GET | `/v1/admin/machines/{{machineId}}/inventory/anomalies` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0044 | GET | `/v1/admin/machines/{{machineId}}/refill-suggestions` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0045 | GET | `/v1/admin/machines/{{machineId}}/technicians` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0046 | GET | `/v1/admin/machines/{{machineId}}/timeline` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0047 | GET | `/v1/admin/media?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0048 | GET | `/v1/admin/media/assets?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0049 | GET | `/v1/admin/media/assets/{{mediaId}}` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0050 | GET | `/v1/admin/media/{{mediaId}}` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0051 | GET | `/v1/admin/operations/machines/health?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0052 | GET | `/v1/admin/ops/outbox?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0053 | GET | `/v1/admin/ops/retention?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0054 | GET | `/v1/admin/orders/{{orderId}}/timeline?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0055 | GET | `/v1/admin/ota?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0056 | GET | `/v1/admin/ota/campaigns?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0057 | GET | `/v1/admin/price-books?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0058 | GET | `/v1/admin/products?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0059 | GET | `/v1/admin/promotions?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0060 | GET | `/v1/admin/refunds?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0061 | GET | `/v1/admin/reports/cash?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0062 | GET | `/v1/admin/reports/commands?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0063 | GET | `/v1/admin/reports/inventory?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0064 | GET | `/v1/admin/reports/inventory-low-stock?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0065 | GET | `/v1/admin/reports/machine-health?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0066 | GET | `/v1/admin/reports/machines?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0067 | GET | `/v1/admin/reports/payments?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0068 | GET | `/v1/admin/reports/reconciliation?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0069 | GET | `/v1/admin/reports/refunds?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0070 | GET | `/v1/admin/reports/sales?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0071 | GET | `/v1/admin/reports/vends?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0072 | GET | `/v1/admin/restock/suggestions?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0073 | GET | `/v1/admin/rollouts?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0074 | GET | `/v1/admin/sites?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0075 | GET | `/v1/admin/sites/{{siteId}}` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0076 | GET | `/v1/admin/system/outbox?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0077 | GET | `/v1/admin/system/outbox/stats?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0078 | GET | `/v1/admin/system/retention/stats?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0079 | GET | `/v1/admin/tags?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0080 | GET | `/v1/admin/technician-assignments?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0081 | GET | `/v1/admin/technicians?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0082 | GET | `/v1/auth/sessions?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0083 | GET | `/v1/machines/{{machineId}}/commands/receipts` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0084 | GET | `/v1/machines/{{machineId}}/commands/00000000-0000-4000-8000-000000000099/status` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-GET-0085 | GET | `/v1/operator-insights/users/action-attributions?user_principal={{adminUserId}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0086 | GET | `/v1/orders?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0087 | GET | `/v1/payments?limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-GET-0088 | GET | `/v1/reports/payments-summary?from={{reportFrom}}&to={{reportTo}}&limit=1` | bearer_admin | 09 - Route Coverage Smoke | OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT |
| REST-COV-PAT-0089 | PATCH | `/v1/admin/auth/users/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0090 | PATCH | `/v1/admin/auth/users/00000000-0000-4000-8000-000000000099/roles` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0091 | PATCH | `/v1/admin/auth/users/00000000-0000-4000-8000-000000000099/status` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0092 | PATCH | `/v1/admin/brands/{{brandId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0093 | PATCH | `/v1/admin/categories/{{categoryId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0094 | PATCH | `/v1/admin/feature-flags/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0095 | PATCH | `/v1/admin/ota/campaigns/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0096 | PATCH | `/v1/admin/price-books/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0097 | PATCH | `/v1/admin/price-books/00000000-0000-4000-8000-000000000099/items/{{productId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0098 | PATCH | `/v1/admin/products/{{productId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0099 | PATCH | `/v1/admin/promotions/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0100 | PATCH | `/v1/admin/sites/{{siteId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0101 | PATCH | `/v1/admin/tags/{{tagId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0102 | PATCH | `/v1/admin/technician-assignments/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0103 | PATCH | `/v1/admin/technicians/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0104 | PATCH | `/v1/admin/users/00000000-0000-4000-8000-000000000099` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0105 | PATCH | `/v1/admin/users/00000000-0000-4000-8000-000000000099/roles` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PAT-0106 | PATCH | `/v1/admin/users/00000000-0000-4000-8000-000000000099/status` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0107 | POST | `/v1/admin/activation-codes` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0108 | POST | `/v1/admin/activation-codes/00000000-0000-4000-8000-000000000099/revoke` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0109 | POST | `/v1/admin/anomalies/00000000-0000-4000-8000-000000000099/ignore` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0110 | POST | `/v1/admin/anomalies/00000000-0000-4000-8000-000000000099/resolve` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0111 | POST | `/v1/admin/artifacts` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0112 | POST | `/v1/admin/assignments` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0113 | POST | `/v1/admin/auth/users` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0114 | POST | `/v1/admin/auth/users/00000000-0000-4000-8000-000000000099/activate` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0115 | POST | `/v1/admin/auth/users/00000000-0000-4000-8000-000000000099/deactivate` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0116 | POST | `/v1/admin/auth/users/00000000-0000-4000-8000-000000000099/reset-password` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0117 | POST | `/v1/admin/auth/users/00000000-0000-4000-8000-000000000099/revoke-sessions` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0118 | POST | `/v1/admin/auth/users/00000000-0000-4000-8000-000000000099/roles` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0119 | POST | `/v1/admin/commands/{{commandId}}/cancel` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0120 | POST | `/v1/admin/commands/{{commandId}}/retry` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0121 | POST | `/v1/admin/commerce/reconciliation/00000000-0000-4000-8000-000000000099/ignore` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0122 | POST | `/v1/admin/commerce/reconciliation/00000000-0000-4000-8000-000000000099/request-refund` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0123 | POST | `/v1/admin/commerce/reconciliation/00000000-0000-4000-8000-000000000099/resolve` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0124 | POST | `/v1/admin/feature-flags` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0125 | POST | `/v1/admin/feature-flags/00000000-0000-4000-8000-000000000099/disable` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0126 | POST | `/v1/admin/feature-flags/00000000-0000-4000-8000-000000000099/enable` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0127 | POST | `/v1/admin/finance/daily-close` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0128 | POST | `/v1/admin/inventory/anomalies/00000000-0000-4000-8000-000000000099/resolve` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0129 | POST | `/v1/admin/machine-config/rollouts` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0130 | POST | `/v1/admin/machines/{{machineId}}/archive` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0131 | POST | `/v1/admin/machines/{{machineId}}/cash-collections` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0132 | POST | `/v1/admin/machines/{{machineId}}/cash-collections/00000000-0000-4000-8000-000000000099/close` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0133 | POST | `/v1/admin/machines/{{machineId}}/commands` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0134 | POST | `/v1/admin/machines/{{machineId}}/diagnostics/requests` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0135 | POST | `/v1/admin/machines/{{machineId}}/disable` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0136 | POST | `/v1/admin/machines/{{machineId}}/enable` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0137 | POST | `/v1/admin/machines/{{machineId}}/inventory/reconcile` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0138 | POST | `/v1/admin/machines/{{machineId}}/mark-compromised` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0139 | POST | `/v1/admin/machines/{{machineId}}/resume` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0140 | POST | `/v1/admin/machines/{{machineId}}/retire` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0141 | POST | `/v1/admin/machines/{{machineId}}/revoke-credentials` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0142 | POST | `/v1/admin/machines/{{machineId}}/revoke-sessions` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0143 | POST | `/v1/admin/machines/{{machineId}}/revoke-token` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0144 | POST | `/v1/admin/machines/{{machineId}}/rotate-credential` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0145 | POST | `/v1/admin/machines/{{machineId}}/rotate-credentials` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0146 | POST | `/v1/admin/machines/{{machineId}}/rotate-token-version` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0147 | POST | `/v1/admin/machines/{{machineId}}/suspend` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0148 | POST | `/v1/admin/machines/{{machineId}}/sync` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0149 | POST | `/v1/admin/machines/{{machineId}}/technicians` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0150 | POST | `/v1/admin/machines/{{machineId}}/transfer-site` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0151 | POST | `/v1/admin/media/assets` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0152 | POST | `/v1/admin/media/external-images` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0153 | POST | `/v1/admin/media/uploads` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0154 | POST | `/v1/admin/media/{{mediaId}}/complete` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0155 | POST | `/v1/admin/ops/outbox/00000000-0000-4000-8000-000000000099/retry` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0156 | POST | `/v1/admin/orders/{{orderId}}/refunds` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0157 | POST | `/v1/admin/ota/campaigns` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0158 | POST | `/v1/admin/ota/campaigns/00000000-0000-4000-8000-000000000099/cancel` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0159 | POST | `/v1/admin/ota/campaigns/00000000-0000-4000-8000-000000000099/pause` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0160 | POST | `/v1/admin/ota/campaigns/00000000-0000-4000-8000-000000000099/resume` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0161 | POST | `/v1/admin/price-books` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0162 | POST | `/v1/admin/price-books/00000000-0000-4000-8000-000000000099/activate` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0163 | POST | `/v1/admin/price-books/00000000-0000-4000-8000-000000000099/archive` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0164 | POST | `/v1/admin/price-books/00000000-0000-4000-8000-000000000099/assign-target` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0165 | POST | `/v1/admin/price-books/00000000-0000-4000-8000-000000000099/deactivate` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0166 | POST | `/v1/admin/pricing/preview` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0167 | POST | `/v1/admin/products/{{productId}}/image` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0168 | POST | `/v1/admin/products/{{productId}}/media` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0169 | POST | `/v1/admin/promotions` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0170 | POST | `/v1/admin/promotions/preview` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0171 | POST | `/v1/admin/promotions/00000000-0000-4000-8000-000000000099/activate` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0172 | POST | `/v1/admin/promotions/00000000-0000-4000-8000-000000000099/archive` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0173 | POST | `/v1/admin/promotions/00000000-0000-4000-8000-000000000099/assign-target` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0174 | POST | `/v1/admin/promotions/00000000-0000-4000-8000-000000000099/deactivate` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0175 | POST | `/v1/admin/promotions/00000000-0000-4000-8000-000000000099/pause` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0176 | POST | `/v1/admin/provisioning/machines/bulk` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0177 | POST | `/v1/admin/rollouts` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0178 | POST | `/v1/admin/rollouts/00000000-0000-4000-8000-000000000099/cancel` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0179 | POST | `/v1/admin/rollouts/00000000-0000-4000-8000-000000000099/pause` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0180 | POST | `/v1/admin/rollouts/00000000-0000-4000-8000-000000000099/resume` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0181 | POST | `/v1/admin/rollouts/00000000-0000-4000-8000-000000000099/rollback` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0182 | POST | `/v1/admin/rollouts/00000000-0000-4000-8000-000000000099/start` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0183 | POST | `/v1/admin/sites/{{siteId}}/archive` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0184 | POST | `/v1/admin/sites/{{siteId}}/disable` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0185 | POST | `/v1/admin/system/outbox/00000000-0000-4000-8000-000000000099/mark-dlq` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0186 | POST | `/v1/admin/system/outbox/00000000-0000-4000-8000-000000000099/replay` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0187 | POST | `/v1/admin/technician-assignments` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0188 | POST | `/v1/admin/technician-assignments/00000000-0000-4000-8000-000000000099/cancel` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0189 | POST | `/v1/admin/technicians` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0190 | POST | `/v1/admin/technicians/00000000-0000-4000-8000-000000000099/disable` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0191 | POST | `/v1/admin/technicians/00000000-0000-4000-8000-000000000099/enable` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0192 | POST | `/v1/admin/users` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0193 | POST | `/v1/admin/users/00000000-0000-4000-8000-000000000099/disable` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0194 | POST | `/v1/admin/users/00000000-0000-4000-8000-000000000099/enable` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0195 | POST | `/v1/admin/users/00000000-0000-4000-8000-000000000099/reset-password` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0196 | POST | `/v1/admin/users/00000000-0000-4000-8000-000000000099/revoke-sessions` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0197 | POST | `/v1/admin/users/00000000-0000-4000-8000-000000000099/roles` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0198 | POST | `/v1/auth/change-password` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0199 | POST | `/v1/auth/password/change` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0200 | POST | `/v1/auth/password/reset/confirm` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-POS-0201 | POST | `/v1/auth/password/reset/request` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0202 | PUT | `/v1/admin/artifacts/00000000-0000-4000-8000-000000000099/content` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0203 | PUT | `/v1/admin/auth/users/00000000-0000-4000-8000-000000000099/roles` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0204 | PUT | `/v1/admin/brands/{{brandId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0205 | PUT | `/v1/admin/categories/{{categoryId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0206 | PUT | `/v1/admin/feature-flags/00000000-0000-4000-8000-000000000099/targets` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0207 | PUT | `/v1/admin/ota/campaigns/00000000-0000-4000-8000-000000000099/targets` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0208 | PUT | `/v1/admin/price-books/00000000-0000-4000-8000-000000000099/items` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0209 | PUT | `/v1/admin/products/{{productId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0210 | PUT | `/v1/admin/products/{{productId}}/image` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0211 | PUT | `/v1/admin/products/{{productId}}/media` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0212 | PUT | `/v1/admin/tags/{{tagId}}` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-COV-PUT-0213 | PUT | `/v1/admin/users/00000000-0000-4000-8000-000000000099/roles` | none | 09 - Route Coverage Smoke | RUNNABLE |
| REST-INV-001 | GET | `/v1/admin/machines/{{machineId}}/inventory` | bearer_admin | 09 - Route Coverage Smoke/verify | RUNNABLE |
| REST-MACHINE-001 | POST | `/v1/admin/machines` | bearer_admin | 09 - Route Coverage Smoke/provisioning | CONFIG_REQUIRED |
| REST-MACHINE-002 | PATCH | `/v1/admin/machines/{{machineId}}` | bearer_admin | 09 - Route Coverage Smoke/provisioning | RUNNABLE |
| REST-MACHINE-003 | POST | `/v1/admin/machines/{{machineId}}/activation-codes` | bearer_admin | 09 - Route Coverage Smoke/provisioning | RUNNABLE |
| REST-MACHINE-004 | POST | `/v1/setup/activation-codes/claim` | none | 09 - Route Coverage Smoke/provisioning | CONFIG_REQUIRED |
| REST-MACHINE-005 | GET | `/v1/setup/machines/{{machineId}}/bootstrap` | bearer_machine | 09 - Route Coverage Smoke/provisioning | CONFIG_REQUIRED |
| REST-MACHINE-006 | GET | `/v1/machines/{{machineId}}/sale-catalog?include_unavailable=true&include_images=true` | bearer_machine | 06 - Topology / Planogram / Stock/Planogram Draft | CONFIG_REQUIRED |
| REST-MEDIA-001 | POST | `/v1/admin/media/uploads/init` | bearer_admin | 04 - Admin Media/Cloudinary / Direct Backend Upload | CONFIG_REQUIRED |
| REST-MEDIA-002 | POST | `/v1/admin/media/uploads/{{mediaId}}/complete` | bearer_admin | 04 - Admin Media/Cloudinary / Direct Backend Upload | CONFIG_REQUIRED |
| REST-MEDIA-COMPLETE | POST | `/v1/admin/media/uploads/{{mediaId}}/complete` | bearer_admin | 09 - Route Coverage Smoke/misc | RUNNABLE |
| REST-MEDIA-INIT | POST | `/v1/admin/media/uploads/init` | bearer_admin | 09 - Route Coverage Smoke/misc | RUNNABLE |
| REST-MEDIA-INIT | POST | `/v1/admin/product-images` | bearer_admin | 09 - Route Coverage Smoke/misc | CONFIG_REQUIRED |
| REST-NEG-002 | POST | `/v1/commerce/orders/{{orderId}}/payments/{{paymentId}}/webhooks` | webhook_hmac_invalid | 08 - Commerce No-Online-Payment/Payment Excluded Contract | ONLINE_PAYMENT_EXCLUDED |
| REST-NEG-003 | POST | `/v1/commerce/orders/{{orderId}}/payments/{{paymentId}}/webhooks` | webhook_hmac_stale | 08 - Commerce No-Online-Payment/Payment Excluded Contract | ONLINE_PAYMENT_EXCLUDED |
| REST-OP-001 | POST | `/v1/admin/machines/{{machineId}}/stock-adjustments` | bearer_admin | 06 - Topology / Planogram / Stock/Planogram Draft | RUNNABLE |
| REST-OP-002 | POST | `/v1/machines/{{machineId}}/operator-sessions/logout` | bearer_admin | 06 - Topology / Planogram / Stock/Planogram Draft | CONFIG_REQUIRED |
| REST-PLANO-000 | GET | `/v1/admin/planograms?limit=20` | bearer_admin | 06 - Topology / Planogram / Stock/Planogram Draft | RUNNABLE |
| REST-PLANO-001 | POST | `/v1/admin/machines/{{machineId}}/operator-sessions/start` | bearer_admin | 06 - Topology / Planogram / Stock/Planogram Draft | RUNNABLE |
| REST-PLANO-002 | PUT | `/v1/admin/machines/{{machineId}}/topology` | bearer_admin | 06 - Topology / Planogram / Stock/Planogram Draft | RUNNABLE |
| REST-PLANO-003 | PUT | `/v1/admin/machines/{{machineId}}/planograms/draft` | bearer_admin | 06 - Topology / Planogram / Stock/Planogram Draft | CONFIG_REQUIRED |
| REST-PLANO-004 | POST | `/v1/admin/machines/{{machineId}}/planograms/publish` | bearer_admin | 06 - Topology / Planogram / Stock/Planogram Draft | CONFIG_REQUIRED |
| REST-PLANO-005 | POST | `/v1/admin/machines/{{machineId}}/stock-adjustments` | bearer_admin | 06 - Topology / Planogram / Stock/Planogram Draft | RUNNABLE |
| REST-PLANO-006 | GET | `/v1/admin/machines/{{machineId}}/slots` | bearer_admin | 06 - Topology / Planogram / Stock/Planogram Draft | RUNNABLE |
| REST-PREFLIGHT-001 | GET | `/health/live` | none | 01 - Health & Version | RUNNABLE |
| REST-PREFLIGHT-002 | GET | `/health/ready` | none | 01 - Health & Version | RUNNABLE |
| REST-PREFLIGHT-003 | GET | `/version` | none | 01 - Health & Version | RUNNABLE |
| REST-REPORT-001 | GET | `/v1/reports/inventory-exceptions?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z&exception_kind=low_stock&limit=5` | bearer_admin | 09 - Route Coverage Smoke/verify | RUNNABLE |
| REST-REPORT-002 | GET | `/v1/reports/fleet-health?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z` | bearer_admin | 09 - Route Coverage Smoke/verify | RUNNABLE |
| REST-REPORT-003 | GET | `/v1/admin/reports/failed-vends?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z&limit=5` | bearer_admin | 09 - Route Coverage Smoke/verify | RUNNABLE |
| REST-REPORT-004 | GET | `/v1/admin/reports/reconciliation-queue?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z&limit=5` | bearer_admin | 09 - Route Coverage Smoke/verify | RUNNABLE |
| REST-REPORT-005 | GET | `/v1/reports/sales-summary?from=2026-03-01T00:00:00Z&to=2026-05-23T23:59:59Z&group_by=none` | bearer_admin | 09 - Route Coverage Smoke/verify | RUNNABLE |
| REST-SITE-001 | POST | `/v1/admin/sites` | bearer_admin | 09 - Route Coverage Smoke/provisioning | CONFIG_REQUIRED |

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

