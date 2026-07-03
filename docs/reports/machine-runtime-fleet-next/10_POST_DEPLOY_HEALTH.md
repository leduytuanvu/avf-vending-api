# Post-Deploy Health — Runtime Fleet

**UTC:** 20260703T230700Z  
**Deployed SHA:** `277a3ad4dbe34f204704ed4c3d713ec49bff4ec2`

## REST

| Endpoint | Status | Notes |
|----------|--------|-------|
| `GET /health/live` | **200** | body: `ok` |
| `GET /health/ready` | **200** | body: `ok` |
| `GET /version` | **200** | `git_sha=277a3ad4dbe34f204704ed4c3d713ec49bff4ec2`, `app_env=production`, `build_time=2026-07-03T22:52:10Z` |

## gRPC

| Check | Result |
|-------|--------|
| TLS to `machine-api.ldtv.dev:443` | reachable |
| gRPC reflection | not enabled (expected in production) |
| `MachineRuntimeSessionService` | defined in `proto/avf/machine/v1/runtime_session.proto`; exercised in Phase 5 suite via proto-backed grpcurl matrix |

## MQTT

| Check | Result |
|-------|--------|
| TCP/TLS port `mqtt.ldtv.dev:8883` | **TcpTestSucceeded=True** |

## Migrations / schema

From deploy log (`app-node-0-72.62.244.94-deploy.log`):

| Check | Result |
|-------|--------|
| Goose version before | 16 |
| Goose version after | **18** |
| `00017_machine_runtime_fleet.sql` | applied 2026-07-03T23:04:12Z |
| `00018_machine_runtime_fleet_fixes.sql` | applied 2026-07-03T23:04:12Z |

Schema objects introduced by migrations (verified at deploy; REST read-back in suite `verify_db_state.py`):

- Tables: `machine_device_attachments`, `machine_runtime_app_sessions`
- Columns: `machines.sale_enabled`, `machines.current_device_attachment_id`, `machines.current_runtime_app_session_id`, check-in runtime columns

## Gate

**PASS** — all Phase 4 health checks green. Proceed to 3× production full suite.
