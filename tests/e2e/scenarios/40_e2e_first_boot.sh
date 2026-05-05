#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 8 / E2E-40: First boot — activation if needed, catalog/media, MQTT heartbeat, optional admin machine health.

set -euo pipefail

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

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

SID="E2E-40-first-boot"
start_step "phase8-${SID}"

MID="$(get_data machineId)"
ORG="$(get_data organizationId)"
PID="$(get_data productId)"
IDS_JSON="$(jq -nc --arg m "${MID:-}" --arg o "${ORG:-}" --arg p "${PID:-}" '{machineId:$m,organizationId:$o,productId:$p}')"
APIS_JSON='[]'
EVID_JSON='[]'
EXPECTED="Machine registered; sale-catalog reachable; MQTT heartbeat published; optional admin /health."
ACTUAL=""
REMED=""

if [[ -z "$MID" || "$MID" == "null" ]]; then
  ACTUAL="missing machineId in ${E2E_RUN_DIR}/test-data.json — run WA-SETUP-01 (01_web_admin_setup.sh) first"
  EVID_JSON="$(jq -nc --arg f "${E2E_RUN_DIR}/test-data.json" '[$f]')"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Provision machine via web admin setup; see docs/testing/e2e-remediation-playbook.md"
  end_step failed "E2E-40: ${ACTUAL}"
  exit 1
fi

MT="$(get_secret machineToken 2>/dev/null || true)"
if [[ -z "$MT" ]]; then
  if ! bash "${E2E_SCENARIO_DIR}/02_machine_activation_bootstrap_rest.sh"; then
    ACTUAL="activation/bootstrap failed — see ${E2E_RUN_DIR}/rest/vm-claim.meta.json and vm-claim.response.json"
    EVID_JSON="$(jq -nc --arg a "${E2E_RUN_DIR}/rest/vm-claim.meta.json" --arg b "${E2E_RUN_DIR}/rest/vm-claim.response.json" '[$a,$b]')"
    APIS_JSON='["POST /v1/setup/activation-codes/claim","GET /v1/setup/machines/{id}/bootstrap"]'
    phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Fix E2E_ACTIVATION_CODE or secrets; see remediation playbook (activation)"
    end_step failed "E2E-40: ${ACTUAL}"
    exit 1
  fi
  APIS_JSON="$(jq -nc '["POST /v1/setup/activation-codes/claim","GET /v1/setup/machines/{id}/bootstrap"]')"
fi

if ! bash "${E2E_SCENARIO_DIR}/03_catalog_media_sync_rest.sh"; then
  ACTUAL="catalog/media sync failed — see ${E2E_RUN_DIR}/rest/vm-sale-catalog.meta.json"
  EVID_JSON="$(jq -nc --arg f "${E2E_RUN_DIR}/rest/vm-sale-catalog.meta.json" '[$f]')"
  APIS_JSON="$(echo "$APIS_JSON" | jq -c '. + ["GET /v1/machines/{id}/sale-catalog?include_images=true"]')"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Ensure machineToken and planogram; rest/vm-sale-catalog.response.json"
  end_step failed "E2E-40: ${ACTUAL}"
  exit 1
fi
APIS_JSON="$(echo "$APIS_JSON" | jq -c '. + ["GET /v1/machines/{id}/sale-catalog?include_images=true"]')"
EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/rest/vm-sale-catalog.meta.json" '. + [$f]')"

# --- MQTT heartbeat (optional when broker/clients absent) ---
# shellcheck source=../lib/e2e_mqtt.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_mqtt.sh"
MQTT_NOTE="skipped_no_mosquitto_or_broker"
if command -v mosquitto_pub >/dev/null 2>&1 && e2e_mqtt_tcp_open && e2e_mqtt_resolve_topics; then
  EID="e2e-p8-hb-${RANDOM}"
  TSNOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  PAYLOAD="$(jq -nc \
    --arg mid "$MID" \
    --arg eid "$EID" \
    --arg ts "$TSNOW" \
    '{
      schema_version: 1,
      event_id: $eid,
      machine_id: $mid,
      event_type: "heartbeat",
      occurred_at: $ts,
      dedupe_key: $eid,
      payload: { type: "heartbeat", source: "e2e-phase8-40" }
    }')"
  if e2e_mqtt_publish "${E2E_MQTT_TOPIC_TELEMETRY}" "$PAYLOAD" "phase8-first-boot-telemetry"; then
    APIS_JSON="$(echo "$APIS_JSON" | jq -c --arg t "${E2E_MQTT_TOPIC_TELEMETRY}" '. + ["MQTT " + $t]')"
    MQTT_NOTE="mqtt_heartbeat_published"
    EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/mqtt/phase8-first-boot-telemetry.publish.json" '. + [$f]')"
  else
    MQTT_NOTE="mqtt_publish_failed"
    EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/mqtt/phase8-first-boot-telemetry.publish.json" '. + [$f]')"
    ACTUAL="MQTT publish failed — mosquitto_pub non-zero; broker ACL or topic mismatch. Telemetry topic: ${E2E_MQTT_TOPIC_TELEMETRY}"
    phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Start local broker; set MQTT_* env per docs/api/mqtt-contract.md"
    end_step failed "E2E-40: ${ACTUAL}"
    exit 1
  fi
else
  log_warn "E2E-40: MQTT heartbeat SKIPPED (no mosquitto or broker unreachable) — recorded as skip with remediation"
  APIS_JSON="$(echo "$APIS_JSON" | jq -c '. + ["MQTT telemetry (skipped)"]')"
  MQTT_NOTE="skipped_broker_or_client"
  REMED="Start mosquitto on MQTT_HOST:MQTT_PORT or install mosquitto_pub; see e2e-troubleshooting MQTT Phase 7"
fi

# --- Admin machine health (optional) ---
ADM="$(get_secret adminAccessToken 2>/dev/null || true)"
[[ -z "$ADM" ]] && ADM="${E2E_ADMIN_TOKEN:-}"
if [[ -n "$ADM" ]] && [[ -n "$ORG" && "$ORG" != "null" ]]; then
  export ADMIN_TOKEN="$ADM"
  code_h="$(e2e_http_request_json "GET" "p8-40-machine-health" "/v1/admin/organizations/${ORG}/machines/${MID}/health" "")"
  APIS_JSON="$(echo "$APIS_JSON" | jq -c '. + ["GET /v1/admin/organizations/{org}/machines/{id}/health"]')"
  if [[ "$code_h" == "200" ]]; then
    ADMIN_NOTE="admin_health_http_200"
    EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/rest/p8-40-machine-health.meta.json" '. + [$f]')"
  else
    ADMIN_NOTE="admin_health_http_${code_h}"
    EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/rest/p8-40-machine-health.meta.json" '. + [$f]')"
  fi
else
  ADMIN_NOTE="no_admin_token_org_skip_health"
fi

ACTUAL="activation_ok catalog_ok ${MQTT_NOTE} ${ADMIN_NOTE}"
RESULT="pass"
[[ "$MQTT_NOTE" == "skipped_broker_or_client" ]] && RESULT="skip"

phase8_record "$SID" "$RESULT" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "$REMED"

if [[ "$RESULT" == "pass" ]]; then
  end_step passed "E2E-40 first boot completed"
elif [[ "$RESULT" == "skip" ]]; then
  end_step skipped "E2E-40 completed with MQTT skipped — see phase8-scenario-results.jsonl and remediation"
  log_mqtt_contract_issue "P2" "$SID" "40_e2e_first_boot.sh" "mqtt-heartbeat" "MQTT" "telemetry topic" "First-boot narrative skipped MQTT heartbeat (broker/clients missing) — field parity gap in CI" "Incomplete protocol coverage" "Run Phase 7 with broker or document skip" "${E2E_RUN_DIR}/reports/phase8-scenario-results.jsonl"
else
  end_step failed "E2E-40 unexpected result ${RESULT}"
  exit 1
fi
e2e_flow_review_scenario_complete "$SID" "40_e2e_first_boot.sh" "flow-review-complete" "phase8_first_boot_reviewed"
exit 0
