#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT full coverage wrapper with production-safe canary guards.

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PYTHON="${PYTHON:-python3}"
if ! command -v "${PYTHON}" >/dev/null 2>&1; then
  PYTHON="python"
fi
if ! command -v "${PYTHON}" >/dev/null 2>&1 && [[ -x "/c/Python314/python.exe" ]]; then
  PYTHON="/c/Python314/python.exe"
fi
REPORT_DIR="${ROOT}/reports/test"
EVIDENCE_DIR="${REPORT_DIR}/mqtt-full-evidence"
mkdir -p "${REPORT_DIR}" "${EVIDENCE_DIR}"

MQTT_HOST="${MQTT_HOST:-}"
MQTT_PORT="${MQTT_PORT:-1883}"
MQTT_TOPIC_PREFIX="${MQTT_TOPIC_PREFIX:-}"
MQTT_PRODUCTION="${MQTT_PRODUCTION:-false}"
OUT_JSON="${REPORT_DIR}/mqtt-full-coverage.json"
OUT_MD="${REPORT_DIR}/mqtt-full-coverage.md"

flows_json="$(mktemp)"
trap 'rm -f "${flows_json}"' EXIT

"${PYTHON}" - "${flows_json}" <<'PY'
import json
import sys
from pathlib import Path

flows = [
    {"name": "connect", "topics": [], "classification": "safe-read", "priority": "P0"},
    {"name": "telemetry_publish", "topics": ["{prefix}/{machineId}/telemetry", "{prefix}/machines/{machineId}/telemetry"], "classification": "local-write", "priority": "P0"},
    {"name": "command_subscribe", "topics": ["{prefix}/{machineId}/commands/dispatch", "{prefix}/machines/{machineId}/commands"], "classification": "safe-read", "priority": "P0"},
    {"name": "command_dispatch", "topics": ["{prefix}/{machineId}/commands/dispatch", "{prefix}/machines/{machineId}/commands"], "classification": "canary-write", "priority": "P0"},
    {"name": "ack_success", "topics": ["{prefix}/{machineId}/commands/ack"], "classification": "canary-write", "priority": "P0"},
    {"name": "ack_duplicate", "topics": ["{prefix}/{machineId}/commands/ack"], "classification": "canary-write", "priority": "P1"},
    {"name": "ack_timeout_no_ack", "topics": ["{prefix}/{machineId}/commands/ack"], "classification": "hardware-required", "priority": "P0"},
    {"name": "invalid_payload", "topics": ["{prefix}/{machineId}/telemetry"], "classification": "local-write", "priority": "P1"},
    {"name": "topic_prefix_correctness", "topics": ["{prefix}/#"], "classification": "safe-read", "priority": "P0"},
    {"name": "crlf_windows_handling", "topics": ["{prefix}/{machineId}/telemetry"], "classification": "local-write", "priority": "P1"},
    {"name": "reconnect_offline", "topics": ["{prefix}/{machineId}/telemetry"], "classification": "hardware-required", "priority": "P1"},
    {"name": "acl_topic_isolation", "topics": ["{prefix}/other-machine/#"], "classification": "production-readonly", "priority": "P1"},
]
for flow in flows:
    flow.update({"status": "partial", "reason": "not executed", "evidence_path": ""})
Path(sys.argv[1]).write_text(json.dumps(flows, indent=2), encoding="utf-8")
PY

broker_status="blocked-missing-seed"
broker_reason=""
if [[ -z "${MQTT_HOST}" || -z "${MQTT_TOPIC_PREFIX}" ]]; then
  broker_reason="MQTT_HOST and MQTT_TOPIC_PREFIX are required"
elif ! command -v mosquitto_pub >/dev/null 2>&1 || ! command -v mosquitto_sub >/dev/null 2>&1; then
  broker_status="blocked-tooling"
  broker_reason="mosquitto_pub/mosquitto_sub not installed"
else
  sub_log="${EVIDENCE_DIR}/connect-subscribe.log"
  set +e
  timeout 8 mosquitto_sub -h "${MQTT_HOST}" -p "${MQTT_PORT}" -t "${MQTT_TOPIC_PREFIX}/healthcheck/coverage" -C 1 >"${sub_log}" 2>&1 &
  sub_pid=$!
  sleep 1
  mosquitto_pub -h "${MQTT_HOST}" -p "${MQTT_PORT}" -t "${MQTT_TOPIC_PREFIX}/healthcheck/coverage" -m '{"coverage":"ping"}' >"${EVIDENCE_DIR}/connect-publish.log" 2>&1
  pub_ec=$?
  wait "${sub_pid}"
  sub_ec=$?
  set -e
  if [[ "${pub_ec}" -eq 0 && ( "${sub_ec}" -eq 0 || "${sub_ec}" -eq 27 ) ]] && grep -q '"coverage":"ping"' "${sub_log}"; then
    broker_status="reachable"
    broker_reason="publish/subscribe round-trip evidence captured"
  else
    broker_status="fail"
    broker_reason="publish/subscribe evidence missing (pub=${pub_ec}, sub=${sub_ec})"
  fi
fi

"${PYTHON}" - "${flows_json}" "${OUT_JSON}" "${OUT_MD}" "${broker_status}" "${broker_reason}" "${MQTT_HOST}" "${MQTT_PORT}" "${MQTT_TOPIC_PREFIX}" "${EVIDENCE_DIR}" <<'PY'
import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

flows = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
broker_status, broker_reason = sys.argv[4], sys.argv[5]
host, port, prefix = sys.argv[6], sys.argv[7], sys.argv[8]
evidence_dir = Path(sys.argv[9])
prod = os.environ.get("MQTT_PRODUCTION") == "true"
confirm = (
    os.environ.get("ALLOW_PROD_WRITES") == "true"
    and os.environ.get("PROD_WRITE_CONFIRMATION") == "RUN_DESTRUCTIVE_PRODUCTION_TESTS"
    and bool(os.environ.get("CANARY_MACHINE_ID"))
)

for flow in flows:
    cls = flow["classification"]
    if broker_status != "reachable":
        flow["status"] = broker_status
        flow["reason"] = broker_reason
    elif cls == "safe-read":
        flow["status"] = "pass" if flow["name"] == "connect" else "partial"
        flow["reason"] = broker_reason if flow["name"] == "connect" else "broker reachable; flow needs scenario assertion"
        flow["evidence_path"] = str(evidence_dir / "connect-subscribe.log")
    elif prod and cls in {"local-write", "canary-write"} and not confirm:
        flow["status"] = "blocked-production-confirmation"
        flow["reason"] = "production write requires ALLOW_PROD_WRITES plus canary env"
    elif cls == "hardware-required":
        flow["status"] = "blocked-hardware"
        flow["reason"] = "requires real canary device reconnect/no-ACK evidence"
    else:
        flow["status"] = "partial"
        flow["reason"] = "covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip"

summary = {
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "mqtt_host": host,
    "mqtt_port": port,
    "mqtt_topic_prefix": prefix,
    "broker_status": broker_status,
    "broker_reason": broker_reason,
    "total_flows": len(flows),
    "passed": sum(1 for f in flows if f["status"] == "pass"),
    "failed": sum(1 for f in flows if f["status"] == "fail"),
    "partial": sum(1 for f in flows if f["status"] == "partial"),
    "blocked": sum(1 for f in flows if str(f["status"]).startswith("blocked")),
}
Path(sys.argv[2]).write_text(json.dumps({"summary": summary, "flows": flows}, indent=2), encoding="utf-8")
with Path(sys.argv[3]).open("w", encoding="utf-8") as f:
    f.write("# MQTT Full Coverage\n\n")
    for key in ("generated_at", "mqtt_host", "mqtt_port", "mqtt_topic_prefix", "broker_status", "total_flows", "passed", "failed", "partial", "blocked"):
        f.write(f"- {key.replace('_', ' ').title()}: `{summary.get(key)}`\n")
    f.write(f"- Broker reason: {summary.get('broker_reason')}\n\n")
    f.write("| Flow | Priority | Class | Status | Reason |\n")
    f.write("|---|---|---|---|---|\n")
    for flow in flows:
        f.write(f"| `{flow['name']}` | {flow['priority']} | {flow['classification']} | **{flow['status']}** | {flow['reason'][:120]} |\n")
PY

echo "Wrote ${OUT_JSON} and ${OUT_MD}"
if [[ "${broker_status}" == "fail" ]]; then
  exit 1
fi
exit 0
