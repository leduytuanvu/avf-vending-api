# Kiosk implementation — HTTP payload & idempotency reference

Companion to [kiosk-app-implementation-checklist.md](../kiosk-app-implementation-checklist.md). **Field-level schemas:** `docs/swagger/swagger.json` (run `make swagger`).

## Idempotency-Key header

- Header: `Idempotency-Key` or `X-Idempotency-Key` (alias).
- Value: opaque string, **stable per logical operation** (max length per gateway; use UUID v4 or deterministic composite).
- On timeout, **retry with the same key**; do not mint a new key for the same user action.

## Commerce (kiosk)

| Route | Method | Idempotency-Key | Notes |
| --- | --- | --- | --- |
| `/v1/commerce/cash-checkout` | POST | **Required** | Prefer one-step cash path when applicable |
| `/v1/commerce/orders` | POST | **Required** | Multi-step wallet flow |
| `/v1/commerce/orders/{orderId}/payment-session` | POST | **Required** | PSP session |
| `/v1/commerce/orders/{orderId}/vend/start` | POST | **Required** | |
| `/v1/commerce/orders/{orderId}/vend/success` | POST | **Required** | |
| `/v1/commerce/orders/{orderId}/vend/failure` | POST | **Required** | |

**Finalize success / inventory coupling:** retries of **`vend/success`** or device **`vend-results`** replay the same **`Idempotency-Key`**; **`FinalizeAfterVend`** builds the inventory duplicate-suppression string as **`{Idempotency-Key}:vend_sale_inventory`**, so **`orders`** + **`vend_sessions`** + **`machine_slot_state`** + ledger rows commit together—or roll back on insufficient stock—with no orphaned “paid but inventory missing” splits. `GET /v1/commerce/orders/{orderId}`, `GET .../reconciliation`.

## Device bridge (optional integration)

| Route | Method | Idempotency-Key |
| --- | --- | --- |
| `/v1/device/machines/{machineId}/vend-results` | POST | **Required** |

**JWT policy:** Confirm deployment allows kiosk vs admin-only (see [api-client-classification.md](../api-client-classification.md)).

## Machine runtime

| Route | Method | Idempotency-Key |
| --- | --- | --- |
| `/v1/machines/{machineId}/check-ins` | POST | Optional per product; use correlation in body if supported |
| `/v1/machines/{machineId}/config-applies` | POST | Optional |

## Operator sessions

| Route | Method | Idempotency-Key |
| --- | --- | --- |
| `/v1/machines/{machineId}/operator-sessions/login` | POST | — |
| `/v1/machines/{machineId}/operator-sessions/logout` | POST | — |
| `/v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat` | POST | — |

## Admin (technician / settlement only)

| Route | Method | Idempotency-Key |
| --- | --- | --- |
| `/v1/admin/machines/{machineId}/stock-adjustments` | POST | **Required** |
| `/v1/admin/machines/{machineId}/planograms/publish` | POST | **Required** |
| `/v1/admin/machines/{machineId}/sync` | POST | **Required** |

## MQTT critical telemetry (mirror)

Not HTTP — publish JSON envelope per [mqtt-contract.md](../mqtt-contract.md):

- Set **`dedupe_key`** from app `idempotency_key`.
- Map replay **`emitted_at`** → wire **`occurred_at`**.
- Provide **`event_id`** or **`boot_id` + `seq_no`** for critical classes.

JSON samples: `testdata/telemetry/valid_vend_success.json`, `valid_payment_success.json`, `valid_cash_inserted.json`, etc.

## Target / confirm routes

These **may** exist on your backend revision; **verify in OpenAPI** before coding defaults:

- `GET /v1/machines/{machineId}/sale-catalog`
- `POST /v1/machines/{machineId}/events/reconcile`
- Cash settlement under `/v1/admin/machines/{machineId}/cashbox` / `cash-collections`
- Refund/cancel under commerce or admin
