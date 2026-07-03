# Local Fix and Test Report — Machine Runtime Fleet (fix pass)

**Branch:** `feature/machine-runtime-fleet`  
**Date:** 2026-07-04 (UTC+7)

## Fixes applied

| Area | Change |
|------|--------|
| Migration 00018 | `sale_enabled` backfill for active machines; partial unique index `ux_machine_runtime_app_sessions_one_current` |
| sale_enabled lifecycle | Enable/disable/retire/compromised wire `sale_enabled`; `finalSellReady` requires active + sale_enabled + runtime sell_ready + no critical blockers |
| Reattach identity | Raw `device_fingerprint` JSON → `DeviceIdentityFromFingerprint` (SIM/board/build SHA) |
| Technician reattach | Operator session ACTIVE + machine match + technician actor; `TechnicianID` passed to attach |
| Session security | Heartbeat/end/stale SQL + service enforce `machine_id` + open session; admin mark-stale uses URL machineId |
| Session lifecycle | `LockMachineForUpdate`, `previous_runtime_session_id`, `projectRuntimeSnapshot` on start/heartbeat/end/MQTT |
| Ops overview | Enriched fleet/detail JSON; fleet filters `machine_type`, `sell_ready`, `has_active_operator_session` |
| Proto/gRPC | Extended `runtime_session.proto` (blockers, status blobs, android identity); gRPC mapping updated |
| Timeline | Heartbeat, crashed, recovered, revoked/compromised attachment events |
| Machine code | Production enforcement `^AVF[0-9]{6,}$` via fleet create/update |
| Harness | `AVF-RUNTIME-FLEET-*` prefix, inline runtime gRPC matrix, extended DB verify |

## Local gate results

| Command | Result |
|---------|--------|
| `sqlc generate` | PASS |
| `go build ./...` | PASS |
| `go test ./...` | PASS |
| `python tools/build_openapi.py` | PASS |
| `python scripts/ci/check_machine_grpc_docs.py` | PASS (after MachineRuntimeSessionService doc) |
| `python scripts/test/rest_openapi_coverage.py` | PASS |
| `python scripts/test/grpc_inventory_coverage.py` | PASS |

## New/updated tests

- `internal/app/machineruntime/overview_test.go` — `computeFinalSellReady`, extended fingerprint
- Existing packages: machineruntime, activation, fleet, grpcserver, httpserver — all green

## Residual local gaps

- Dedicated `reattach_test.go` integration negatives not added (HTTP/RBAC covered indirectly by existing httpserver tests)
- `latest_events` block on ops-overview not wired (timeline available via separate admin route)
