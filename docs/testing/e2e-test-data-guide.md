# E2E test data guide

Fake values below are **examples**; replace with locally generated UUIDs and **never** commit live passwords, HMAC secrets, TLS keys, or production MQTT passwords.

## Harness files (Phase 1)

| File | Purpose |
|------|---------|
| **`tests/e2e/.env.example`** | Documented variables for `tests/e2e/.env` (copy locally) |
| **`.e2e-runs/run-*/test-data.json`** | Public capture produced per run (IDs, masked tokens) |
| **`.e2e-runs/run-*/secrets.private.json`** | Full **ADMIN_TOKEN**, **MACHINE_TOKEN**, MQTT password, etc. (never copy out of the run dir) |
| **`tests/e2e/data/seed.local.example.json`** | Not loaded automatically — template for org/site/slots |
| **`tests/e2e/data/reusable-test-data.example.json`** | Expected shape for `--reuse-data` |
| **`tests/e2e/data/test-data.schema.json`** | JSON Schema for reusable capture |

Helpers in **`tests/e2e/lib/e2e_data.sh`**:

- **`e2e_data_initialize`** — honors `--fresh-data` / `--reuse-data PATH` / env `E2E_REUSE_DATA` + `E2E_DATA_FILE`
- **`e2e_set_data` / `e2e_get_data` / `e2e_require_data`** — flat string keys on `test-data.json` (**aliases:** `set_data`, `get_data`, `require_data`)
- **`e2e_set_data_json`** — merge JSON-valued keys (**alias:** `set_data_json`)
- **`e2e_save_token`** — writes full value to `secrets.private.json` and a **masked** value to `test-data.json` (**alias:** `save_token`)
- **`e2e_data_initialize`** (**alias:** `initialize_test_data`)

When **`run-all-local.sh`** runs phase scripts with **`E2E_IN_PARENT=1`**, reuse/fresh flags from the orchestrator are **restored after `load_env`** so a child’s `tests/e2e/.env` does not clobber the parent’s `--reuse-data`.

## Hierarchy

| Entity | Description | Typical source |
|--------|-------------|----------------|
| **Organization** | Tenant boundary | Admin REST or seed |
| **Site** | Physical/logical location | Under organization |
| **Admin user** | Operator | `POST` login (paths per swagger) |
| **Machine** | Vending asset | Admin machine APIs |
| **Activation code** | One-time binding | Admin activation endpoints |
| **Machine token** | JWT after claim | gRPC / auth responses |

## Catalog & planogram

| Data | Notes |
|------|-------|
| **Categories / brands** | Admin REST `/v1/admin/categories`, `/v1/admin/brands` |
| **Products** | SKUs, tax flags, media refs |
| **Planogram** | Draft → publish; **`catalog_version`** for gRPC |
| **Slots** | Cabinet/slot codes align with commerce protos |
| **Prices** | Server-authoritative minors |
| **Inventory** | Quantities for vend; align with FT-VND in [`field-test-cases.md`](field-test-cases.md) |

## Payments

| Mode | Guidance |
|------|----------|
| **PSP sandbox** | Non-live keys in **local** `.env` only |
| **Cash** | Machine commerce gRPC paths |
| **Webhooks** | Configure URL + HMAC in local env only |

## MQTT

| Item | Guidance |
|------|----------|
| **Prefix** | Enterprise vs legacy per [`mqtt-contract.md`](../api/mqtt-contract.md) |
| **Client ID** | Machine-scoped test client id (see seed example) |
| **Auth** | `MQTT_USERNAME` / `MQTT_PASSWORD` in `.env` |

## Modes

| Mode | When |
|------|------|
| **Reusable** | Stable org + machine (`--reuse-data`) |
| **Fresh** | After activation collision or 409 idempotency (`--fresh-data`) |

## Idempotency

- REST: **`Idempotency-Key`** / **`X-Idempotency-Key`** on mutating routes (per swagger).
- gRPC: idempotency fields in protos — align with [`local-e2e.md`](local-e2e.md).

## Cleanup policy

| Environment | Policy |
|-------------|--------|
| **Local scratch DB** | Recreate volume or reset schema |
| **Staging** | Admin/support APIs; revoke activation codes |
| **Production** | No automated cleanup |

## Related

- **[`e2e-flow-coverage.md`](e2e-flow-coverage.md)**
- **[`e2e-local-test-guide.md`](e2e-local-test-guide.md)**
