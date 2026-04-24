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

## Behavior

- **Tenant isolation**: Same as other admin machine routes (`organization_id` query for `platform_admin`).
- **Review threshold**: `CASH_SETTLEMENT_VARIANCE_REVIEW_THRESHOLD_MINOR` (default **500** minor units). When `abs(variance)` exceeds the threshold, `requires_review` is true and `cash_reconciliations.status` is **`review`**.
- **Hardware**: Responses and stored metadata state that this surface is **accounting-only** and does **not** command bill recyclers or other cash hardware.

## Migrations

`migrations/00021_cash_collection_settlement.sql` adds lifecycle columns, variance fields, and `ux_cash_collections_machine_one_open`.
