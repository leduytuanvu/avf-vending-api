# Production E2E canary runbook

Safe production integration testing for the Android vending backend without accidentally vending on customer machines.

## Scripts

| Script | Mutations | When to run |
|--------|-----------|-------------|
| [`scripts/e2e/production-readonly-smoke.sh`](../../scripts/e2e/production-readonly-smoke.sh) | **None** (except optional test-machine telemetry heartbeat) | Any time — CI, post-deploy, on-call |
| [`scripts/e2e/production-canary-live-sale.sh`](../../scripts/e2e/production-canary-live-sale.sh) | Order, payment, vend on **canary machine only** | Scheduled QA with operator present |

Legacy wrappers remain under `scripts/test/`; prefer `scripts/e2e/` for new runs.

# Readiness verdict mapping (see docs/release/BACKEND_MARKET_READY_REPORT.md)
# - readonly smoke PASS  → GO (HTTP/gRPC read path healthy)
# - readonly smoke FAIL  → NO-GO
# - canary live sale PASS on marked machine → supports GO-CANARY-ONLY when combined with full smoke + deploy evidence

## Read-only smoke

### What it checks

- `GET /health/live`, `/health/ready`, `/version` (includes `payment_runtime` when present)
- Admin auth + read-only admin routes (when credentials provided)
- gRPC activation **dry-run** (invalid activation code must be rejected)
- gRPC token refresh (when `MACHINE_REFRESH_TOKEN` set)
- gRPC bootstrap, catalog, media manifest/delta, inventory snapshot, planogram
- Bootstrap MQTT config validation (`broker_url`, `topic_prefix`, `topic_layout`, `tls_required`, `client_id_policy`)
- Explicit safety probes: no order, payment, vend, or MQTT command publish
- Optional test-machine telemetry heartbeat (`production_e2e_smoke_heartbeat`)

### What it never does

- Create orders
- Payment sessions or cash confirm
- Start vend / report vend outcome
- MQTT command publish

### Environment

```bash
export BASE_URL=https://api.ldtv.dev
export GRPC_ADDR=machine-api.ldtv.dev:443
export GRPC_USE_PLAINTEXT=false
export GRPC_PROTO_ROOT=/path/to/avf-vending-api/proto

# Test machine (recommended for gRPC section)
export TEST_MACHINE_ID=<canary-machine-uuid>
export MACHINE_ACCESS_TOKEN=<machine JWT>
export MACHINE_REFRESH_TOKEN=<optional refresh token>

# Admin (optional — enables admin read probes)
export ADMIN_EMAIL=...
export ADMIN_PASSWORD=...
# or ADMIN_TOKEN=...

bash scripts/e2e/production-readonly-smoke.sh
```

### Output

- Verdict: **PASS** / **FAIL** (executed probes only; SKIP is not a failure)
- Readiness: **GO** / **GO-CANARY-ONLY** / **NO-GO** (written to `READINESS.txt`)
  - **GO** — all executed probes pass **and** gRPC machine runtime section ran
  - **GO-CANARY-ONLY** — HTTP health pass but gRPC/admin credentials missing (partial smoke)
  - **NO-GO** — any executed probe failed
- Evidence: `reports/e2e/production-readonly-smoke/<timestamp>/`
  - `REPORT.md`, `report.json`, `READINESS.txt`, `raw/*.json`

## Canary live sale

### Required confirmation

```bash
export PRODUCTION_LIVE_TEST_CONFIRMATION=I_UNDERSTAND_THIS_CAN_CHARGE_AND_VEND
```

Without this exact string, the script exits **BLOCKED** (exit 2).

### Required environment

```bash
export BASE_URL=https://api.ldtv.dev
export GRPC_ADDR=machine-api.ldtv.dev:443
export MACHINE_ACCESS_TOKEN=<canary machine JWT>

export TEST_MACHINE_ID=<uuid>
export TEST_SITE_ID=<uuid>
export TEST_SLOT_CODE=A1
export TEST_PRODUCT_ID=<uuid>
export TEST_PRICE_MINOR=12000
export TEST_PAYMENT_METHOD=cash   # cash | qr | card
export TEST_OPERATOR_NAME="QA Operator Name"
export PRODUCTION_CANARY_ROLLBACK_PLAN="Refund order if unintended; restock slot A1; notify on-call."

export ADMIN_EMAIL=...            # required — verifies canary machine
export ADMIN_PASSWORD=...
```

### Protections

| Guard | Behavior |
|-------|----------|
| Confirmation string | Must match exactly |
| Operator name | Non-empty required |
| Rollback plan | `PRODUCTION_CANARY_ROLLBACK_PLAN` non-empty (logged in `CLEANUP.txt`) |
| Price cap | `TEST_PRICE_MINOR` ≤ `PRODUCTION_E2E_MAX_PRICE_MINOR` (default **50000**) |
| Canary machine | Name/code/serial contains `canary` or `e2e-test`, **or** UUID in `PRODUCTION_CANARY_MACHINE_ALLOWLIST` |
| Site match | Admin machine `siteId` must equal `TEST_SITE_ID` |
| Payment config | Bootstrap `cash_enabled` / `qr_card_enabled` must match `TEST_PAYMENT_METHOD` |
| Inventory + planogram | Slot must exist; planogram product must match `TEST_PRODUCT_ID` |
| Catalog price | `TEST_PRICE_MINOR` must match catalog snapshot for `TEST_PRODUCT_ID` |
| Non-canary | **Refused** |

### Flow

1. Verify canary machine via admin API
2. Verify catalog slot/product/price
3. `CreateOrder`
4. `ConfirmCashPayment` **or** `CreatePaymentSession` (qr/card — fails if cash-only production)
5. `StartVend`
6. Wait for hardware **or** `SIMULATE_HARDWARE_VEND=true` for backend-only success
7. `ReportVendFailure` when hardware reports failure
8. `GetOrder` reconcile + inventory delta check
9. `PushTelemetryBatch` canary event
10. Log all order IDs to `test-order-ids.log`
11. Print rollback plan + cleanup instructions

### Backend-only vend (no physical hardware)

```bash
export SIMULATE_HARDWARE_VEND=true
```

Reports `ConfirmVendSuccess` after `StartVend` without waiting for device dispense.

### Marking a canary machine

Pick **one**:

1. Name, code, or serial containing `canary` (recommended: `CANARY-01`)
2. Set `PRODUCTION_CANARY_MACHINE_ALLOWLIST=<machine-uuid>` for explicit allowlist

**Never** run the live sale script against production customer machines.

### Cleanup

After each run, review `CLEANUP.txt` and `test-order-ids.log` in the run directory:

- Cancel/refund unintended orders in admin UI
- Verify inventory on the canary slot
- Archive `reports/e2e/production-canary-live-sale/<timestamp>/` for audit

## Payment mode notes

When production runs `PAYMENT_ENV=cash_only`, `CreatePaymentSession` fails with `provider_unavailable`. Use `TEST_PAYMENT_METHOD=cash` for canary sales unless a wired live PSP is deployed.

See [`docs/payments/PRODUCTION_PAYMENT_PROVIDER_STATUS.md`](../payments/PRODUCTION_PAYMENT_PROVIDER_STATUS.md).

## Related docs

- [`production-canary-test-guide.md`](production-canary-test-guide.md)
- [`integrated-rest-grpc-mqtt-production-verification.md`](integrated-rest-grpc-mqtt-production-verification.md)
- [`docs/api/machine-grpc-production-contract.md`](../api/machine-grpc-production-contract.md)
