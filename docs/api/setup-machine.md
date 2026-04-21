# Machine setup (bootstrap, topology, planogram)

All paths are under **`/v1`**, Bearer JWT, roles **`platform_admin`** or **`org_admin`** (plus machine access where noted). `platform_admin` must pass **`organization_id`** query on several admin routes—match your deployment’s tenancy rules.

## GET `/v1/setup/machines/{machineId}/bootstrap`

- **Auth**: `RequireMachineURLAccess("machineId")` (machine on JWT).
- **Response**: `machine`, `topology.cabinets[]` with nested `slots[]`, `catalog.products[]` (assortment lines for assignment).
- **Copy-paste**: see OpenAPI **example** on this path in `docs/swagger/swagger.json` (`machineId` in path only; no body).

## PUT `/v1/admin/machines/{machineId}/topology`

- **Body**: `operator_session_id` (UUID, **ACTIVE** session on this machine), `cabinets[]` (`code`, `title`, `sortOrder`, optional `metadata` object), `layouts[]` (`cabinetCode`, `layoutKey`, `revision`, `layoutSpec` object, `status`).
- **Response**: **204 No Content** on success.
- **Example**: OpenAPI request body example on this path.

## PUT `/v1/admin/machines/{machineId}/planograms/draft`

- **Body**: `operator_session_id`, `planogramId` (UUID string), `planogramRevision`, `syncLegacyReadModel` (bool), `items[]` per slot (`cabinetCode`, `layoutKey`, `layoutRevision`, `slotCode`, optional `legacySlotIndex`, optional `productId`, `maxQuantity`, `priceMinor`, optional `metadata`).
- **Response**: **204** on success.
- **Example**: OpenAPI request body example (same shape used for publish).

## POST `/v1/admin/machines/{machineId}/planograms/publish`

- **Headers**: **`Idempotency-Key`** (or `X-Idempotency-Key`) required.
- **Body**: same slot assignment shape as draft (`operator_session_id`, `planogramId`, `planogramRevision`, `syncLegacyReadModel`, `items[]`).
- **Response**: `desiredConfigVersion`, `planogramId`, `planogramRevision`, `command` (`commandId`, `sequence`, `dispatchState`, `replay`) after enqueueing **`machine_planogram_publish`** to MQTT command path (or **503** if MQTT publisher not configured).
- **Example**: OpenAPI request + **200** response example on this path.

## Commerce checkout (related)

- **QR / PSP**: `POST /v1/commerce/orders` → `payment-session` → provider webhook → vend commands → device path.
- **Cash**: `POST /v1/commerce/cash-checkout` (same totals body as create order; marks paid with provider **`cash`**).
- **Device vend result HTTP**: `POST /v1/device/machines/{machineId}/vend-results` (see `docs/api/mqtt-contract.md`). Headers: **`Idempotency-Key`**. Success body example is in OpenAPI; failure outcome (no inventory decrement):

```json
{
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "slot_index": 3,
  "outcome": "failed",
  "failure_reason": "motor_timeout",
  "correlation_id": "11111111-2222-3333-4444-555555555555"
}
```

Use Swagger UI (`/swagger/index.html` when enabled) or `docs/swagger/swagger.json` for exact field names and enums.
