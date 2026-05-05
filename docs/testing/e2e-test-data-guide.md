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

## Web Admin automated setup (WA-SETUP-01)

Runner: **`tests/e2e/run-web-admin-flows.sh`**, scenario **`tests/e2e/scenarios/01_web_admin_setup.sh`**.

- **Auth:** Either set **`ADMIN_TOKEN`** (Bearer for admin APIs) and **`E2E_ORGANIZATION_ID`** (or put **`organizationId`** in reuse data), or set **`ADMIN_EMAIL`**, **`ADMIN_PASSWORD`**, and **`E2E_ORGANIZATION_ID`** so the script calls **`POST /v1/auth/login`** with `{ email, password, organizationId }` (matches OpenAPI / Postman).
- **Mutations:** Set **`E2E_ALLOW_WRITES=true`** (default from `load_env` for local). The script exits early with a clear message if writes are disabled.
- **Production:** If **`E2E_TARGET=production`**, writes require **`E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`** (see `e2e_target_safety_guard`).
- **Seed file:** Optional **`E2E_SEED_FILE`** (default **`tests/e2e/data/seed.local.example.json`**) supplies names, slot codes (`A1`), prices, and quantities. It does **not** create an organization; the org must already exist in your DB.
- **Artifacts:** Each REST call writes **`rest/*.request.json`**, **`rest/*.response.json`**, **`rest/*.meta.json`**. Structured flow log: **`test-events.jsonl`** (`flow_id`, `step_name`, `protocol=REST`, `endpoint`, `resource_ids`, `status`, `message`). Public IDs: **`test-data.json`**; plaintext activation code (if returned): **`secrets.private.json`**.
- **`summary.md`:** After a standalone run (`run-web-admin-flows.sh` not under `run-all-local`), the report includes a **Web admin setup steps (WA-SETUP-01)** checklist plus the **Web admin test events** table from **`test-events.jsonl`**.

### Create fresh data (`--fresh-data`)

1. Ensure the API is up (`BASE_URL`, e.g. `http://127.0.0.1:8080`).
2. Copy **`tests/e2e/.env.example`** → **`tests/e2e/.env`** and set login/tenant vars (see above).
3. Run: `E2E_ALLOW_WRITES=true ./tests/e2e/run-web-admin-flows.sh --fresh-data`. This clears **`test-data.json`** for that run and creates timestamped site, machine, catalog rows, etc.
4. If login fails **`401`**/**`403`**, seed an admin user and org in local DB (migrations / ops scripts), then retry.

### Reuse data (`--reuse-data path/to/test-data.json`)

1. Capture a previous run’s **`test-data.json`** (and ensure **`secrets.private.json`** if you rely on stored tokens — usually you still log in or pass **`ADMIN_TOKEN`**).
2. Run: `./tests/e2e/run-web-admin-flows.sh --reuse-data path/to/test-data.json`. Existing **`organizationId`**, **`siteId`**, **`machineId`**, **`productId`**, **`categoryId`**, etc. are reused when present; missing steps are created.
3. Useful for iterative UI or gRPC tests against a stable machine.

### Manual cleanup

| What | How |
|------|-----|
| Local scratch DB | Reset Postgres volume / run dev reset per runbook. |
| Orphan E2E entities | Admin UI or support APIs: retire machine, revoke activation codes, archive SKUs/categories as policy allows. |
| Run artifacts | Delete **`.e2e-runs/run-*`** if disk noise matters (never commit secrets). |

### Common errors (Web Admin setup)

| Symptom | Likely cause | What to do |
|---------|----------------|------------|
| **`E2E_ALLOW_WRITES must be true`** | Writes gated | `export E2E_ALLOW_WRITES=true` or set in **`.env`**. |
| **`ADMIN_TOKEN set but organizationId unknown`** | Token only | Set **`E2E_ORGANIZATION_ID`** or use **`--reuse-data`** with **`organizationId`**. |
| **Login HTTP 401/404** | User/org mismatch | Confirm org and user exist locally; check **`rest/wa-login.response.json`**. |
| **Site/machine create 403/422** | Role or validation | Inspect **`rest/wa-site-create.response.json`** / **`wa-machine-create`**. |
| **“No planogram in org” (skip)** | Empty planogram list | Seed at least one org planogram template (admin UI or DB); script exits **0** after catalog steps. |
| **operator-sessions/login skip** | No kiosk permission / assignment | Machine may need admin takeover policy; check **`rest/wa-operator-login.response.json`**. |
| **Draft/publish/stock skip** | Session, revision, or slot mismatch | Compare **`planogramRevision`** and slot codes with seed (**`A`/`A1`**). |

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
