# Cash settlement (field collections)

Admin HTTP APIs record **expected** vault cash from **commerce** (`payments` with `provider=cash`, `state=captured`, minus completed `refunds` on the same machine) since the **last closed** `cash_collections` row. Operators run an **open → close** session with a physical **count**; the API stores **variance** and optional **evidence** URLs.

## Endpoints

| Method | Path | Notes |
|--------|------|--------|
| GET | `/v1/admin/machines/{machineId}/cashbox` | Query `currency` (default USD). Returns `expected_amount_minor`, `last_collection_closed_at`, `open_collection_id`, `variance_review_threshold_minor`. |
| POST | `/v1/admin/machines/{machineId}/cash-collections` | Starts an **open** collection. Requires **active** `operator_session_id` and **`Idempotency-Key`**. |
| GET | `/v1/admin/machines/{machineId}/cash-collections` | Paginated list (`limit` / `offset`). |
| GET | `/v1/admin/machines/{machineId}/cash-collections/{collectionId}` | Single row. |
| POST | `/v1/admin/machines/{machineId}/cash-collections/{collectionId}/close` | Closes with `counted_amount_minor`, optional `evidence_artifact_url`. **Idempotent** when the canonical payload hash matches; conflicting numbers → **409**. |
| GET | `/v1/admin/organizations/{organizationId}/reports/cash` | Organization cash collection report. Requires `from`/`to`; optional `site_id`, `machine_id`, `limit`, `offset`, and `format=csv`. CSV exports are audit logged. |

Related finance/reporting endpoints:

- `/v1/admin/organizations/{organizationId}/reports/sales`
- `/v1/admin/organizations/{organizationId}/reports/payments`
- `/v1/admin/organizations/{organizationId}/reports/refunds`
- `/v1/admin/organizations/{organizationId}/reports/inventory-low-stock`
- `/v1/admin/organizations/{organizationId}/reports/machine-health`
- `/v1/admin/organizations/{organizationId}/reports/failed-vends`
- `/v1/admin/organizations/{organizationId}/reports/reconciliation-queue`

## Behavior

- **Tenant isolation**: Same as other admin machine routes (`organization_id` query for `platform_admin`).
- **Report isolation**: Organization-scoped report paths require the caller to be `platform_admin` or belong to the path organization. `finance` and `finance_admin` include `reports.read`; `technician` does not.
- **Date/time**: Reports validate `from < to` and accept IANA `timezone` for business-day buckets where applicable.
- **Review threshold**: `CASH_SETTLEMENT_VARIANCE_REVIEW_THRESHOLD_MINOR` (default **500** minor units). When `abs(variance)` exceeds the threshold, `requires_review` is true and `cash_reconciliations.status` is **`review`**.
- **Hardware**: Responses and stored metadata state that this surface is **accounting-only** and does **not** command bill recyclers or other cash hardware.

## Migrations

`migrations/00021_cash_collection_settlement.sql` adds lifecycle columns, variance fields, and `ux_cash_collections_machine_one_open`.
