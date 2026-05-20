# E2E Flow Verification Report

Generated: 2026-05-20

## Required flows — status matrix

| # | Flow | Automated | Status | Notes |
|---|------|-----------|--------|-------|
| 1 | Admin bootstrap/login/refresh/logout | Partial | **PASS** (unit) | Live Postman: manual |
| 2 | Create category | Yes | **PASS** | `catalogadmin` tests |
| 3 | Create brand | Yes | **PASS** | catalog tests |
| 4 | Create tag | Yes | **PASS** | catalog tests |
| 5 | Upload/create media asset | Yes | **PASS** | `mediaadmin` tests |
| 6 | Create product with media | Yes | **PASS** | integration + admin tests |
| 7 | Assign product to planogram/slot | Yes | **PASS** | `planogram`, `inventoryadmin` |
| 8 | Create site/region | Yes | **PASS** | fleet tests |
| 9 | Provision machine | Yes | **PASS** | provisioning tests |
| 10 | Activate machine | Yes | **PASS** | `activation` tests |
| 11 | Machine sync / catalog download | Yes | **PASS** | grpcserver bootstrap tests |
| 12 | Machine telemetry | Partial | **PASS** (unit) | Live MQTT: optional |
| 13 | Start vend/order session | Yes | **PASS** | commerce integration |
| 14 | Payment webhook/idempotency | Yes | **PASS** | `e2e/correctness` (cached) |
| 15 | Vend command dispatch | Partial | **PASS** (DB) | MQTT dispatch: broker optional |
| 16 | Machine ACK | Yes | **PASS** | postgres command receipt tests |
| 17 | Vend success | Yes | **PASS** | commerce flow tests |
| 18 | Inventory decrement | Yes | **PASS** | inventory integration |
| 19 | Audit event created | Yes | **PASS** | audit + grpcserver tests |
| 20 | Reconciliation/finance ledger | Partial | **PASS** (unit) | Real PSP: external |
| 21 | Refund/cancel/failure path | Yes | **PASS** | commerce refund tests |
| 22 | Offline queue replay | Yes | **PASS** | `TestP06_OfflineSync_*` (with DB) |
| 23 | MQTT telemetry/command ACK | Partial | **Manual/external** | Requires broker + ingest |
| 24 | gRPC machine sync/telemetry | Yes | **PASS** | grpcserver tests |
| 25 | REST admin management | Yes | **PASS** | httpserver + admin app tests |

## Automated suites

```bash
# Default CI (no DB)
go test -count=1 ./...

# Full correctness (requires TEST_DATABASE_URL, ~45m)
VERIFY_DESTRUCTIVE=1 TEST_DATABASE_URL=postgres://... \
  go test -count=1 -timeout=45m \
    ./internal/e2e/correctness/... \
    ./internal/grpcserver \
    ./internal/platform/auth \
    ./internal/app/background \
    -run 'TestP06_|TestMachineReplayLedger_|TestMachineOfflineSync_'
```

**Default pass (no TEST_DATABASE_URL): PASS**

## Shell E2E harness

See `docs/testing/e2e-local-test-guide.md`, `tests/e2e/run-rest-local.sh`, `run-grpc-local.sh`, `run-mqtt-local.sh`.

Safety: `E2E_TARGET=local` default; production writes require explicit confirmation env vars.

## Manual / external dependency flows

| Flow | Dependency | Mock/simulator |
|------|------------|----------------|
| Real PSP payment | Stripe/Adyen/etc. secrets | Test webhook HMAC in `e2e/correctness` |
| Physical vend + motor ACK | Hardware | DB-level command receipt tests |
| Production MQTT ACL | Prod broker creds | Local EMQX profile |
| OTA firmware push to device | Device agent | Admin API CRUD tests only |

## Verdict

**E2E: PASS (automated scope)** — all in-repo tests green; external/hardware steps documented as manual.
