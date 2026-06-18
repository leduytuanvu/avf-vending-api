# API payload examples

JSON files here are **documentation examples** for operators and integrators (idempotency keys, MQTT/telemetry shapes, kiosk payloads).

## Relationship to `testdata/telemetry/`

Several examples intentionally mirror test fixtures:

| Doc example | Test fixture |
|-------------|--------------|
| `cash-inserted-idempotency.json` | `testdata/telemetry/valid_cash_inserted.json` |
| `inventory-delta-idempotency.json` | `testdata/telemetry/valid_inventory_delta.json` |
| `payment-success-idempotency.json` | `testdata/telemetry/valid_payment_success.json` |
| `vend-success-idempotency.json` | `testdata/telemetry/valid_vend_success.json` |

**Canonical for CI/tests:** `testdata/telemetry/`. **Canonical for human docs:** this directory. Do not delete either copy without updating the other and any narrative docs that reference them.
