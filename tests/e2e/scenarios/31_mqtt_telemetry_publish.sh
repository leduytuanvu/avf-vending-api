#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT-31: publish heartbeat envelope to telemetry; optional REST verification (partial if absent).

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_mqtt.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_mqtt.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"

FLOW_ID="MQTT-31"
ec=0

e2e_mqtt_resolve_topics || exit 2

MID="${E2E_MQTT_MACHINE_ID}"
EID="e2e-tel-hb-${RANDOM}"
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
    payload: { type: "heartbeat", source: "e2e-mqtt-31", note: "e2e telemetry heartbeat" }
  }')"

set +e
e2e_mqtt_publish "${E2E_MQTT_TOPIC_TELEMETRY}" "$PAYLOAD" "telemetry"
pub_ec=$?
set -e
if [[ "$pub_ec" -ne 0 ]]; then
  mqtt_contract_record "$FLOW_ID" "publish-heartbeat" "${E2E_MQTT_TOPIC_TELEMETRY}" "fail" "mosquitto_pub_${pub_ec}"
  e2e_append_test_event "$FLOW_ID" "publish-heartbeat" "MQTT" "${E2E_MQTT_TOPIC_TELEMETRY}" "fail" "pub_failed" "{}"
  exit 1
fi
mqtt_contract_record "$FLOW_ID" "publish-heartbeat" "${E2E_MQTT_TOPIC_TELEMETRY}" "pass" "published event_id=${EID}"
e2e_append_test_event "$FLOW_ID" "publish-heartbeat" "MQTT" "${E2E_MQTT_TOPIC_TELEMETRY}" "pass" "ok" "{}"

# No stable public REST read-back of a single MQTT event_id in this repo — mark verification partial.
mqtt_contract_record "$FLOW_ID" "verify-rest" "—" "skip" "partial_no_per_event_mqtt_read_api_documented"
e2e_append_test_event "$FLOW_ID" "verify-ingest" "MQTT" "${E2E_MQTT_TOPIC_TELEMETRY}" "skipped" "partial_projection_async" "{}"

ORG="$(get_data organizationId)"
if [[ -n "${ADMIN_TOKEN:-}" ]] && [[ -n "${ORG:-}" ]] && [[ "$ORG" != "null" ]]; then
  code_h="$(e2e_http_request_json "GET" "mqtt-hint-health" "/v1/admin/organizations/${ORG}/machines/${MID}/health" "")"
  if [[ "$code_h" == "200" ]]; then
    mqtt_contract_record "$FLOW_ID" "hint-machine-health" "GET .../machines/{id}/health" "pass" "optional_freshness_signal"
  else
    mqtt_contract_record "$FLOW_ID" "hint-machine-health" "GET .../machines/{id}/health" "skip" "http_${code_h}"
  fi
else
  mqtt_contract_record "$FLOW_ID" "hint-machine-health" "—" "skip" "no_ADMIN_TOKEN_or_organizationId"
fi

exit "$ec"
