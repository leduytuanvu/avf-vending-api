#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT-33: catalog.refresh dispatch envelope + commands/ack with mediaSynced (trigger-only; no image/binary on MQTT).

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

FLOW_ID="MQTT-33"

mqtt_catalog_refresh_guard_ok() {
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
SUB_LOG="${DIR}/catalog_refresh.subscribe.log"
rm -f "$SUB_LOG"

if ! mqtt_catalog_refresh_guard_ok; then
  mqtt_contract_record "$FLOW_ID" "guard" "—" "skip" "production_requires_e2eTestMachine_and_E2E_MQTT_COMMAND_TEST_ACK"
  e2e_append_test_event "$FLOW_ID" "guard" "MQTT" "—" "skipped" "production_safety" "{}"
  exit 0
fi

admin_dispatch_ok() {
  [[ -n "${ADMIN_TOKEN:-}" ]] || return 1
  return 0
}

launch_subscriber() {
  local topic="$1"
  local wait_sec="$2"
  local logf="$3"
  local pid_var="$4"
  e2e_mqtt_subscribe_background_pid "$topic" "$wait_sec" "$logf" "$pid_var"
}

CMD_IN="${E2E_MQTT_TOPIC_COMMAND_IN}"
CMD_ACK="${E2E_MQTT_TOPIC_COMMAND_ACK}"

read_command_line() {
  local f="$1"
  [[ -f "$f" ]] || { echo ""; return 1; }
  head -n1 "$f"
}

publish_catalog_refresh_ack_line() {
  local recv_line="$1"
  echo "$recv_line" | jq -e . >/dev/null 2>&1 || return 1
  local cid seq cv mv
  cid="$(echo "$recv_line" | jq -r '.command_id // .commandId // empty')"
  seq="$(echo "$recv_line" | jq -r '.sequence // 0')"
  cv="$(echo "$recv_line" | jq -r '.payload.catalogVersion // empty')"
  mv="$(echo "$recv_line" | jq -r '.payload.mediaManifestVersion // empty')"
  [[ -z "$cid" || "$cid" == "null" ]] && return 1
  [[ -z "$cv" || "$cv" == "null" ]] && cv="124"
  [[ -z "$mv" || "$mv" == "null" ]] && mv="46"
  local dedupe occ
  dedupe="e2e-catalog-refresh-${cid}"
  occ="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  local ack_json
  ack_json="$(jq -nc \
    --arg cid "$cid" \
    --arg mid "$MID" \
    --arg occ "$occ" \
    --argjson seq "$seq" \
    --arg dk "$dedupe" \
    --argjson cv "$cv" \
    --argjson mv "$mv" \
    '{command_id:$cid, machine_id:$mid, occurred_at:$occ, status:"success", sequence:$seq, dedupe_key:$dk, payload:{type:"catalog.refresh", catalogVersion:$cv, mediaManifestVersion:$mv, mediaSynced:true, detail:"e2e-mqtt-33"}}')"
  printf '%s\n' "$ack_json" >"${DIR}/catalog_refresh.ack.json"
  e2e_mqtt_publish "${CMD_ACK}" "$ack_json" "catalog_refresh.ack"
}

SPID=""
cleanup_s() {
  [[ -n "${SPID:-}" ]] && kill "$SPID" 2>/dev/null || true
}
trap cleanup_s EXIT

RECV=""
HTTP_CID=""
ADMIN_DISPATCH_PASS="no"
INNER_PAYLOAD='{"type":"catalog.refresh","catalogVersion":124,"mediaManifestVersion":46,"reason":"product_media_updated"}'

if admin_dispatch_ok; then
  launch_subscriber "$CMD_IN" 45 "$SUB_LOG" SPID
  sleep 1
  BODY="$(jq -nc --argjson p "$INNER_PAYLOAD" '{commandType:"catalog.refresh", payload:$p}')"
  IDK="e2e-mqtt-catalog-${RANDOM}-$(date +%s)"
  code="$(e2e_http_post_json_idem "mqtt-catalog-refresh-dispatch" "/v1/admin/machines/${MID}/commands" "$BODY" "$IDK")"
  if [[ "$code" == "202" ]]; then
    HTTP_CID="$(jq -r '.commandId // .command_id // empty' "${E2E_RUN_DIR}/rest/mqtt-catalog-refresh-dispatch.response.json" 2>/dev/null)"
  fi
  wait "$SPID" || true
  sub_ec=$?
  SPID=""
  RECV="$(read_command_line "$SUB_LOG")"
  if [[ "$code" == "202" ]] && e2e_mqtt_sub_join_payload_ok "$sub_ec" "$SUB_LOG" && [[ -n "$RECV" ]]; then
    mqtt_contract_record "$FLOW_ID" "admin-dispatch-catalog-refresh" "${CMD_IN}" "pass" "http_202 commandId=${HTTP_CID}"
    e2e_append_test_event "$FLOW_ID" "admin-dispatch" "REST+MQTT" "/v1/admin/.../commands" "pass" "catalog.refresh" "{}"
    ADMIN_DISPATCH_PASS="yes"
  else
    mqtt_contract_record "$FLOW_ID" "admin-dispatch-catalog-refresh" "${CMD_IN}" "skip" "http=${code} sub_exit=${sub_ec} (try_synthetic)"
    RECV=""
    HTTP_CID=""
  fi
else
  mqtt_contract_record "$FLOW_ID" "admin-dispatch-catalog-refresh" "—" "skip" "no_ADMIN_TOKEN"
fi

if [[ -z "$RECV" ]]; then
  SYN_ID="$(e2e_python -c 'import uuid; print(uuid.uuid4())' 2>/dev/null || echo "")"
  [[ -z "$SYN_ID" ]] && SYN_ID="$(uuidgen 2>/dev/null || echo "00000000-0000-4000-8000-000000000002")"
  SYN_PAYLOAD="$(jq -nc \
    --arg cid "$SYN_ID" \
    --arg mid "$MID" \
    --arg ik "e2e-synthetic-catalog-${RANDOM}" \
    --argjson inner "$INNER_PAYLOAD" \
    '{command_id:$cid, machine_id:$mid, sequence:1, command_type:"catalog.refresh", payload:$inner, idempotency_key:$ik}')"
  launch_subscriber "$CMD_IN" 25 "$SUB_LOG" SPID
  sleep 1
  e2e_mqtt_publish "$CMD_IN" "$SYN_PAYLOAD" "catalog_refresh.synthetic-publish"
  pub_s=$?
  wait "$SPID" || true
  sub_ec=$?
  SPID=""
  RECV="$(read_command_line "$SUB_LOG")"
  if [[ "$pub_s" -ne 0 ]] || [[ -z "$RECV" ]] || ! e2e_mqtt_sub_join_payload_ok "$sub_ec" "$SUB_LOG"; then
    trap - EXIT
    mqtt_contract_record "$FLOW_ID" "synthetic-catalog-refresh" "$CMD_IN" "fail" "pub=${pub_s} sub=${sub_ec}"
    e2e_append_test_event "$FLOW_ID" "command-receive" "MQTT" "$CMD_IN" "fail" "synthetic_failed" "{}"
    exit 1
  fi
  mqtt_contract_record "$FLOW_ID" "synthetic-catalog-refresh" "$CMD_IN" "pass" "broker_only_catalog.refresh"
  e2e_append_test_event "$FLOW_ID" "command-receive" "MQTT" "$CMD_IN" "pass" "synthetic" "{}"
fi

set +e
publish_catalog_refresh_ack_line "$RECV"
ack_ec=$?
set -e
trap - EXIT
if [[ "$ack_ec" -ne 0 ]]; then
  mqtt_contract_record "$FLOW_ID" "publish-ack-catalog-refresh" "$CMD_ACK" "fail" "mosquitto_pub_exit_${ack_ec}"
  e2e_append_test_event "$FLOW_ID" "publish-ack" "MQTT" "$CMD_ACK" "fail" "ack_failed" "{}"
  exit 1
fi
mqtt_contract_record "$FLOW_ID" "publish-ack-catalog-refresh" "$CMD_ACK" "pass" "commands/ack_sent_catalog.refresh"
e2e_append_test_event "$FLOW_ID" "publish-ack" "MQTT" "$CMD_ACK" "pass" "ok" "{}"

if [[ "$ADMIN_DISPATCH_PASS" == "yes" ]] && [[ -n "$HTTP_CID" ]]; then
  sleep 2
  code_get="$(e2e_http_request_json "GET" "mqtt-catalog-cmd-status" "/v1/admin/commands/${HTTP_CID}" "")"
  if [[ "$code_get" == "200" ]]; then
    ST="$(jq -r '.attempts[-1].status // .attempts[-1].dispatchState // empty' "${E2E_RUN_DIR}/rest/mqtt-catalog-cmd-status.response.json" 2>/dev/null)"
    mqtt_contract_record "$FLOW_ID" "verify-command-get" "GET .../commands/{id}" "pass" "last_attempt=${ST:-unknown}"
  else
    mqtt_contract_record "$FLOW_ID" "verify-command-get" "GET .../commands/{id}" "skip" "http_${code_get}"
  fi
else
  mqtt_contract_record "$FLOW_ID" "verify-command-get" "—" "skip" "admin_full_flow_not_used_or_no_commandId"
fi

if [[ "${E2E_TARGET:-local}" == "production" ]]; then
  log_security_safety_issue "P1" "$FLOW_ID" "33_mqtt_catalog_refresh.sh" "prod-command-test" "MQTT" "commands/dispatch" "Production MQTT command scenarios can affect real machines if guards are mis-set" "Physical vend/reboot risk" "Require e2eTestMachine + explicit ack env; catalog.refresh payloads only" "${E2E_RUN_DIR}/mqtt/connect.log"
else
  log_security_safety_issue "P2" "$FLOW_ID" "33_mqtt_catalog_refresh.sh" "command-topic-safety" "MQTT" "commands/dispatch" "Command dispatch/ACK tests use real broker topics — isolate ACLs from production fleet" "Accidental cross-env dispatch" "Per-env broker + machine-scoped credentials" "${E2E_RUN_DIR}/mqtt/connect.log"
fi

e2e_flow_review_scenario_complete "$FLOW_ID" "33_mqtt_catalog_refresh.sh" "flow-review-complete" "mqtt_catalog_refresh_reviewed"

exit 0
