# Machine Runtime Fleet — Current State Audit

**UTC:** 20260704T045100Z  
**Repo:** avf-vending-api  
**Baseline SHA (origin/main):** d903756d  

---

## 1. Existing machine fields

`machines` ([`db/schema/01_platform.sql`](../../../db/schema/01_platform.sql) L61–83): `id`, `site_id`, `hardware_profile_id`, `serial_number`, `code`, `model`, `cabinet_type`, `credential_version`, `last_seen_at`, `timezone_override`, `name`, `status`, `command_sequence`, credential timestamps, `published_planogram_version_id`.

**Missing vs target:** `sale_enabled`, `machine_type`, `online_status`, `current_device_attachment_id`, `current_runtime_app_session_id`.

## 2. Machine code behavior

- `code text NOT NULL DEFAULT ''`; unique index on normalized code in fleet queries.
- No enforced `^AVF[0-9]{6,}$` pattern in DB; bootstrap uses `ENTERPRISE_PROD_TEST_*` prefixes in harness.

## 3. Lifecycle / admin active-deactive

- Canonical: `POST /disable|/suspend` → `suspended`; `POST /enable|/resume` → `active`; `POST /retire|/archive` → `decommissioned`.
- No `/activate` or `/deactivate` aliases. Activation via public claim; admin lifecycle requires `RequireFleetMachineLifecycle` (admin/fleet_manager/platform_admin).

## 4. Technician assignment

- Table `technician_machine_assignments`; routes `POST /machines/{id}/technicians`, `/technician-assignments`.
- Operator service checks `TechnicianActiveAssignmentExists` before technician scope.

## 5. Operator session

- `machine_operator_sessions` (ACTIVE/ENDED/EXPIRED/REVOKED); one ACTIVE per machine partial unique index.
- Routes: machine-side operator gRPC + admin reads in ops-overview.

## 6. Reattach-device

- `POST /v1/admin/machines/{machineId}/reattach-device` — [`internal/app/activation/reattach.go`](../../../internal/app/activation/reattach.go).
- Returns one-time access/refresh JWT + MQTT username/password; rotates EMQX user.
- Does **not** create `machine_device_attachments` or end app runtime sessions.

## 7. machine_sessions behavior

- Credential/token sessions (refresh hash, JWT JTI, credential_version).
- One active session per machine (partial unique index).

## 8. runtime-sessions routes = credential session

- [`db/queries/machine_runtime_sessions.sql`](../../../db/queries/machine_runtime_sessions.sql) queries `machine_sessions`.
- Admin GET current/history = credential session view (documented in handler comments).

## 9. machine_check_ins fields

- android_id, sim_serial, package_name, version_name, version_code, android_release, sdk_int, manufacturer, model, timezone, network_state, boot_id, occurred_at, metadata.
- **Missing:** sim_iccid, app_build_sha, runtime_app_session_id, device_attachment_id.

## 10. machine_current_snapshot fields

- Heartbeat, identity, android_id, sim_serial, sim_iccid, device_model, os_version, device_config ack fields.
- **Missing:** runtime app session linkage, online_status, storefront_state, sell_ready, blockers.

## 11. ops-overview response shape

- [`internal/app/adminops/enterprise_ops.go`](../../../internal/app/adminops/enterprise_ops.go): machineId, status, credentialVersion, health, runtimeSession (credential), lastActivationClaim, activeOperatorSession.
- **Missing:** machineCode, androidBoard, sim block, runtimeAppSession, readiness/finalSellReady, latestEvents.

## 12. timeline/unified shape

- [`db/queries/machine_ops_timeline.sql`](../../../db/queries/machine_ops_timeline.sql): audit_events, activation claims, action attributions, machine_sessions issued.
- **Missing:** device attachment events, runtime app session events, operator session lifecycle, sell_readiness_changed.

## 13. OpenAPI routes

- Enterprise routes in [`swagger_operations.go`](../../../internal/httpserver/swagger_operations.go); generated to `docs/swagger/swagger.json` via `tools/build_openapi.py`.
- No app-sessions or device-attachments routes yet.

## 14. gRPC services

- 14 services under `proto/avf/machine/v1/`; registered in [`machine_grpc_services.go`](../../../internal/grpcserver/machine_grpc_services.go).
- **No** `MachineRuntimeSessionService` yet. CheckIn via telemetry/bootstrap writes check_ins + snapshot.

## 15. MQTT / ingest

- Enterprise layout `{prefix}/machines/{machineId}/...` in [`internal/platform/mqtt/topics.go`](../../../internal/platform/mqtt/topics.go).
- Ingest → JetStream → worker → snapshot/command receipts. No `last_mqtt_seen_at` on runtime app sessions.
- EMQX per-machine creds at activation; ACL in `deployments/prod/emqx/acl.conf`.

## 16. Gaps vs target model

| Gap | Resolution |
|-----|------------|
| No board identity table | `machine_device_attachments` |
| No app runtime lifecycle table | `machine_runtime_app_sessions` |
| Credential vs app session confusion | Keep runtime-sessions; add app-sessions |
| No deterministic online_status | Config thresholds + ComputeMachineOnlineStatus |
| Reattach lacks full fingerprint | Extend reattach body + attachment row |
| Ops overview incomplete | Enriched REST + SQL aggregation |

## 17. Files to change

- `migrations/00017_machine_runtime_fleet.sql`, `db/schema/01_platform.sql`
- `db/queries/machine_device_attachments.sql`, `machine_runtime_app_sessions.sql`, `machine_ops_timeline.sql`, fleet admin queries
- `internal/app/machineruntime/session_service.go` (new)
- `internal/app/adminops/enterprise_ops.go`, `internal/app/activation/reattach.go`
- `internal/httpserver/admin_machine_enterprise_http.go`, `admin_fleet_write_http.go`
- `internal/grpcserver/machine_runtime_session_grpc.go`, `machine_grpc_services.go`, `interceptors.go`
- `proto/avf/machine/v1/runtime_session.proto`
- `internal/config/config.go` (online thresholds)
- `tools/production_full_test/*` (new route/RPC coverage)
- Docs under `docs/api/`, `docs/reports/machine-runtime-fleet/`

## 18. DB migrations required

- Goose `00017_machine_runtime_fleet.sql` (Up/Down)

## 19. SQLC queries required

- Device attachments CRUD + replace active
- Runtime app sessions start/heartbeat/end/list/current
- Admin operational overview aggregation
- Timeline UNION extensions

## 20. Tests required

- Migration invariants, service unit tests, httpserver RBAC tests, grpc auth tests, production harness extensions (scenarios A–J).

## 21. Deployment & production test requirements

- Deploy from main via `deploy-prod.yml`; inline pg_dump before goose.
- 3-pass production suite with prefix `AVF-RUNTIME-FLEET-{UTC}`; fake-pass audit; no secrets in reports.

---

**Gate:** Proceed to P1 migration.
