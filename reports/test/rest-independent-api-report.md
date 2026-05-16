# Independent REST API smoke report

- Generated (UTC): `2026-05-16T11:12:08.929314+00:00`
- `BASE_URL`: `http://127.0.0.1:18080`
- Login identity: `e2e-local-admin@invalid.local`
- Forbidden-field scan skips OpenAPI (`/swagger/doc.json`) and Prometheus (`/metrics`) because those payloads routinely contain schema strings that would false-positive.
- No `organization_id` / `organizationId` query parameters were sent on any request.

## Infra / auth notes

- None inferred automatically.

> Machine-scoped inventory GETs were skipped (no `REST_INDEPENDENT_MACHINE_ID` and no machine id parsed from `GET /v1/admin/machines`).

## Results

| method | path | status | pass/fail | evidence file |
| --- | --- | --- | --- | --- |
| GET | `/health/live` | 200 | pass | `get_health_live.json` |
| GET | `/health/ready` | 200 | pass | `get_health_ready.json` |
| GET | `/version` | 200 | pass | `get_version.json` |
| GET | `/swagger/doc.json` | 200 | pass | `get_swagger_doc_json.json` |
| GET | `/metrics` | 200 | pass | `get_metrics.json` |
| POST | `/v1/auth/login` | 200 | pass | `post_v1_auth_login.json` |
| GET | `/v1/auth/me` | 200 | pass | `get_v1_auth_me.json` |
| GET | `/v1/admin/sites` | 200 | pass | `get_v1_admin_sites.json` |
| GET | `/v1/admin/machines` | 500 | fail | `get_v1_admin_machines.json` |
| GET | `/v1/admin/products` | 200 | pass | `get_v1_admin_products.json` |
| GET | `/v1/admin/inventory/low-stock` | 200 | pass | `get_v1_admin_inventory_low_stock.json` |
| GET | `/v1/admin/inventory/refill-suggestions` | 200 | pass | `get_v1_admin_inventory_refill_suggestions.json` |
| GET | `/v1/orders` | 200 | pass | `get_v1_orders.json` |
| GET | `/v1/payments` | 200 | pass | `get_v1_payments.json` |
| GET | `/v1/admin/commerce/reconciliation` | 200 | pass | `get_v1_admin_commerce_reconciliation.json` |
| GET | `/v1/admin/payments/webhook-events` | 500 | fail | `get_v1_admin_payments_webhook_events.json` |
| GET | `/v1/admin/payments/settlements` | 500 | fail | `get_v1_admin_payments_settlements.json` |
| GET | `/v1/admin/payments/disputes` | 500 | fail | `get_v1_admin_payments_disputes.json` |
| GET | `/v1/reports/sales-summary` | 200 | pass | `get_v1_reports_sales_summary.json` |
| GET | `/v1/reports/payments-summary` | 200 | pass | `get_v1_reports_payments_summary.json` |
| GET | `/v1/reports/fleet-health` | 200 | pass | `get_v1_reports_fleet_health.json` |
| GET | `/v1/reports/inventory-exceptions` | 200 | pass | `get_v1_reports_inventory_exceptions.json` |
| GET | `/v1/admin/audit/events` | 200 | pass | `get_v1_admin_audit_events.json` |
| GET | `/v1/admin/ops/outbox` | 403 | pass | `get_v1_admin_ops_outbox.json` |
| GET | `/v1/admin/ops/retention` | 403 | pass | `get_v1_admin_ops_retention.json` |
| GET | `/v1/admin/system/outbox/stats` | 403 | pass | `get_v1_admin_system_outbox_stats.json` |
| GET | `/v1/admin/operations/machines/health` | 404 | pass | `get_v1_admin_operations_machines_health.json` |
| GET | `/v1/admin/sites/3a717e68-f109-4a9a-b28f-70cdc619f2ab` | 400 | pass | `get_v1_admin_sites_3a717e68_f109_4a9a_b28f_70cdc619f2ab.json` |

## Failing endpoints (summary)

- `GET` `/v1/admin/machines` — **http_500**
- `GET` `/v1/admin/payments/webhook-events` — **http_500**
- `GET` `/v1/admin/payments/settlements` — **http_500**
- `GET` `/v1/admin/payments/disputes` — **http_500**

## Consolidated root causes (this run)

1. Review per-endpoint `fail_reasons` in evidence JSON; no single consolidated failure was inferred automatically beyond the login/health probes.

## Code note (operations routes)

`mountAdminOperationsRoutes` is only referenced from `mountAdminCompanyFleetRoutes`, and `mountAdminCompanyFleetRoutes` has **no callers** in `internal/httpserver/server.go`. Even with a healthy API, **`GET /v1/admin/operations/machines/health` may 404** until that mount is wired.

## How to re-run

```bash
export BASE_URL=http://127.0.0.1:18080
export LOGIN_EMAIL=admin@local.test
export LOGIN_PASSWORD='...'
python scripts/test/rest_independent_api_smoke.py
```

## Files changed (this deliverable)

- `scripts/test/rest_independent_api_smoke.py` (runner)
- `reports/test/rest-independent-api-report.md` (this report)
- `reports/test/rest-independent/*.json` (per-request evidence)

## Environment blockers observed during authoring

- Docker Desktop daemon was unreachable (`docker ps` failed), so the compose stack in `deployments/docker/docker-compose.yml` could not be started from here.
- `goose` against `postgres://postgres:postgres@127.0.0.1:5432/avf_vending` failed with **password authentication failed** — local Postgres on `:5432` is present but credentials do not match the compose defaults.
