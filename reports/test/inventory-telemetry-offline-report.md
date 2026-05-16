# Inventory, telemetry, offline replay report

- Generated (UTC): `2026-05-16T10:20:11.079333+00:00`
- `BASE_URL`: `http://127.0.0.1:18080`
- No `organization_id` query parameters were used on HTTP steps; admin fleet scope resolves as single-company server-side (`uuid.Nil`).

## Go tests (focused packages)

Command:

```text
go test ./internal/app/inventoryapp/... ./internal/app/inventoryadmin/... ./internal/app/telemetryapp/... ./internal/modules/postgres/... -run 'Inventory|Telemetry|Offline|Replay|Critical|Adjustment|Refill|Stock' -count=1
```

Output (`reports/test/inventory-telemetry-offline/go-test-packages.txt`):

```text
ok  	github.com/avf/avf-vending-api/internal/app/inventoryapp	0.691s
ok  	github.com/avf/avf-vending-api/internal/app/inventoryadmin	0.847s
ok  	github.com/avf/avf-vending-api/internal/app/telemetryapp	0.215s
ok  	github.com/avf/avf-vending-api/internal/modules/postgres	0.213s
```

Supplementary (MQTT contract, critical idempotency helpers, grpc offline replay, postgres telemetry duplicates):

```text
--- internal/platform/mqtt (OfflineReplayContract) ---
ok  	github.com/avf/avf-vending-api/internal/platform/mqtt	0.143s
--- internal/platform/telemetry (critical/offline classify) ---
ok  	github.com/avf/avf-vending-api/internal/platform/telemetry	0.527s
--- internal/app/telemetryapp (projection duplicate guard) ---
ok  	github.com/avf/avf-vending-api/internal/app/telemetryapp	0.119s

--- Integration tests (optional; require Docker Postgres + TEST_DATABASE_URL) ---
SKIPPED: Docker Desktop engine not available in this run.
To execute locally:
  $env:TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:15432/avf_vending_test?sslmode=disable'
  go test ./internal/grpcserver -run 'TestP06_OfflineSync_duplicateOfflineSequenceReplayed|TestP06_OfflineSync_duplicateClientEventIdAtLaterSequenceRejected' -count=1
  go test ./internal/modules/postgres -run 'TestAppendDeviceTelemetryEdgeEvent_duplicateSafe|TestAppendInventoryEventFromDeviceTelemetry_duplicateSafe' -count=1
```

## HTTP flow (`scripts/test/inventory_telemetry_http_flow.py`)

- Stock adjustment idempotency replay: **False**
- Negative `quantity_after` rejected (HTTP error): **True**

### Resource IDs

- `site_id`: `5ebb338e-3b02-4c55-b963-de45b29bf1f1`
- `product_id`: `68818542-933f-4429-9390-6bd36e4fece0`
- `machine_id`: `5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9`
- machine-local **planogramId** (draft + adjustments): `bfe8575d-0aa7-4d0e-96bd-b1d6538a97c4`

### Organization / tenant scan

- No `organization_id` / `tenant` substrings detected in JSON bodies for HTTP steps.

### HTTP steps

| step | method | path | status | pass |
| --- | --- | --- | --- | --- |
| login | POST | `/v1/auth/login` | 200 | pass |
| admin_inventory_initial | GET | `/v1/admin/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/inventory` | 200 | pass |
| admin_slots_initial | GET | `/v1/admin/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/slots` | 200 | pass |
| admin_inventory_low_stock | GET | `/v1/admin/inventory/low-stock?machine_id=5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9&limit=10` | 200 | pass |
| admin_inventory_events_list | GET | `/v1/admin/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/inventory-events?limit=5` | 200 | pass |
| operator_session_login | POST | `/v1/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/operator-sessions/login` | 200 | pass |
| admin_topology_put | PUT | `/v1/admin/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/topology` | 204 | pass |
| admin_planogram_draft_put | PUT | `/v1/admin/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/planograms/draft` | 500 | fail |
| admin_stock_adjustment | POST | `/v1/admin/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/stock-adjustments` | 404 | pass |
| admin_stock_adjustment_idempotency_replay | POST | `/v1/admin/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/stock-adjustments` | 404 | pass |
| admin_stock_adjustment_negative_quantity_rejected | POST | `/v1/admin/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/stock-adjustments` | 500 | fail |
| admin_inventory_reconcile_marker | POST | `/v1/admin/operations/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/inventory/reconcile` | 404 | pass |
| machine_telemetry_snapshot | GET | `/v1/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/telemetry/snapshot` | 500 | fail |
| machine_telemetry_incidents | GET | `/v1/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/telemetry/incidents?limit=5` | 200 | pass |
| machine_telemetry_rollups | GET | `/v1/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/telemetry/rollups?limit=5` | 200 | pass |
| admin_inventory_after_adjustment | GET | `/v1/admin/machines/5f33a97d-6c34-4a5f-82d5-b7ffa06f8dc9/inventory` | 200 | pass |

## Offline replay & critical telemetry (automated coverage)

- **gRPC offline replay** (`PushOfflineEvents`): duplicate sequence replays as `REPLAYED` — `go test ./internal/grpcserver -run TestP06_OfflineSync_duplicateOfflineSequenceReplayed` (requires `TEST_DATABASE_URL`).
- **Uniqueness / rejection**: conflicting `client_event_id` at different sequence — `TestP06_OfflineSync_duplicateClientEventIdAtLaterSequenceRejected`.
- **MQTT offline replay contract** (critical duplicate idempotency payload): `internal/platform/mqtt/offline_replay_contract_test.go`.
- **Critical telemetry idempotency key derivation**: `internal/platform/telemetry/critical_idempotency_test.go`.
- **OLTP duplicate suppression** (edge telemetry): `internal/modules/postgres/telemetry_idempotency_integration_test.go`.

## Observations

- Provisioned canary site/product/active machine.
- Planogram draft PUT expected 204; got status=500
- Stock adjustment replay: expected replay=true on duplicate Idempotency-Key.

## Final result

| Gate | Result |
| --- | --- |
| Go tests (focused packages) | **PASS** |
| Go tests (supplementary MQTT/telemetry + optional integration notes) | **PASS** |
| HTTP inventory/telemetry flow | **FAIL** |
| **Overall** | **FAIL** |

## Evidence files

- `reports/test/inventory-telemetry-offline/http-*.json`
- `reports/test/inventory-telemetry-offline/go-test-*.txt`
