# Stock adjustments (`POST /v1/admin/machines/{machineId}/stock-adjustments`)

Implements append-only **`inventory_events`** plus **`machine_slot_state`** updates in one transaction (`internal/httpserver/admin_inventory_http.go`).

## Headers

- **`Idempotency-Key`** or **`X-Idempotency-Key`** (required).
- **`Authorization`**: Bearer access token.
- **`platform_admin`**: pass **`organization_id`** query to resolve tenant (same as other admin inventory routes).

## Body

```json
{
  "operator_session_id": "dddddddd-eeee-ffff-0000-111111111111",
  "reason": "restock",
  "occurredAt": "2026-04-19T12:00:00.000000000Z",
  "items": [
    {
      "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "slotIndex": 3,
      "quantityBefore": 2,
      "quantityAfter": 10,
      "cabinetCode": "A",
      "slotCode": "A3",
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff"
    }
  ]
}
```

- **`operator_session_id`**: must reference an **ACTIVE** operator session on this machine.
- **`occurredAt`** (optional): business time for the adjustment as **RFC3339Nano** with explicit timezone offset; omitted means “now”. **`recordedAt`** on persisted events is always server time.
- **`reason`**: one of **`restock`**, **`cycle_count`**, **`manual_adjustment`**, **`machine_reconcile`** (see handler validation).
- **`items`**: each row must match current `machine_slot_state` for **`quantityBefore`** or the API returns **409** `quantity_before_mismatch`.
- **`cabinetCode`**, **`slotCode`**, **`productId`**: optional disambiguators when useful; mirror OpenAPI `V1AdminStockAdjustmentsRequest`.

## Success response

```json
{
  "replay": false,
  "eventIds": [1001, 1002]
}
```

`replay: true` when the same idempotency key is replayed (no double-apply).

## Reference

- OpenAPI: **`docs/swagger/swagger.json`** on `POST /v1/admin/machines/{machineId}/stock-adjustments` (generated example matches `tools/build_openapi.py`).
- Go types: `V1AdminStockAdjustmentsRequest` / `V1AdminStockAdjustmentsResponse` in `internal/httpserver/openapi_types.go`.
