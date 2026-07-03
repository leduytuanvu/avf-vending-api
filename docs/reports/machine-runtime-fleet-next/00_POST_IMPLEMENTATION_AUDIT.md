# Post-Implementation Audit — Machine Runtime Fleet

**UTC:** 20260704T061500Z  
**Branch:** `feature/machine-runtime-fleet` @ `8c7b806c`  
**Scope:** Re-audit before merge/deploy fixes

---

## Answers (28 questions)

| # | Question | Finding |
|---|----------|---------|
| 1 | Files changed for migration 00017? | `migrations/00017_machine_runtime_fleet.sql`, `db/schema/01_platform.sql`, SQLC queries, `internal/app/machineruntime/*`, REST/gRPC/bootstrap wiring |
| 2 | 00017 applies cleanly? | Yes locally via sqlc; prod not yet applied |
| 3 | sale_enabled default false? | Yes (`00017` ADD COLUMN DEFAULT false) |
| 4 | activate/enable/resume sets sale_enabled=true? | **NO** — only `status=active` |
| 5 | deactivate/disable sets sale_enabled=false? | **NO** — only `status=suspended` |
| 6 | finalSellReady impossible? | **YES** — requires `sale_enabled` which stays false |
| 7 | device_attachments has all Android/SIM fields? | Schema yes; reattach only persists subset |
| 8 | reattach preserves full fingerprint? | **NO** — `DeviceFingerprint` has 7 fields only |
| 9 | technician reattach validates operator session? | **NO** — presence only in HTTP |
| 10 | technician assignment validated? | **NO** — `TechnicianID` never passed |
| 11 | reattach passes technician_id to assignment checker? | **NO** |
| 12 | reattach reason board_replacement vs admin_reattach? | Uses `admin_reattach`/`technician_reattach` only |
| 13 | board replacement closes runtime session? | Yes via `CloseCurrentRuntimeAppSessionForMachine` |
| 14 | Heartbeat validates session belongs to machine? | **NO** — SQL `WHERE id=$1` only |
| 15 | End validates session belongs to machine? | **NO** |
| 16 | admin force-end/mark-stale validate URL machineId? | force-end: **NO**; mark-stale: **NO** (machineId ignored) |
| 17 | DB unique guard one current session? | **NO** — missing partial unique index |
| 18 | Start idempotent boot_id+app_start_id? | Yes |
| 19 | previous_runtime_session_id on crash recovery? | **NO** — always NULL |
| 20 | machine_current_snapshot updated? | **NO** — `UpdateMachineCurrentSnapshotRuntime` never called |
| 21 | ops-overview has all required fields? | **PARTIAL** — SQL loads more than HTTP emits |
| 22 | fleet ops-overview filters? | site_id, machine_code, status, online_status only |
| 23 | timeline includes new events? | Partial — no heartbeat/sell_readiness_changed |
| 24 | gRPC proto blockers/hardware/catalog/outbox? | **NO** |
| 25 | MQTT updates last_mqtt_seen_at? | Yes via telemetry heartbeat + TouchMQTTSeen |
| 26 | Tests cover DB/HTTP/gRPC/RBAC/MQTT/harness? | JWT contract only; no integration for new surfaces |
| 27 | Missing tests? | sale_enabled lifecycle, reattach RBAC, session ownership, snapshot, harness |
| 28 | Must fix before deploy? | All critical gaps in Phase 1 of fix plan |

**Gate:** Proceed to Phase 1 fixes (migration 00018 + service/security/overview/harness).
