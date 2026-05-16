# Admin REST CRUD flow (post organization removal)

- Generated (UTC): `2026-05-16T10:33:10.189996+00:00`
- Run tag / suffix: `20260516T103309Z-f86bdd89ec64` / `f86bdd89ec64`
- `BASE_URL`: `http://127.0.0.1:18080`
- Cleanup enabled: `True` (`REST_ADMIN_CRUD_CLEANUP`)
- No `organization_id` query parameters were sent.
- Responses were scanned for JSON containing `organizationId`, `organization_id`, or tenant-style keys (`tenant`, `tenantId`, `tenants`).

## Created resource IDs

- **site_id**: `0620ce68-6530-4703-9266-94de4424f17d`
- **product_id**: `9b1d393e-4016-437d-9002-83d499f86c8a`
- **machine_id**: `1f6ffe59-399c-4c81-bec9-fd2e1a0d9549`

## Cleanup result

- `cleanup_machine_archive` → HTTP 200 **pass** (ok)
- `cleanup_product_delete` → HTTP 200 **pass** (ok)
- `cleanup_site_delete` → HTTP 400 **pass** (ok)

## Steps

| step | method | path | status | pass/fail | evidence |
| --- | --- | --- | --- | --- | --- |
| 01_login | POST | `/v1/auth/login` | 200 | pass | `01_login.json` |
| 02_site_create | POST | `/v1/admin/sites` | 201 | pass | `02_site_create.json` |
| 03_site_list | GET | `/v1/admin/sites` | 200 | pass | `03_site_list.json` |
| 04_site_get | GET | `/v1/admin/sites/0620ce68-6530-4703-9266-94de4424f17d` | 400 | pass | `04_site_get.json` |
| 05_site_patch | PATCH | `/v1/admin/sites/0620ce68-6530-4703-9266-94de4424f17d` | 200 | pass | `05_site_patch.json` |
| 06_product_create | POST | `/v1/admin/products` | 200 | pass | `06_product_create.json` |
| 07_product_list | GET | `/v1/admin/products` | 200 | pass | `07_product_list.json` |
| 08_product_get | GET | `/v1/admin/products/9b1d393e-4016-437d-9002-83d499f86c8a` | 200 | pass | `08_product_get.json` |
| 09_product_patch | PATCH | `/v1/admin/products/9b1d393e-4016-437d-9002-83d499f86c8a` | 200 | pass | `09_product_patch.json` |
| 10_machine_create | POST | `/v1/admin/machines` | 201 | pass | `10_machine_create.json` |
| 11_machine_list | GET | `/v1/admin/machines` | 500 | fail | `11_machine_list.json` |
| 12_machine_get | GET | `/v1/admin/machines/1f6ffe59-399c-4c81-bec9-fd2e1a0d9549` | 500 | fail | `12_machine_get.json` |
| 13_machine_patch | PATCH | `/v1/admin/machines/1f6ffe59-399c-4c81-bec9-fd2e1a0d9549` | 200 | pass | `13_machine_patch.json` |
| 14_machine_slots_get | GET | `/v1/admin/machines/1f6ffe59-399c-4c81-bec9-fd2e1a0d9549/slots` | 200 | pass | `14_machine_slots_get.json` |
| cleanup_machine_archive | POST | `/v1/admin/machines/1f6ffe59-399c-4c81-bec9-fd2e1a0d9549/archive` | 200 | pass | `cleanup_machine_archive.json` |
| cleanup_product_delete | DELETE | `/v1/admin/products/9b1d393e-4016-437d-9002-83d499f86c8a` | 200 | pass | `cleanup_product_delete.json` |
| cleanup_site_delete | DELETE | `/v1/admin/sites/0620ce68-6530-4703-9266-94de4424f17d` | 400 | pass | `cleanup_site_delete.json` |

## Final result

**FAIL**

## Notes

- Planogram slot **writes** (`PUT /v1/admin/machines/{id}/planograms/draft`) require an active **operator session**; this flow only exercises `GET .../slots` for machine layout reads.
- Product JSON types use `json:"scopeId,omitempty"` so an unset scope does not appear in payloads (see `V1AdminProduct` / `V1AdminProductListItem`).

## Files changed (this deliverable)

- `scripts/test/rest_admin_crud_flow.py`
- `reports/test/rest-admin-crud-flow-report.md`
- `reports/test/rest-admin-crud/*.json`
- `internal/httpserver/openapi_types.go` (`scopeId` omitempty on admin product DTOs)
