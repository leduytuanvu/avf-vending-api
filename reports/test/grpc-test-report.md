# gRPC tests & Phase 6 machine contract report

- Evidence directory: `reports/test/grpc-evidence/`

## 1. grpcurl

Evidence: `reports/test/grpc-evidence/grpc-preflight.txt`

- **Installed** (Git Bash): `/c/Users/admin/go/bin/grpcurl`
- PowerShell `where.exe grpcurl` → `C:\Users\admin\go\bin\grpcurl.exe`

Repeatable check: `bash scripts/test/grpc_preflight.sh`

## 2. Local API + gRPC listener

Evidence: `grpc-preflight.txt` (TCP probe)

- **`127.0.0.1:9090`**: **CLOSED** during this run — `cmd/api` was not listening (Docker Desktop was also unavailable here, so Postgres-backed API startup was not automated).

To run locally (see `scripts/local/start-api-local.ps1`): set `GRPC_ENABLED=true`, `MACHINE_GRPC_ENABLED=true`, `GRPC_ADDR=:9090`, plus working `DATABASE_URL`, Redis/NATS/MQTT as required by your slice.

## 3. Go tests: `go test ./internal/grpcserver/... -count=1`

Two outcomes were captured:

### A) With `TEST_DATABASE_URL` pointing at `127.0.0.1:15432/avf_vending_test` (Postgres **not** reachable)

File: `reports/test/grpc-evidence/go-test-grpcserver.txt`

- **Result: FAIL** — integration tests call `goose` against `TEST_DATABASE_URL`; connection refused to `:15432`.
- **Failing tests (24)** — all integration-style tests that use `machineGRPCTestPool`:

  - `TestMachineGRPC_Commerce_CashSale_EndToEnd`
  - `TestMachineGRPC_Commerce_QRFlow_WebhookThenVend`
  - `TestMachineGRPC_Commerce_StartVend_BlockedBeforePayment`
  - `TestMachineGRPC_Commerce_CreateOrder_IdempotentReplay`
  - `TestMachineGRPC_Commerce_CreatePaymentSession_IdempotentReplay_NoDuplicatePaymentRow`
  - `TestMachineGRPC_Commerce_GetOrder_WrongMachineDenied`
  - `TestMachineGRPC_Commerce_ExpiredCheckoutWindow_Blocked`
  - `TestP06_MachineGRPC_CreateOrder_MissingIdempotencyKeyRejected`
  - `TestP06_MachineGRPC_FillReport_AppliesExpectedStock`
  - `TestP06_MachineGRPC_StockAdjustment_IsAudited`
  - `TestP06_MachineGRPC_StockAdjustment_IdempotentLedgerReplayNoDoubleDelta`
  - `TestP06_OfflineSync_devMachineCashAndVendReplayDoesNotDoubleDecrement`
  - `TestP06_OfflineSync_duplicateInventoryAdjustmentDoesNotDoubleApply`
  - `TestP06_OfflineSync_sortedDescendingMetaStillProcessesAscendingSequences`
  - `TestP06_OfflineSync_gapInSequenceRejectedAfterSuccessfulCursorBump`
  - `TestMachineGRPC_GetBootstrap_RetiredMachineRejected`
  - `TestP06_OfflineSync_duplicateClientEventIdAtLaterSequenceRejected`
  - `TestP06_OfflineSync_outOfOrderErrorIncludesExpectedSequence`
  - `TestMachineOfflineSync_OutOfOrderRejected`
  - `TestP23_PushOfflineEvents_rejectsOverConfiguredBatchCap`
  - `TestP06_OfflineSync_duplicateOfflineSequenceReplayed`
  - `TestMachineReplayLedger_ConcurrentReplayAfterSuccess`
  - `TestMachineGRPC_GetInventorySnapshot_MaintenanceMachineRejected`
  - `TestMachineReplayLedger_ReplayAndConflict`
  - `TestP23_SubmitTelemetryBatch_rejectsOverConfiguredEventCap`

**Root cause:** no Postgres on `TEST_DATABASE_URL` (Docker engine / DB not up).

### B) With `TEST_DATABASE_URL` **unset** (integration skipped)

File: `reports/test/grpc-evidence/go-test-grpcserver-no-db.txt`

- **Result: PASS** (`ok github.com/.../internal/grpcserver …`) — tests that require DB skip when `TEST_DATABASE_URL` is empty (`machine_grpc_integration_test.go`).

**Recommendation:** For a green **full** `./internal/grpcserver/...` run, start Docker deps + migrate `avf_vending_test`, export `TEST_DATABASE_URL`, then rerun the same `go test` command.

## 4. Local gRPC E2E: `bash tests/e2e/run-grpc-local.sh`

Evidence: `reports/test/grpc-evidence/run-grpc-local.console.txt`

- **Result: FAIL** — `gRPC server not reachable at 127.0.0.1:9090 (TCP closed or reflection list failed)`.
- Run directory: `.e2e-runs/run-20260516T092808Z-1950-31202/` (no `grpc/*.log` artifacts because the suite aborted before scenarios).

With API up + reflection enabled (`GRPC_USE_REFLECTION=true`), scenarios **20–24** cover auth/bootstrap, commerce cash sale, inventory/offline, and command update/status.

## 5. Organization removal verification

| Check | Result |
|-------|--------|
| **`E2E_ORGANIZATION_ID` required** | **No** — string does not appear anywhere in the repository (`grep E2E_ORGANIZATION`). |
| **gRPC metadata** (`tests/e2e/lib/e2e_grpc.sh`) | Only **`authorization: Bearer …`**, optional **`x-machine-id`**, optional **`idempotency-key`**. No company/org metadata headers. |
| **Machine access JWT claims** (`internal/platform/auth/machine_jwt.go` → `MachineAccessClaims`) | **`MachineID`, `SiteID`, `SessionID`, `CredentialVersion`, scopes, `Subject`, audience, `token_use`** — **no organization / tenant claim.** |
| **Protos** (`proto/avf/machine/v1`) | **No** `organization` / `tenant` matches in `.proto` files under machine v1. |
| **Generated Go** (`internal/gen`, machine-related) | **No** `organization` / `OrganizationId` / `company_id` matches in scanned machine paths. |

Scenario RPCs use JSON bodies with **`machineId`** / **`meta.machineId`** only (e.g. `21_grpc_bootstrap_catalog_media.sh`), consistent with single-company runtime.

## 6. grpcurl JSON bodies on Windows / Git Bash

Implemented pattern (documented at top of `tests/e2e/lib/e2e_grpc.sh`):

- Write payload to `grpc/<stem>.request.json`.
- Invoke **`grpcurl … -d @ … < "${req}"`** so the body is stdin-fed (avoids broken `-d@file` combining on Windows builds).

Each call still records **stdout** → `*.response.json`, **stderr** (+ banners) → `*.log`, **meta** → `*.meta.json`.

## 7. Proto / codegen changes

**None.** No organization fields found in machine gRPC protos or generated stubs; no regeneration required.

## Files changed (this agent)

| File | Change |
|------|--------|
| `tests/e2e/lib/e2e_grpc.sh` | Expanded file header: Windows/Git Bash stdin body pattern for grpcurl. |
| `scripts/test/grpc_preflight.sh` | **New** — grpcurl path/version + TCP probe for `GRPC_ADDR`. |

## Evidence index

| File | Description |
|------|-------------|
| `grpc-preflight.txt` | grpcurl + port 9090 |
| `go-test-grpcserver.txt` | Full package test with DB URL set but DB down (**FAIL**) |
| `go-test-grpcserver-no-db.txt` | Same package with `TEST_DATABASE_URL` unset (**PASS**, integrations skipped) |
| `run-grpc-local.console.txt` | Phase 6 runner stderr/stdout |

## Final pass / fail

| Gate | Result |
|------|--------|
| grpcurl installed | **PASS** |
| Local API + gRPC :9090 | **NOT VERIFIED** (port closed) |
| `go test ./internal/grpcserver/...` with live `TEST_DATABASE_URL` + migrations | **FAIL** (Postgres unreachable in this environment) |
| `go test ./internal/grpcserver/...` without `TEST_DATABASE_URL` | **PASS** (integration skipped) |
| `run-grpc-local.sh` | **FAIL** (server unreachable) |
| Org/tenant not required in harness, JWT, or protos | **PASS** (static review) |
| **Overall** | **FAIL** — live gRPC server + DB-backed integration/E2E not executed here |

To reach **PASS overall**: start Docker (or your Postgres), apply migrations to `avf_vending_test`, export `TEST_DATABASE_URL`, start `cmd/api` with machine gRPC on `9090`, then rerun `go test ./internal/grpcserver/... -count=1` and `./tests/e2e/run-grpc-local.sh` with `GRPC_USE_REFLECTION=true`, `MACHINE_TOKEN` / activation flow per `docs/testing/e2e-local-test-guide.md`.
