# Admin reporting API (P2.1)

Enterprise BI endpoints live under authenticated admin routes:

`GET /v1/admin/organizations/{organizationId}/reports/*`

All responses honor **[from, to)** on RFC3339 timestamps (required). Optional filters narrow rows:

| Query           | Applies to |
| ---             | --- |
| `site_id`       | Sales, payments, vends, inventory, machines, products, reconciliation BI, commands, technician/fill ops |
| `machine_id`    | Same |
| `product_id`    | Same where the underlying fact links to a product |
| `timezone`      | Day buckets for sales/payments JSON paths (same as legacy reports) |
| `format=csv`    | CSV export where implemented (audited as `reports.exported`) |

Pagination: **`limit`** / **`offset`** on paged reports (default cap enforced server-side).

## Canonical routes (P2.1)

| Report | GET path | Notes |
| --- | --- | --- |
| Sales | `/sales` | `group_by=day|site|machine|payment_method|product|none` |
| Payments | `/payments` | Provider settlement rollup (aggregate amounts, **no PAN / instrument**) |
| Vends | `/vends` | Totals + paginated failed vend drill-down |
| Inventory | `/inventory` | `kind=low_stock` (default) uses slot projections; `kind=movement` uses `inventory_events` |
| Machines | `/machines` | Uptime / last seen / offline heuristic |
| Products | `/products` | Product performance (vends + allocated revenue split) |
| Reconciliation | `/reconciliation` | `reconciliation_scope=open|closed|all` (default **all**) |
| Commands | `/commands` | Terminal failed/expired/NACK command attempts (**no** raw payloads) |
| Technician / fills | `/fills` | Restock, refill, operator-attributed, and related `inventory_events` (**no technician email/phone in CSV**) |

## Bulk CSV export

`GET .../reports/export?report=<name>` forces CSV using the same filters as the matching JSON route.

Supported `report` values: `sales`, `payments`, `products`, `reconciliation`, `machines`, `vends`, `inventory`, `commands`, `fills` (aliases: `technician_fills`).

## Legacy aliases (unchanged)

`/inventory-low-stock`, `/machine-health`, `/failed-vends`, `/reconciliation-queue`, `/cash`, `/refunds` remain available.

## Privacy and limits

- Exports **must not** include payment card or provider secret fields; payment routes expose **aggregates and statuses** only.
- Technician fill CSV includes **display name and UUIDs**, not email or phone.
- Very large windows remain bounded by **reporting max span** configuration (fail closed on excessive sync exports); use narrower `from`/`to` or contact ops for async patterns if introduced later.
