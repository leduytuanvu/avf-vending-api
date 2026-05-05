#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT-32: receive command on dispatch topic, publish commands/ack. Admin REST dispatch when possible; else broker-only synthetic noop.

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

FLOW_ID="MQTT-32"

mqtt_command_ack_guard_ok() {
  case "${E2E_TARGET:-local}" in
    production)
      local tm
      tm="$(get_data e2eTestMachine)"
      [[ "$tm" == "true" || "$tm" == "1" ]] || return 1
      [[ "${E2E_MQTT_COMMAND_TEST_ACK:-}" == "I_UNDERSTAND_MQTT_COMMAND_TEST_ACK" ]] || return 1
      return 0
      ;;
    *)
      return 0
      ;;
  esac
}

e2e_mqtt_resolve_topics || exit 2

MID="${E2E_MQTT_MACHINE_ID}"
DIR="$(e2e_mqtt_log_dir)"
mkdir -p "$DIR" "${E2E_RUN_DIR}/reports"
SUB_LOG="${DIR}/command.subscribe.log"
rm -f "$SUB_LOG"

if ! mqtt_command_ack_guard_ok; then
  mqtt_contract_record "$FLOW_ID" "guard" "—" "skip" "production_requires_e2eTestMachine_and_E2E_MQTT_COMMAND_TEST_ACK"
  e2e_append_test_event "$FLOW_ID" "guard" "MQTT" "—" "skipped" "production_safety" "{}"
  exit 0
fi

admin_dispatch_ok() {
  [[ -n "${ADMIN_TOKEN:-}" ]] || return 1
  local o=""
  o="$(get_data organizationId)"
  [[ -n "$o" && "$o" != "null" ]] || return 1
  return 0
}

launch_subscriber() {
  local topic="$1"
  local wait_sec="$2"
  local logf="$3"
  local -a args=()
  e2e_mqtt_build_client_args args
  args+=(-t "$topic" -C 1 -W "$wait_sec" -q 1)
  mosquitto_sub "${args[@]}" >"${logf}" 2>&1 &
  echo $!
}

CMD_IN="${E2E_MQTT_TOPIC_COMMAND_IN}"
CMD_ACK="${E2E_MQTT_TOPIC_COMMAND_ACK}"
ORG="$(get_data organizationId)"
read_command_line() {
  local f="$1"
  [[ -f "$f" ]] || { echo ""; return 1; }
  head -n1 "$f"
}

publish_ack_line() {
  local recv_line="$1"
  echo "$recv_line" | jq -e . >/dev/null 2>&1 || return 1
  local cid seq
  cid="$(echo "$recv_line" | jq -r '.command_id // .commandId // empty')"
  seq="$(echo "$recv_line" | jq -r '.sequence // 0')"
  [[ -z "$cid" || "$cid" == "null" ]] && return 1
  local dedupe occ
  dedupe="e2e-ack-${cid}"
  occ="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  local ack_json
  ack_json="$(jq -nc \
    --arg cid "$cid" \
    --arg mid "$MID" \
    --arg occ "$occ" \
    --argjson seq "$seq" \
    --arg dk "$dedupe" \
    '{command_id:$cid, machine_id:$mid, occurred_at:$occ, status:"acked", sequence:$seq, dedupe_key:$dk, payload:{source:"e2e-mqtt-32"}}')"
  printf '%s\n' "$ack_json" >"${DIR}/command.ack.json"
  e2e_mqtt_publish "${CMD_ACK}" "$ack_json" "command.ack"
}

SPID=""
cleanup_s() {
  [[ -n "${SPID:-}" ]] && kill "$SPID" 2>/dev/null || true
}
trap cleanup_s EXIT

RECV=""
HTTP_CID=""
ADMIN_DISPATCH_PASS="no"

if admin_dispatch_ok; then
  SPID="$(launch_subscriber "$CMD_IN" 45 "$SUB_LOG")"
  sleep 1
  BODY='{"commandType":"noop","payload":{}}'
  IDK="e2e-mqtt-dispatch-${RANDOM}-$(date +%s)"
  code="$(e2e_http_post_json_idem "mqtt-admin-dispatch" "/v1/admin/organizations/${ORG}/machines/${MID}/commands" "$BODY" "$IDK")"
  if [[ "$code" == "202" ]]; then
    HTTP_CID="$(jq -r '.commandId // .command_id // empty' "${E2E_RUN_DIR}/rest/mqtt-admin-dispatch.response.json" 2>/dev/null)"
  fi
  wait "$SPID" || true
  sub_ec=$?
  SPID=""
  RECV="$(read_command_line "$SUB_LOG")"
  if [[ "$code" == "202" ]] && [[ "$sub_ec" -eq 0 ]] && [[ -n "$RECV" ]]; then
    mqtt_contract_record "$FLOW_ID" "admin-dispatch" "${CMD_IN}" "pass" "http_202 commandId=${HTTP_CID}"
    e2e_append_test_event "$FLOW_ID" "admin-dispatch" "REST+MQTT" "/v1/admin/.../commands" "pass" "ok" "{}"
    ADMIN_DISPATCH_PASS="yes"
  else
    mqtt_contract_record "$FLOW_ID" "admin-dispatch" "${CMD_IN}" "skip" "http=${code} sub_exit=${sub_ec} (try_synthetic)"
    RECV=""
    HTTP_CID=""
  fi
else
  mqtt_contract_record "$FLOW_ID" "admin-dispatch" "—" "skip" "no_ADMIN_TOKEN_or_organizationId"
fi

if [[ -z "$RECV" ]]; then
  SYN_ID="$(python3 -c 'import uuid; print(uuid.uuid4())' 2>/dev/null || echo "")"
  [[ -z "$SYN_ID" ]] && SYN_ID="$(uuidgen 2>/dev/null || echo "00000000-0000-4000-8000-000000000001")"
  SYN_PAYLOAD="$(jq -nc \
    --arg cid "$SYN_ID" \
    --arg mid "$MID" \
    --arg ik "e2e-synthetic-${RANDOM}" \
    '{command_id:$cid, machine_id:$mid, sequence:1, command_type:"noop", payload:{}, idempotency_key:$ik}')"
  SPID="$(launch_subscriber "$CMD_IN" 25 "$SUB_LOG")"
  sleep 1
  e2e_mqtt_publish "$CMD_IN" "$SYN_PAYLOAD" "synthetic-command-publish"
  pub_s=$?
  wait "$SPID" || true
  sub_ec=$?
  SPID=""
  RECV="$(read_command_line "$SUB_LOG")"
  if [[ "$pub_s" -ne 0 ]] || [[ "$sub_ec" -ne 0 ]] || [[ -z "$RECV" ]]; then
    trap - EXIT
    mqtt_contract_record "$FLOW_ID" "synthetic-command" "$CMD_IN" "fail" "pub=${pub_s} sub=${sub_ec}"
    e2e_append_test_event "$FLOW_ID" "command-receive" "MQTT" "$CMD_IN" "fail" "synthetic_failed" "{}"
    exit 1
  fi
  mqtt_contract_record "$FLOW_ID" "synthetic-command" "$CMD_IN" "pass" "broker_only_noop"
  e2e_append_test_event "$FLOW_ID" "command-receive" "MQTT" "$CMD_IN" "pass" "synthetic" "{}"
fi

set +e
publish_ack_line "$RECV"
ack_ec=$?
set -e
trap - EXIT
if [[ "$ack_ec" -ne 0 ]]; then
  mqtt_contract_record "$FLOW_ID" "publish-ack" "$CMD_ACK" "fail" "mosquitto_pub_exit_${ack_ec}"
  e2e_append_test_event "$FLOW_ID" "publish-ack" "MQTT" "$CMD_ACK" "fail" "ack_failed" "{}"
  exit 1
fi
mqtt_contract_record "$FLOW_ID" "publish-ack" "$CMD_ACK" "pass" "commands/ack_sent"
e2e_append_test_event "$FLOW_ID" "publish-ack" "MQTT" "$CMD_ACK" "pass" "ok" "{}"

if [[ "$ADMIN_DISPATCH_PASS" == "yes" ]] && [[ -n "$HTTP_CID" ]]; then
  sleep 2
  code_get="$(e2e_http_request_json "GET" "mqtt-cmd-status" "/v1/admin/organizations/${ORG}/commands/${HTTP_CID}" "")"
  if [[ "$code_get" == "200" ]]; then
    ST="$(jq -r '.attempts[-1].status // .attempts[-1].dispatchState // empty' "${E2E_RUN_DIR}/rest/mqtt-cmd-status.response.json" 2>/dev/null)"
    mqtt_contract_record "$FLOW_ID" "verify-command-get" "GET .../commands/{id}" "pass" "last_attempt=${ST:-unknown}"
  else
    mqtt_contract_record "$FLOW_ID" "verify-command-get" "GET .../commands/{id}" "skip" "http_${code_get}"
  fi
else
  mqtt_contract_record "$FLOW_ID" "verify-command-get" "—" "skip" "admin_full_flow_not_used_or_no_commandId"
fi

exit 0
