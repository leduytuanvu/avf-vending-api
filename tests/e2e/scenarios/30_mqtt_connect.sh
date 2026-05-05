#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT-30: broker reachability, subscribe probe on command topic, telemetry publish smoke.

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_mqtt.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_mqtt.sh"

FLOW_ID="MQTT-30"
MID="${MQTT_MACHINE_ID:-}"
[[ -z "$MID" ]] && MID="$(get_data machineId)"
[[ "$MID" == "null" ]] && MID=""
ec=0
MLOG="$(e2e_mqtt_log_dir)/connect.log"
mkdir -p "$(e2e_mqtt_log_dir)" "${E2E_RUN_DIR}/reports"

{
  echo "=== MQTT connect $(now_utc) ==="
  echo "host=${MQTT_HOST} port=${MQTT_PORT:-1883} tls=${MQTT_USE_TLS:-false}"
  echo "machineId=${MID:-<unset>}"
} >>"$MLOG"

if ! e2e_mqtt_tcp_open; then
  echo "FAIL: TCP connect to ${MQTT_HOST}:${MQTT_PORT:-1883}" >>"$MLOG"
  mqtt_contract_record "$FLOW_ID" "tcp" "—" "fail" "broker_tcp_closed"
  e2e_append_test_event "$FLOW_ID" "tcp" "MQTT" "tcp://${MQTT_HOST}:${MQTT_PORT:-1883}" "fail" "broker unreachable" "{}"
  exit 1
fi
mqtt_contract_record "$FLOW_ID" "tcp" "tcp://${MQTT_HOST}:${MQTT_PORT:-1883}" "pass" "port_open"
e2e_append_test_event "$FLOW_ID" "tcp" "MQTT" "tcp://${MQTT_HOST}:${MQTT_PORT:-1883}" "pass" "port_open" "{}"

if ! e2e_mqtt_resolve_topics; then
  echo "FAIL: topic resolution (need machineId and prefix/layout)" >>"$MLOG"
  mqtt_contract_record "$FLOW_ID" "resolve-topics" "—" "fail" "missing_machine_or_prefix"
  exit 1
fi

if [[ -n "${MQTT_TOPIC_TELEMETRY:-}" ]]; then
  log_mqtt_contract_issue "P2" "$FLOW_ID" "30_mqtt_connect" "resolve-topics" "MQTT" "${E2E_MQTT_TOPIC_TELEMETRY:-}" \
    "MQTT topic layout came only from MQTT_TOPIC_* env overrides (not derived in harness from bootstrap/config)" \
    "Field clients may lack a documented way to discover broker topics from API responses" \
    "Expose topic prefix and layout via GetBootstrap or config; document in mqtt-contract.md" \
    "$MLOG"
fi

echo "command_in_topic=${E2E_MQTT_TOPIC_COMMAND_IN}" >>"$MLOG"
echo "telemetry_topic=${E2E_MQTT_TOPIC_TELEMETRY}" >>"$MLOG"

set +e
e2e_mqtt_subscribe_accept_connect "${E2E_MQTT_TOPIC_COMMAND_IN}" 5 "command"
sub_ec=$?
set -e
cat "$(e2e_mqtt_log_dir)/command.subscribe.log" >>"$MLOG" 2>/dev/null || true
if [[ "$sub_ec" -ne 0 ]]; then
  echo "FAIL: subscribe probe exit ${sub_ec}" >>"$MLOG"
  mqtt_contract_record "$FLOW_ID" "subscribe-command" "${E2E_MQTT_TOPIC_COMMAND_IN}" "fail" "subscribe_exit_${sub_ec}"
  e2e_append_test_event "$FLOW_ID" "subscribe-command" "MQTT" "${E2E_MQTT_TOPIC_COMMAND_IN}" "fail" "subscribe_exit_${sub_ec}" "{}"
  ec=1
else
  mqtt_contract_record "$FLOW_ID" "subscribe-command" "${E2E_MQTT_TOPIC_COMMAND_IN}" "pass" "connected_or_timeout_ok"
  e2e_append_test_event "$FLOW_ID" "subscribe-command" "MQTT" "${E2E_MQTT_TOPIC_COMMAND_IN}" "pass" "ok" "{}"
fi

HB="$(jq -nc --arg mid "$MID" --arg eid "e2e-connect-hb-${RANDOM}" --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{schema_version:1, event_id:$eid, machine_id:$mid, event_type:"heartbeat", occurred_at:$ts, dedupe_key:$eid, payload:{type:"heartbeat", phase:"connect"}}')"
set +e
e2e_mqtt_publish "${E2E_MQTT_TOPIC_TELEMETRY}" "$HB" "telemetry-connect"
pub_ec=$?
set -e
echo "telemetry_publish_exit=${pub_ec}" >>"$MLOG"
if [[ "$pub_ec" -ne 0 ]]; then
  mqtt_contract_record "$FLOW_ID" "publish-telemetry" "${E2E_MQTT_TOPIC_TELEMETRY}" "fail" "mosquitto_pub_exit_${pub_ec}"
  e2e_append_test_event "$FLOW_ID" "publish-telemetry" "MQTT" "${E2E_MQTT_TOPIC_TELEMETRY}" "fail" "pub_failed" "{}"
  ec=1
else
  mqtt_contract_record "$FLOW_ID" "publish-telemetry" "${E2E_MQTT_TOPIC_TELEMETRY}" "pass" "mosquitto_pub_ok"
  e2e_append_test_event "$FLOW_ID" "publish-telemetry" "MQTT" "${E2E_MQTT_TOPIC_TELEMETRY}" "pass" "ok" "{}"
fi

exit "$ec"
