# E2E test data guide

Fake values below are **examples**; replace with locally generated UUIDs and **never** commit live passwords, HMAC secrets, TLS keys, or production MQTT passwords.

## Hierarchy

| Entity | Description | Typical source |
|--------|-------------|----------------|
| **Organization** | Tenant boundary | Admin REST create or seed SQL |
| **Site** | Physical/logical location | Under organization |
| **Admin user** | Human or service operator | `POST /v1/auth/login` |
| **Machine** | Vending asset | `POST /v1/admin/machines` (path per swagger) |
| **Activation code** | One-time binding | Admin machine or org activation endpoints |
| **Machine token** | JWT after claim | gRPC `MachineAuthService` response |

## Catalog & planogram

| Data | Notes |
|------|-------|
| **Categories / brands** | Admin REST `/v1/admin/categories`, `/v1/admin/brands` |
| **Products** | SKUs, tax flags, media refs |
| **Planogram** | Draft → publish; records **`catalog_version`** consumed by gRPC catalog |
| **Slots** | Cabinet/slot codes align with `SlotSelection` in `commerce.proto` |
| **Prices** | Server-authoritative minors; machine **must not** forge totals |
| **Inventory** | Quantities for vend; align with FT-VND rows in [`field-test-cases.md`](field-test-cases.md) |

## Payments

| Mode | Guidance |
|------|----------|
| **PSP sandbox** | Use staging policy: `payment_env` ≠ `live` when base URL is staging (see Postman prerequest) |
| **Cash** | `ConfirmCashPayment` / `ConfirmCashReceived` paths per **`MachineCommerceService`** |
| **Webhooks** | REST PSP callbacks — configure test harness URL + HMAC secret in **local** env only |

## MQTT

| Item | Guidance |
|------|----------|
| **Prefix** | `MQTT_TOPIC_PREFIX`; enterprise vs legacy per [`mqtt-contract.md`](../api/mqtt-contract.md) |
| **Client ID** | Machine-scoped; include credential version where broker requires it |
| **Auth** | Username/password or mTLS per stack; store in `.env` gitignored |
| **Subscribe** | Machine: **only** its command topic |
| **Publish** | Telemetry, ACK, receipts on allowed tails only |

## Modes

| Mode | When |
|------|------|
| **Reusable** | Stable org + machine for daily dev (`--reuse-data`) |
| **Fresh** | After activation collision, credential rotation tests, or 409 idempotency (`--fresh-data`) |

## Idempotency

- REST: header **`Idempotency-Key`** (alias `X-Idempotency-Key`) on **mutating** commerce/command routes (per swagger).
- gRPC: `IdempotencyContext` in protos — align `client_request_id` / keys with replay tests in [`local-e2e.md`](local-e2e.md).

## Cleanup policy

| Environment | Policy |
|-------------|--------|
| **Local scratch DB** | `DROP` schema or recreate container volume after full runs |
| **Staging** | Delete entities via admin APIs or support playbook; **revoke** activation codes |
| **Production** | **No** automated cleanup — field procedures only |

## Seed & capture files

- **`tests/e2e/data/seed.local.example.json`** — starting template
- **`tests/e2e/data/reusable-test-data.example.json`** — shape after a successful run
- **`tests/e2e/data/test-data.schema.json`** — JSON Schema for capture validation

## Related

- **[`e2e-flow-coverage.md`](e2e-flow-coverage.md)**
