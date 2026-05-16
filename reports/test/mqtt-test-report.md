# MQTT connectivity & broker contract report

- Generated (UTC): `2026-05-16T09:30:00Z` (assemble manually after runs; timestamps in evidence files are authoritative.)
- Repository: `avf-vending-api`

## 1. Mosquitto clients (`mosquitto_pub` / `mosquitto_sub`)

Evidence: `reports/test/mqtt-evidence/broker-preflight.txt`

On this workstation (Git Bash + `e2e_prepend_windows_tool_paths`):

- **`mosquitto_pub`**: `/c/Program Files/Mosquitto/mosquitto_pub`
- **`mosquitto_sub`**: `/c/Program Files/Mosquitto/mosquitto_sub`

PowerShell `where.exe` alone did **not** find them on `PATH`; the E2E harness prepends `C:\Program Files\Mosquitto` under Git Bash (see `tests/e2e/lib/e2e_common.sh`).

## 2. Broker reachability

Evidence: `reports/test/mqtt-evidence/broker-preflight.txt`

- **TCP probe**: `tcp://127.0.0.1:1883` → **OPEN** (same check as `e2e_mqtt_tcp_open`).

## 3. MQTT-focused Go tests

Command:

```text
go test ./internal/platform/mqtt/... ./internal/app/device/... ./internal/app/telemetryapp/... \
  -run 'MQTT|Mqtt|Telemetry|Command|Ack|Topic|Broker' -count=1
```

Output: `reports/test/mqtt-evidence/go-test-mqtt.txt`

Result: **PASS** (all packages `ok`).

## 4. Local MQTT E2E (`tests/e2e/run-mqtt-local.sh`)

Console log: `reports/test/mqtt-evidence/run-mqtt-local.console.txt`

- First run **failed** without `machineId` / `MQTT_MACHINE_ID` (harness error: `set MQTT_MACHINE_ID or machineId in test-data`).
- Successful run used:

  ```bash
  export MQTT_MACHINE_ID='55555555-5555-5555-5555-555555555555'
  export MQTT_TOPIC_PREFIX='avf/devices'
  ./tests/e2e/run-mqtt-local.sh
  ```

  Exit code: **0**.

Copied artifacts: `reports/test/mqtt-evidence/e2e-run-20260516T092608Z/` (`mqtt/*.json`, `mqtt/*.log`, `reports/mqtt-contract-summary.md`, `reports/mqtt-contract-results.jsonl`).

Contract summary excerpt: **6 pass**, **0 fail**, **4 skip** (expected skips without `ADMIN_TOKEN`).

## 5. Mosquitto exit code 27 (Windows / Git Bash)

**Documentation**: `docs/api/mqtt-contract.md` — section **“Phase 7 local smoke: Mosquitto client exit codes”**.

**Helper behavior** (`tests/e2e/lib/e2e_mqtt.sh`):

| Helper | Exit 27 handling |
|--------|------------------|
| `e2e_mqtt_subscribe_accept_connect` | Treat **0** or **27** as success for subscribe **idle/connect** probe (unchanged contract). |
| `e2e_mqtt_publish` | **27** succeeds **only if** `*.publish.log` does **not** match connection/auth/error patterns (`error`, `unable to connect`, `connection refused`, `not authorised`, `denied`). Otherwise **27** is a failure — avoids masking broker failures. |
| `e2e_mqtt_sub_join_payload_ok` (new) | Used by **`32_mqtt_command_ack.sh`** after `wait` on background `mosquitto_sub`: **27** counts as success **only when** the subscribe log’s **first line is non-empty** (payload received). Empty log + **27** ⇒ failure. |

Evidence: subscribe probe meta recorded exit **27** with **pass** in MQTT-30 (`mqtt/command.meta.json` in the copied run).

## 6. Topics — no organization / tenant segments

Resolved topics (legacy layout) from the successful run:

- Telemetry: `avf/devices/55555555-5555-5555-5555-555555555555/telemetry`
- Commands in: `avf/devices/55555555-5555-5555-5555-555555555555/commands/dispatch`
- Command ACK: `avf/devices/55555555-5555-5555-5555-555555555555/commands/ack`

Shape matches `internal/platform/mqtt/topics.go` and `tests/e2e/lib/e2e_mqtt.sh`: `{prefix}/{machineId}/…` or enterprise `{prefix}/machines/{machineId}/…` — **no** `organization`, `tenant`, or `scope` path segments.

## 7. Payloads — no organization fields

Checked E2E artifacts under `reports/test/mqtt-evidence/e2e-run-20260516T092608Z/mqtt/`:

- **Telemetry heartbeat** (`telemetry.publish.json`): `schema_version`, `event_id`, `machine_id`, `event_type`, `occurred_at`, `dedupe_key`, nested `payload` — **no** `organization_id` / `organizationId` / `tenant`.
- **Synthetic command** (`command.subscribe.log`): `command_id`, `machine_id`, `sequence`, `command_type`, `payload`, `idempotency_key` — **no** organization fields.
- **Command ACK** (`command.ack.publish.json` body): `command_id`, `machine_id`, `occurred_at`, `status`, `sequence`, `dedupe_key`, `payload` — **no** organization fields.

Static scan: `internal/platform/mqtt/*.go` contains **no** `organization` / `tenant` / `scope_id` string matches for topic construction.

## Evidence index

| Path | Description |
|------|-------------|
| `reports/test/mqtt-evidence/broker-preflight.txt` | Client paths + TCP probe |
| `reports/test/mqtt-evidence/go-test-mqtt.txt` | Go test output |
| `reports/test/mqtt-evidence/run-mqtt-local.console.txt` | E2E runner console |
| `reports/test/mqtt-evidence/e2e-run-20260516T092608Z/` | MQTT logs + contract summary |
| `scripts/test/mqtt_broker_preflight.sh` | Repeatable preflight script |

## Final pass / fail

| Gate | Result |
|------|--------|
| `mosquitto_pub` / `mosquitto_sub` available (Git Bash + Mosquitto install) | **PASS** |
| Broker TCP reachable (`127.0.0.1:1883`) | **PASS** |
| Go tests (`mqtt` / `device` / `telemetryapp` filter) | **PASS** |
| `run-mqtt-local.sh` (with `MQTT_MACHINE_ID` + prefix) | **PASS** |
| Topic layout without org/tenant segments | **PASS** |
| Telemetry + command + ACK payloads without organization fields | **PASS** |
| **Overall** | **PASS** |

**Note:** Phase 7 MQTT E2E **requires** `MQTT_MACHINE_ID` or `machineId` in test-data; without it the suite exits non-zero before broker scenarios.
