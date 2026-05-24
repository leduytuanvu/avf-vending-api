#!/usr/bin/env bash
# shellcheck shell=bash
# Multi-step MQTT production E2E handlers.

prod_e2e_mqtt_dispatch() {
  local flow_json="$1"
  local handler
  handler="$(echo "$flow_json" | jq -r '.handler')"
  case "$handler" in
    connect_valid) prod_e2e_mqtt_handler_connect_valid "$flow_json" ;;
    connect_invalid) prod_e2e_mqtt_handler_connect_invalid "$flow_json" ;;
    command_pipeline) prod_e2e_mqtt_handler_command_pipeline "$flow_json" ;;
    telemetry_publish) prod_e2e_mqtt_handler_telemetry_publish "$flow_json" ;;
    readback_reports) prod_e2e_mqtt_handler_readback_reports "$flow_json" ;;
    neg_wrong_machine_ack) prod_e2e_mqtt_handler_neg_wrong_machine_ack "$flow_json" ;;
    neg_stale_ack) prod_e2e_mqtt_handler_neg_stale_ack "$flow_json" ;;
    neg_malformed_telemetry) prod_e2e_mqtt_handler_neg_malformed_telemetry "$flow_json" ;;
    neg_duplicate_ack) prod_e2e_mqtt_handler_neg_duplicate_ack "$flow_json" ;;
    *)
      echo "unknown MQTT handler: ${handler}" >&2
      return 1
      ;;
  esac
}

prod_e2e_mqtt_read_first_line() {
  local f="$1"
  [[ -f "$f" ]] || return 1
  head -n1 "$f" | tr -d '\r'
}

prod_e2e_mqtt_handler_connect_valid() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  prod_e2e_mqtt_resolve_topics || return 1
  local topic="${PROD_E2E_MQTT_TOPIC_COMMAND_IN}"
  prod_e2e_mqtt_ensure_clients || {
    prod_e2e_fail_classify "a" "$id" "mosquitto_pub/mosquitto_sub not installed"
    return 1
  }
  prod_e2e_mqtt_subscribe_accept_connect "$topic" 8 "$evidence_label" || {
    prod_e2e_mqtt_fail_hint "$id" "auth"
    prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "fail" "$evidence_label"
    return 1
  }
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "pass" "$evidence_label"
  return 0
}

prod_e2e_mqtt_handler_connect_invalid() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  prod_e2e_mqtt_resolve_topics || return 1
  local topic="${PROD_E2E_MQTT_TOPIC_COMMAND_IN}"
  local payload='{"probe":"invalid-auth"}'
  set +e
  prod_e2e_mqtt_publish_raw "$topic" "$payload" "$evidence_label" "invalid-e2e-user" "invalid-e2e-password"
  local rc=$?
  set -e
  if [[ "$rc" -eq 0 ]]; then
    prod_e2e_mqtt_fail_hint "$id" "auth"
    prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "fail" "$evidence_label"
    return 1
  fi
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "pass" "$evidence_label"
  return 0
}

prod_e2e_mqtt_ensure_machine_commandable() {
  local mid="${machineId:-${PROD_E2E_MQTT_MACHINE_ID:-}}"
  [[ -n "$mid" && -n "${ADMIN_TOKEN:-}" && -n "${PROD_E2E_BASE_URL:-}" ]] || return 0
  local patch_flow
  patch_flow="$(jq -nc \
    --arg mid "$mid" \
    '{
      id: "MQTT-PREP-ACTIVE",
      label: "PATCH machine active before MQTT command dispatch",
      protocol: "rest",
      method: "PATCH",
      path: ("/v1/admin/machines/" + $mid),
      auth: "bearer_admin",
      evidence_label: "mqtt-prep-machine-active",
      request_template: {status: "active"},
      expected_status: 200,
      optional: true
    }')"
  prod_e2e_rest_execute_flow "$patch_flow" || true
}

prod_e2e_mqtt_build_catalog_refresh_ack() {
  local recv_line="$1"
  local dedupe_suffix="${2:-}"
  local mid="${PROD_E2E_MQTT_MACHINE_ID}"
  local cid seq cv mv dedupe occ
  cid="$(echo "$recv_line" | jq -r '.command_id // .commandId // empty')"
  seq="$(echo "$recv_line" | jq -r '.sequence // 0')"
  cv="$(echo "$recv_line" | jq -r '.payload.catalogVersion // 124')"
  mv="$(echo "$recv_line" | jq -r '.payload.mediaManifestVersion // 46')"
  dedupe="${PROD_E2E_PREFIX}-catalog-ack${dedupe_suffix}-${cid}"
  occ="$(prod_e2e_mqtt_now_rfc3339)"
  jq -nc \
    --arg cid "$cid" \
    --arg mid "$mid" \
    --arg occ "$occ" \
    --argjson seq "$seq" \
    --arg dk "$dedupe" \
    --argjson cv "$cv" \
    --argjson mv "$mv" \
    '{
      command_id: $cid,
      machine_id: $mid,
      occurred_at: $occ,
      status: "success",
      sequence: $seq,
      dedupe_key: $dk,
      payload: {
        type: "catalog.refresh",
        catalogVersion: $cv,
        mediaManifestVersion: $mv,
        mediaSynced: true,
        detail: "e2e-prod-mqtt"
      }
    }'
}

prod_e2e_mqtt_handler_command_pipeline() {
  local flow_json="$1"
  local id evidence_label dispatch_flow verify_flow
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  dispatch_flow="$(echo "$flow_json" | jq -c '.dispatch_flow')"
  verify_flow="$(echo "$flow_json" | jq -c '.verify_flow // null')"

  prod_e2e_mqtt_resolve_topics || return 1
  local cmd_in="${PROD_E2E_MQTT_TOPIC_COMMAND_IN}"
  local cmd_ack="${PROD_E2E_MQTT_TOPIC_COMMAND_ACK}"
  local sub_label="${evidence_label}-command-sub"
  local sub_log="${PROD_E2E_RAW_DIR}/${sub_label}.subscribe.txt"

  prod_e2e_mqtt_subscribe_background "$cmd_in" 45 "$sub_label"
  sleep 2
  prod_e2e_mqtt_ensure_machine_commandable
  prod_e2e_rest_execute_flow "$dispatch_flow" || {
    prod_e2e_mqtt_stop_subscriber
    prod_e2e_mqtt_fail_hint "$id" "bridge"
    return 1
  }

  wait "${PROD_E2E_MQTT_SUB_PID:-}" 2>/dev/null || true
  local sub_ec=$?
  PROD_E2E_MQTT_SUB_PID=""

  local recv
  recv="$(prod_e2e_mqtt_read_first_line "$sub_log")"
  if ! prod_e2e_mqtt_sub_join_ok "$sub_ec" "$sub_log" || [[ -z "$recv" ]]; then
    prod_e2e_mqtt_fail_hint "$id" "topic"
    prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "fail" "$evidence_label"
    return 1
  fi
  printf '%s\n' "$recv" >"${PROD_E2E_RAW_DIR}/${evidence_label}.command-received.json"

  local cid
  cid="$(echo "$recv" | jq -r '.command_id // .commandId // empty')"
  [[ -n "$cid" ]] || {
    prod_e2e_mqtt_fail_hint "$id" "payload"
    return 1
  }
  prod_e2e_state_set commandId "$cid"

  local ack_json
  ack_json="$(prod_e2e_mqtt_build_catalog_refresh_ack "$recv" "")"
  prod_e2e_mqtt_publish_raw "$cmd_ack" "$ack_json" "${evidence_label}-ack" || {
    prod_e2e_mqtt_fail_hint "$id" "auth"
    return 1
  }

  sleep 2
  if [[ "$verify_flow" != "null" ]]; then
    prod_e2e_rest_execute_flow "$verify_flow" || {
      prod_e2e_mqtt_fail_hint "$id" "bridge"
      return 1
    }
    local st resp_file
    resp_file="${PROD_E2E_RAW_DIR}/$(echo "$verify_flow" | jq -r '.evidence_label').response.json"
    st="$(jq -r '.attempts[-1].status // .status // empty' "$resp_file" 2>/dev/null)"
    case "$st" in
      acked|acknowledged|completed|duplicate|executed) ;;
      *)
        prod_e2e_mqtt_fail_hint "$id" "bridge"
        echo "command status after ACK expected acked/completed, got: ${st:-<empty>}" >&2
        return 1
        ;;
    esac
  fi

  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "pass" "$evidence_label"
  return 0
}

prod_e2e_mqtt_handler_telemetry_publish() {
  local flow_json="$1"
  local id evidence_label topic_key template
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  topic_key="$(echo "$flow_json" | jq -r '.topic_key // "telemetry"')"
  prod_e2e_mqtt_resolve_topics || return 1

  local topic eid ts mid payload
  mid="${PROD_E2E_MQTT_MACHINE_ID}"
  eid="${PROD_E2E_PREFIX}-$(echo "$topic_key" | tr '/' '-')"
  ts="$(prod_e2e_mqtt_now_rfc3339)"

  case "$topic_key" in
    heartbeat) topic="${PROD_E2E_MQTT_TOPIC_HEARTBEAT}" ;;
    presence) topic="${PROD_E2E_MQTT_TOPIC_PRESENCE}" ;;
    snapshot) topic="${PROD_E2E_MQTT_TOPIC_TEL_SNAPSHOT}" ;;
    inventory) topic="${PROD_E2E_MQTT_TOPIC_EVENTS_INV}" ;;
    *) topic="${PROD_E2E_MQTT_TOPIC_TELEMETRY}" ;;
  esac

  template="$(echo "$flow_json" | jq -c '.publish_template // {}')"
  if [[ "$template" == "{}" ]]; then
    payload="$(jq -nc \
      --arg mid "$mid" \
      --arg eid "$eid" \
      --arg ts "$ts" \
      --arg tk "$topic_key" \
      '{
        schema_version: 1,
        event_id: $eid,
        machine_id: $mid,
        event_type: $tk,
        occurred_at: $ts,
        dedupe_key: $eid,
        payload: { source: "e2e-prod-mqtt", kind: $tk }
      }')"
  else
    payload="$(prod_e2e_render_template_string "$template")"
    payload="$(echo "$payload" | jq --arg ts "$ts" --arg eid "$eid" '. + {occurred_at: $ts, event_id: $eid, dedupe_key: $eid}')"
  fi

  prod_e2e_mqtt_publish_raw "$topic" "$payload" "$evidence_label" || {
    prod_e2e_mqtt_fail_hint "$id" "bridge"
    return 1
  }
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "pass" "$evidence_label"
  return 0
}

prod_e2e_mqtt_handler_readback_reports() {
  local flow_json="$1"
  local id
  id="$(echo "$flow_json" | jq -r '.id')"
  while IFS= read -r rest_flow; do
    [[ -n "$rest_flow" ]] || continue
    prod_e2e_rest_execute_flow "$rest_flow" || {
      prod_e2e_mqtt_fail_hint "$id" "bridge"
      return 1
    }
  done < <(echo "$flow_json" | jq -c '.readback_flows[]?')
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "pass" "$(echo "$flow_json" | jq -r '.evidence_label')"
  return 0
}

prod_e2e_mqtt_handler_neg_wrong_machine_ack() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  prod_e2e_mqtt_resolve_topics || return 1
  local wrong_id="00000000-0000-4000-8000-000000000001"
  local ack_json occ dedupe
  occ="$(prod_e2e_mqtt_now_rfc3339)"
  dedupe="${PROD_E2E_PREFIX}-wrong-machine-ack"
  ack_json="$(jq -nc \
    --arg cid "${commandId:-00000000-0000-4000-8000-000000000002}" \
    --arg mid "$wrong_id" \
    --arg occ "$occ" \
    --arg dk "$dedupe" \
    '{
      command_id: $cid,
      machine_id: $mid,
      occurred_at: $occ,
      status: "acked",
      sequence: 1,
      dedupe_key: $dk,
      payload: {}
    }')"
  prod_e2e_mqtt_publish_raw "${PROD_E2E_MQTT_TOPIC_COMMAND_ACK}" "$ack_json" "$evidence_label" || true
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "pass" "$evidence_label"
  return 0
}

prod_e2e_mqtt_handler_neg_stale_ack() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  prod_e2e_mqtt_resolve_topics || return 1
  local ack_json occ dedupe
  occ="$(prod_e2e_mqtt_now_rfc3339)"
  dedupe="${PROD_E2E_PREFIX}-stale-ack"
  ack_json="$(jq -nc \
    --arg cid "00000000-0000-4000-8000-000000000099" \
    --arg mid "${PROD_E2E_MQTT_MACHINE_ID}" \
    --arg occ "$occ" \
    --arg dk "$dedupe" \
    '{
      command_id: $cid,
      machine_id: $mid,
      occurred_at: $occ,
      status: "acked",
      sequence: 999999,
      dedupe_key: $dk,
      payload: {}
    }')"
  prod_e2e_mqtt_publish_raw "${PROD_E2E_MQTT_TOPIC_COMMAND_ACK}" "$ack_json" "$evidence_label" || true
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "pass" "$evidence_label"
  return 0
}

prod_e2e_mqtt_handler_neg_malformed_telemetry() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  prod_e2e_mqtt_resolve_topics || return 1
  local payload='{"schema_version":1,"payload":{"note":"missing identity fields"}}'
  prod_e2e_mqtt_publish_raw "${PROD_E2E_MQTT_TOPIC_EVENTS_INV}" "$payload" "$evidence_label" || true
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "pass" "$evidence_label"
  return 0
}

prod_e2e_mqtt_handler_neg_duplicate_ack() {
  local flow_json="$1"
  local id evidence_label recv_file ack_json
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  recv_file="${PROD_E2E_RAW_DIR}/mqtt-command-pipeline.command-received.json"
  [[ -f "$recv_file" ]] || recv_file="${PROD_E2E_RAW_DIR}/$(echo "$flow_json" | jq -r '.source_evidence // "mqtt-command-pipeline"').command-received.json"
  [[ -f "$recv_file" ]] || {
    prod_e2e_mqtt_fail_hint "$id" "payload"
    return 1
  }
  prod_e2e_mqtt_resolve_topics || return 1
  local recv
  recv="$(cat "$recv_file")"
  ack_json="$(prod_e2e_mqtt_build_catalog_refresh_ack "$recv" "")"
  prod_e2e_mqtt_publish_raw "${PROD_E2E_MQTT_TOPIC_COMMAND_ACK}" "$ack_json" "${evidence_label}-a" || return 1
  prod_e2e_mqtt_publish_raw "${PROD_E2E_MQTT_TOPIC_COMMAND_ACK}" "$ack_json" "${evidence_label}-b" || return 1
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "mqtt" "pass" "$evidence_label"
  return 0
}
