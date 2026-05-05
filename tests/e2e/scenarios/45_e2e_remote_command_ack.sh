#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 8 / E2E-45: Remote command — reuse MQTT-32 (admin noop + ACK), then publish commands/receipt (result).

set -euo pipefail

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_mqtt.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_mqtt.sh"

phase8_record() {
  local scenario_id="$1" result="$2" ids_json="$3" apis_json="$4" expected_state="$5" actual_state="$6" evidence_json="$7" remediation="$8"
  mkdir -p "${E2E_RUN_DIR}/reports"
  jq -nc \
    --arg ts "$(now_utc)" \
    --arg scenario_id "$scenario_id" \
    --arg result "$result" \
    --argjson input_ids "$ids_json" \
    --argjson apis_topics_used "$apis_json" \
    --arg expected_state "$expected_state" \
    --arg actual_state "$actual_state" \
    --argjson evidence_files "$evidence_json" \
    --arg remediation "$remediation" \
    '{ts:$ts,scenario_id:$scenario_id,input_ids:$input_ids,apis_topics_used:$apis_topics_used,expected_state:$expected_state,actual_state:$actual_state,result:$result,evidence_files:$evidence_files,remediation:$remediation}' \
    >>"${E2E_RUN_DIR}/reports/phase8-scenario-results.jsonl"
}

SID="E2E-45-remote-command-ack"
start_step "phase8-${SID}"

ORG="$(get_data organizationId)"
MID="$(get_data machineId)"
IDS_JSON="$(jq -nc --arg o "${ORG:-}" --arg m "${MID:-}" '{organizationId:$o,machineId:$m}')"
APIS_JSON='["POST /v1/admin/organizations/{org}/machines/{id}/commands","MQTT commands/dispatch (or enterprise .../commands)","MQTT commands/ack","MQTT commands/receipt","GET /v1/admin/organizations/{org}/commands/{id}"]'
EXPECTED="Command dispatched; device receives; ACK + receipt published; admin GET shows terminal / accepted attempt when full admin path used."
EVID_JSON='[]'

set +e
bash "${E2E_SCENARIO_DIR}/32_mqtt_command_ack.sh"
ec32=$?
set -e

EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/mqtt/command.ack.json" '. + [$f]')"
EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/rest/mqtt-admin-dispatch.meta.json" '. + [$f]')"

if [[ "$ec32" -ne 0 ]]; then
  ACTUAL="MQTT-32 failed ec=${ec32} — ${E2E_RUN_DIR}/mqtt/command.subscribe.log and reports/mqtt-contract-results.jsonl"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Broker ACL; MQTT_TOPIC_*; production guard E2E_MQTT_COMMAND_TEST_ACK"
  end_step failed "E2E-45: ${ACTUAL}"
  exit 1
fi

if [[ ! -f "${E2E_RUN_DIR}/mqtt/command.ack.json" ]]; then
  ACTUAL="production_safety_or_broker_skip — no command.ack.json (see MQTT-32 guard in Phase 7)"
  phase8_record "$SID" "skip" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Run on local/staging or set e2eTestMachine + E2E_MQTT_COMMAND_TEST_ACK for production noop tests"
  end_step skipped "E2E-45: command ACK path skipped (see MQTT-32)"
  exit 0
fi

CID="$(jq -r '.command_id // empty' "${E2E_RUN_DIR}/mqtt/command.ack.json")"
[[ -z "$CID" ]] && CID="$(jq -r '.commandId // empty' "${E2E_RUN_DIR}/mqtt/command.ack.json")"
if [[ -z "$CID" ]]; then
  ACTUAL="could not read command_id from command.ack.json"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "${E2E_RUN_DIR}/mqtt/command.ack.json"
  end_step failed "E2E-45: ${ACTUAL}"
  exit 1
fi

e2e_mqtt_resolve_topics || true
layout="$(echo "${MQTT_TOPIC_LAYOUT:-legacy}" | tr '[:upper:]' '[:lower:]')"
prefix="${MQTT_TOPIC_PREFIX:-avf/devices}"
prefix="${prefix%/}"
if [[ "$layout" == "enterprise" ]]; then
  REC_TOPIC="${prefix}/machines/${MID}/commands/receipt"
else
  REC_TOPIC="${prefix}/${MID}/commands/receipt"
fi
[[ -n "${MQTT_TOPIC_COMMAND_RECEIPT:-}" ]] && REC_TOPIC="${MQTT_TOPIC_COMMAND_RECEIPT}"

OCC="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
DED="e2e-p8-receipt-${CID}"
REC_JSON="$(jq -nc \
  --arg cid "$CID" \
  --arg mid "$MID" \
  --arg occ "$OCC" \
  --arg dk "$DED" \
  '{command_id:$cid, machine_id:$mid, occurred_at:$occ, status:"completed", dedupe_key:$dk, payload:{result:"noop_ok", source:"e2e-phase8-45"}}')"

if ! command -v mosquitto_pub >/dev/null 2>&1 || ! e2e_mqtt_tcp_open; then
  ACTUAL="ACK ok but receipt publish skipped (mosquitto or broker)"
  phase8_record "$SID" "skip" "$IDS_JSON" "$(echo "$APIS_JSON" | jq -c --arg r "$REC_TOPIC" '. + ["MQTT " + $r]')" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Install mosquitto clients; start broker"
  end_step skipped "E2E-45: MQTT receipt skipped"
  exit 0
fi

if ! e2e_mqtt_publish "$REC_TOPIC" "$REC_JSON" "phase8-command-receipt"; then
  ACTUAL="mosquitto_pub commands/receipt failed — topic ${REC_TOPIC}"
  EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/mqtt/phase8-command-receipt.publish.json" '. + [$f]')"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "ACL; topic layout docs/api/mqtt-contract.md"
  end_step failed "E2E-45: ${ACTUAL}"
  exit 1
fi

EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/mqtt/phase8-command-receipt.publish.json" '. + [$f]')"
ACTUAL="command_id=${CID} ack_ok receipt_published_to_${REC_TOPIC}"
phase8_record "$SID" "pass" "$IDS_JSON" "$(echo "$APIS_JSON" | jq -c --arg r "$REC_TOPIC" '. + ["MQTT " + $r]')" "$EXPECTED" "$ACTUAL" "$EVID_JSON" ""
end_step passed "E2E-45 remote command ACK + receipt completed"
exit 0
