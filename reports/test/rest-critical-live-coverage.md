# REST Critical Live Coverage

- Generated: `2026-05-11T15:36:16Z`
- E2E run: `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449`
- Total critical checks: **30**
- Passed: **27**
- Partial/non-2xx: **3**

This report is scoped to critical P0/P1 REST live evidence and does not claim 100% OpenAPI live coverage.

| ID | Method | Path | Role | Status | Result | Evidence |
|---|---|---|---|---:|---|---|
| `system.health_live` | `GET` | `/health/live` | `public` | 200 | **pass** | `direct live probe` |
| `system.health_ready` | `GET` | `/health/ready` | `public` | 200 | **pass** | `direct live probe` |
| `system.version` | `GET` | `/version` | `public` | 200 | **pass** | `direct live probe` |
| `auth.login` | `POST` | `/v1/auth/login` | `interactive_admin` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\wa-login.response.json` |
| `auth.operator_login` | `POST` | `/v1/machines/c0d8ee42-4367-49de-a9c6-4f3ad99426ce/operator-sessions/login` | `operator` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\wa-operator-login.response.json` |
| `admin.site_create` | `POST` | `/v1/admin/organizations/11111111-1111-1111-1111-111111111111/sites` | `admin` | 201 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\wa-site-create.response.json` |
| `admin.machine_create` | `POST` | `/v1/admin/organizations/11111111-1111-1111-1111-111111111111/machines` | `admin` | 201 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\wa-machine-create.response.json` |
| `admin.product_create` | `POST` | `/v1/admin/products` | `admin` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\wa-product-create.response.json` |
| `admin.planogram_publish` | `POST` | `/v1/admin/machines/c0d8ee42-4367-49de-a9c6-4f3ad99426ce/planograms/publish` | `admin` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\wa-planogram-publish.response.json` |
| `admin.inventory_stock` | `POST` | `/v1/admin/machines/c0d8ee42-4367-49de-a9c6-4f3ad99426ce/stock-adjustments` | `admin` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\wa-stock.response.json` |
| `admin.inventory_topology` | `GET` | `/v1/admin/machines/c0d8ee42-4367-49de-a9c6-4f3ad99426ce/topology` | `admin` | 405 | **partial** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\inv-topology.response.json` |
| `machine.claim` | `POST` | `/v1/setup/activation-codes/claim` | `machine_setup` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-claim.response.json` |
| `machine.bootstrap` | `GET` | `/v1/setup/machines/c0d8ee42-4367-49de-a9c6-4f3ad99426ce/bootstrap` | `machine` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-bootstrap.response.json` |
| `machine.sale_catalog` | `GET` | `/v1/machines/c0d8ee42-4367-49de-a9c6-4f3ad99426ce/sale-catalog` | `machine` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-sale-catalog.response.json` |
| `commerce.cash_checkout` | `POST` | `/v1/commerce/cash-checkout` | `machine` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-cash-co.response.json` |
| `commerce.vend_start` | `POST` | `/v1/commerce/orders/192abe5a-0c57-46fb-975f-46ce73d2828e/vend/start` | `machine` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-vend-start.response.json` |
| `commerce.vend_success` | `POST` | `/v1/commerce/orders/192abe5a-0c57-46fb-975f-46ce73d2828e/vend/success` | `machine` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-vend-ok.response.json` |
| `commerce.vend_failure` | `POST` | `/v1/commerce/orders/33edbdbd-30ba-4e3c-aa22-100857328e78/vend/failure` | `machine` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-fail-vfail.response.json` |
| `commerce.refund` | `POST` | `/v1/commerce/orders/33edbdbd-30ba-4e3c-aa22-100857328e78/refunds` | `machine` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-fail-refund.response.json` |
| `commerce.idempotency_replay_a` | `POST` | `/v1/commerce/orders` | `machine` | 201 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-idem-a.response.json` |
| `commerce.idempotency_replay_b` | `POST` | `/v1/commerce/orders` | `machine` | 201 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\vm-idem-b.response.json` |
| `payment.qr_order` | `POST` | `/v1/commerce/orders` | `machine` | 201 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\p8-qr-order.response.json` |
| `payment.qr_session` | `POST` | `/v1/commerce/orders/b91dd1cc-a705-4e45-b673-6c2eab3bf6cf/payment-session` | `machine` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\p8-qr-ps.response.json` |
| `payment.webhook_signed` | `GET` | `` | `payment_provider` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\p8-qr-wh1.response.json` |
| `payment.webhook_replay` | `GET` | `` | `payment_provider` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\p8-qr-wh2.response.json` |
| `diagnostics.machine_health` | `GET` | `/v1/admin/organizations/11111111-1111-1111-1111-111111111111/machines/c0d8ee42-4367-49de-a9c6-4f3ad99426ce/health` | `admin` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\p8-40-machine-health.response.json` |
| `reporting.audit_events` | `GET` | `/v1/admin/audit/events` | `admin` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\rpt-audit-events.response.json` |
| `reporting.finance_close` | `GET` | `/v1/admin/finance/daily-close` | `admin` | 200 | **pass** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\rpt-finance-close-list.response.json` |
| `media.artifacts_list` | `GET` | `/v1/admin/organizations/11111111-1111-1111-1111-111111111111/artifacts` | `admin` | 404 | **partial** | `D:\admin\development\avf\avf-vending-system\avf-vending-api\.e2e-runs\run-20260511T143948Z-409-30449\rest\rpt-artifacts-list.response.json` |
| `remote_command.dispatch_ack` | `GET` | `` | `admin` | 0 | **partial** | `direct live probe` |
