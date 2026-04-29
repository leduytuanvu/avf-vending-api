# Operational anomalies & restock suggestions (P2.4)

Bearer JWT admin routes under **`/v1/admin/organizations/{organizationId}/…`** (same scoping rules as fleet: platform admins pass `{organizationId}`; org admins use their tenant).

## Permissions

| Operation | RBAC |
|-----------|------|
| List / get anomalies, restock suggestions | `inventory.read` **or** `fleet.read` **or** `telemetry.read` |
| Resolve / ignore anomaly | `inventory.adjust` |

## Anomalies

Rows live in **`inventory_anomalies`** (extended in migration `00072_p24_operational_anomaly_types.sql`). Open issues are **deduplicated** per machine via **`(machine_id, fingerprint)`** while `status = 'open'`.

**`GET …/anomalies`**

- Query: `refresh=true` runs detectors (inventory + operational) before listing; `limit`, `offset` same as other admin lists.
- Response: `{ "items": [ … ] }` — same item shape as **`GET …/inventory/anomalies`** (includes `machineName`, `payload`, timestamps).

**`GET …/anomalies/{anomalyId}`**

- Single row with machine name/serial.

**`POST …/anomalies/{anomalyId}/resolve`**

- Body (optional): `{ "note": "…" }`.
- Sets `status = resolved`; writes enterprise audit action **`admin.operational_anomaly.resolve`**.

**`POST …/anomalies/{anomalyId}/ignore`**

- Same body; sets **`status = ignored`**; audit **`admin.operational_anomaly.ignore`**.

### Detector types (P2.4)

| `anomalyType` | Signal (summary) |
|---------------|------------------|
| `machine_offline_too_long` | `last_seen_at` older than **2 hours** (non-retired machines) |
| `repeated_vend_failure` | ≥ **3** failed `vend_sessions` in **24h** |
| `repeated_payment_failure` | ≥ **3** failed `payments` in **24h** |
| `stock_mismatch` | `machine_slot_state.current_quantity` **>** slot `max_quantity` |
| `negative_stock_attempt` | Recent failed vend with stock/out/empty-like `failure_reason` |
| `high_cash_variance` | `cash_collections` closed with `abs(variance_amount_minor) ≥ 50000` (90-day lookback) |
| `command_failure_spike` | ≥ **5** failed `machine_command_attempts` in **24h** |
| `telemetry_missing` | Active machine; snapshot missing or **`machine_current_snapshot.updated_at` > 6h** |
| `low_stock_threshold` | On hand ≤ **10%** of slot max (max ≥ **5**) |
| `product_sold_out_soon_estimate` | Successful vend velocity implies **≤ ~3 days** of supply |

Legacy inventory detectors (`negative_stock`, `stale_inventory_sync`, …) still run inside the same **`refresh`** sync.

## Restock suggestions

**`GET …/restock/suggestions`**

- Same query parameters and JSON response as **`GET /v1/admin/inventory/refill-suggestions`** (velocity window, filters, pagination).
- Each item includes **`currentQuantity`**, **`dailyVelocity`**, **`suggestedRefillQuantity`**, **`maxQuantity`** (threshold context), and urgency — suitable for operator explanation.

## Related

- Existing inventory-scoped listing: **`…/inventory/anomalies`** (P1.2).
- OpenAPI / Swagger: paths registered next to other org admin operations.
