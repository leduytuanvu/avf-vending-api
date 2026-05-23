# Production REST route coverage

- **Generated:** `2026-05-23T08:20:24Z`
- **Swagger source:** `.tmp-swagger-prod.json`
- **Total routes:** 329
- **Coverage buckets:** `{'readonly_smoke': 89, 'documented_skip': 42, 'auth_negative': 160, 'success': 36, 'permission_negative': 2}`
- **Matrix JSON:** [`tests/e2e/production/generated/rest-route-matrix.json`](../../../tests/e2e/production/generated/rest-route-matrix.json)

## Coverage rules

Every production `method+path` from `/swagger/doc.json` must map to exactly one of:

| Kind | Meaning |
|------|---------|
| `success` | Happy path with E2E-PROD resources (main manifest) |
| `readonly_smoke` | GET/list probe with admin or machine token |
| `auth_negative` | Unauthenticated or invalid auth → 401/403/422 |
| `permission_negative` | Wrong principal → 403 |
| `documented_skip` | Not live-tested; reason required |

Regenerate: `python tests/e2e/production/scripts/generate_rest_route_matrix.py --fetch-swagger`

## Route index (sample — full list in JSON)

| Method | Path | Coverage | Flows | Postman | Skip reason |
|--------|------|----------|-------|---------|-------------|
| GET | `/health/live` | readonly_smoke | REST-PREFLIGHT-001 | True |  |
| GET | `/health/ready` | readonly_smoke | REST-PREFLIGHT-002 | True |  |
| GET | `/metrics` | documented_skip | — | False | Prometheus metrics on ops listener; scraped privately not vi |
| GET | `/swagger/doc.json` | documented_skip | — | False | OpenAPI documentation surface; not a customer/admin business |
| GET | `/swagger/index.html` | documented_skip | — | False | OpenAPI documentation surface; not a customer/admin business |
| GET | `/v1/admin/activation-codes` | readonly_smoke | REST-COV-GET-0021 | False |  |
| POST | `/v1/admin/activation-codes` | auth_negative | REST-COV-POS-0127 | False |  |
| POST | `/v1/admin/activation-codes/{codeId}/revoke` | auth_negative | REST-COV-POS-0128 | False |  |
| GET | `/v1/admin/anomalies` | readonly_smoke | REST-COV-GET-0022 | False |  |
| GET | `/v1/admin/anomalies/{anomalyId}` | documented_skip | — | False | No auto rule matched |
| POST | `/v1/admin/anomalies/{anomalyId}/ignore` | auth_negative | REST-COV-POS-0129 | False |  |
| POST | `/v1/admin/anomalies/{anomalyId}/resolve` | auth_negative | REST-COV-POS-0130 | False |  |
| GET | `/v1/admin/artifacts` | readonly_smoke | REST-COV-GET-0023 | False |  |
| POST | `/v1/admin/artifacts` | auth_negative | REST-COV-POS-0131 | False |  |
| DELETE | `/v1/admin/artifacts/{artifactId}` | auth_negative | REST-COV-DEL-0001 | False |  |
| GET | `/v1/admin/artifacts/{artifactId}` | documented_skip | — | False | No auto rule matched |
| PUT | `/v1/admin/artifacts/{artifactId}/content` | auth_negative | REST-COV-PUT-0235 | False |  |
| GET | `/v1/admin/artifacts/{artifactId}/download` | documented_skip | — | False | No auto rule matched |
| GET | `/v1/admin/assignments` | readonly_smoke | REST-COV-GET-0024 | False |  |
| POST | `/v1/admin/assignments` | auth_negative | REST-COV-POS-0132 | False |  |
| DELETE | `/v1/admin/assignments/{assignmentId}` | auth_negative | REST-COV-DEL-0002 | False |  |
| GET | `/v1/admin/assignments/{assignmentId}` | documented_skip | — | False | No auto rule matched |
| GET | `/v1/admin/audit/events` | success | REST-AUDIT-001 | True |  |
| GET | `/v1/admin/audit/events/{auditEventId}` | documented_skip | — | False | No auto rule matched |
| GET | `/v1/admin/auth/users` | readonly_smoke | REST-COV-GET-0025 | False |  |
| POST | `/v1/admin/auth/users` | auth_negative | REST-COV-POS-0133 | False |  |
| GET | `/v1/admin/auth/users/{accountId}` | documented_skip | — | False | No auto rule matched |
| PATCH | `/v1/admin/auth/users/{accountId}` | auth_negative | REST-COV-PAT-0109 | False |  |
| POST | `/v1/admin/auth/users/{accountId}/activate` | auth_negative | REST-COV-POS-0134 | False |  |
| POST | `/v1/admin/auth/users/{accountId}/deactivate` | auth_negative | REST-COV-POS-0135 | False |  |
| POST | `/v1/admin/auth/users/{accountId}/reset-password` | auth_negative | REST-COV-POS-0136 | False |  |
| POST | `/v1/admin/auth/users/{accountId}/revoke-sessions` | auth_negative | REST-COV-POS-0137 | False |  |
| PATCH | `/v1/admin/auth/users/{accountId}/roles` | auth_negative | REST-COV-PAT-0110 | False |  |
| POST | `/v1/admin/auth/users/{accountId}/roles` | auth_negative | REST-COV-POS-0138 | False |  |
| PUT | `/v1/admin/auth/users/{accountId}/roles` | auth_negative | REST-COV-PUT-0236 | False |  |
| GET | `/v1/admin/auth/users/{accountId}/sessions` | documented_skip | — | False | No auto rule matched |
| PATCH | `/v1/admin/auth/users/{accountId}/status` | auth_negative | REST-COV-PAT-0111 | False |  |
| GET | `/v1/admin/brands` | readonly_smoke | REST-COV-GET-0026 | False |  |
| POST | `/v1/admin/brands` | success | REST-CATALOG-002 | True |  |
| DELETE | `/v1/admin/brands/{brandId}` | auth_negative | REST-COV-DEL-0003 | False |  |
| PATCH | `/v1/admin/brands/{brandId}` | auth_negative | REST-COV-PAT-0112 | False |  |
| PUT | `/v1/admin/brands/{brandId}` | auth_negative | REST-COV-PUT-0237 | False |  |
| GET | `/v1/admin/categories` | permission_negative | REST-AUTH-005 | True |  |
| POST | `/v1/admin/categories` | success | REST-CATALOG-001 | True |  |
| DELETE | `/v1/admin/categories/{categoryId}` | auth_negative | REST-COV-DEL-0004 | False |  |
| PATCH | `/v1/admin/categories/{categoryId}` | auth_negative | REST-COV-PAT-0113 | False |  |
| PUT | `/v1/admin/categories/{categoryId}` | auth_negative | REST-COV-PUT-0238 | False |  |
| GET | `/v1/admin/commands` | readonly_smoke | REST-COV-GET-0027 | False |  |
| GET | `/v1/admin/commands/{commandId}` | readonly_smoke | REST-COV-GET-0028 | False |  |
| POST | `/v1/admin/commands/{commandId}/cancel` | auth_negative | REST-COV-POS-0139 | False |  |
| POST | `/v1/admin/commands/{commandId}/retry` | auth_negative | REST-COV-POS-0140 | False |  |
| GET | `/v1/admin/commerce/reconciliation` | readonly_smoke | REST-COV-GET-0029 | False |  |
| GET | `/v1/admin/commerce/reconciliation/{caseId}` | documented_skip | — | False | No auto rule matched |
| POST | `/v1/admin/commerce/reconciliation/{caseId}/ignore` | auth_negative | REST-COV-POS-0141 | False |  |
| POST | `/v1/admin/commerce/reconciliation/{caseId}/request-refund` | auth_negative | REST-COV-POS-0142 | False |  |
| POST | `/v1/admin/commerce/reconciliation/{caseId}/resolve` | auth_negative | REST-COV-POS-0143 | False |  |
| GET | `/v1/admin/feature-flags` | readonly_smoke | REST-COV-GET-0030 | False |  |
| POST | `/v1/admin/feature-flags` | auth_negative | REST-COV-POS-0144 | False |  |
| GET | `/v1/admin/feature-flags/{flagId}` | documented_skip | — | False | No auto rule matched |
| PATCH | `/v1/admin/feature-flags/{flagId}` | auth_negative | REST-COV-PAT-0114 | False |  |
| … | *269 more* | | | | |
