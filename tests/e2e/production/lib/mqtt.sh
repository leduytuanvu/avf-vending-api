#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT publish/subscribe via mosquitto clients — evidence under raw/

# shellcheck source=mqtt_common.sh
source "$(dirname "${BASH_SOURCE[0]}")/mqtt_common.sh"

prod_e2e_mqtt_publish_raw() {
  local topic="$1"
  local payload="$2"
  local evidence_label="$3"
  local user_override="${4:-}"
  local pass_override="${5:-}"

  local pub_file="${PROD_E2E_RAW_DIR}/${evidence_label}.publish.json"
  local log_file="${PROD_E2E_RAW_DIR}/${evidence_label}.mqtt.log"
  local meta_file="${PROD_E2E_RAW_DIR}/${evidence_label}.meta.json"

  jq -nc \
    --arg host "${MQTT_HOST:-}" \
    --argjson port "${MQTT_PORT:-8883}" \
    --arg topic "$topic" \
    --arg payload "$payload" \
    --arg tls "${MQTT_USE_TLS:-true}" \
    '{host:$host,port:$port,topic:$topic,payload:$payload,tls:$tls,auth:"credentials_redacted"}' >"$pub_file"
  prod_e2e_redact_file "$pub_file" "${pub_file}.redacted"
  mv "${pub_file}.redacted" "$pub_file"

  local -a args=()
  prod_e2e_mqtt_client_args args
  if [[ -n "$user_override" ]]; then
    args=(-h "${MQTT_HOST:-}" -p "${MQTT_PORT:-8883}" -u "$user_override" -P "$pass_override" -i "${PROD_E2E_PREFIX}-mqtt-bad-${RANDOM}")
    if [[ "${MQTT_USE_TLS:-true}" == "true" && -n "${MQTT_CAFILE:-${MQTT_CA_CERT:-}}" && -f "${MQTT_CAFILE:-${MQTT_CA_CERT:-}}" ]]; then
      args+=(--cafile "${MQTT_CAFILE:-${MQTT_CA_CERT}}")
    fi
  fi
  args+=(-t "$topic" -m "$payload" -q 1)

  local t0 t1 elapsed rc
  t0="$(prod_e2e_py -c 'import time; print(time.time())')"
  set +e
  mosquitto_pub "${args[@]}" >"$log_file" 2>&1
  rc=$?
  set -e
  t1="$(prod_e2e_py -c 'import time; print(time.time())')"
  elapsed="$(prod_e2e_py -c "print(int((${t1} - ${t0}) * 1000))")"
  prod_e2e_mqtt_redact_log "$log_file" "${log_file}.redacted"
  mv "${log_file}.redacted" "$log_file"
  rc="$(prod_e2e_mqtt_normalize_exit "$rc" "$log_file"; echo $?)"

  jq -nc \
    --arg topic "$topic" \
    --arg label "$evidence_label" \
    --argjson exit_code "$rc" \
    --argjson elapsed_ms "$elapsed" \
    '{direction:"publish",topic:$topic,evidence_label:$label,exit_code:$exit_code,elapsed_ms:$elapsed_ms,qos:1,retain:false}' >"$meta_file"
  PROD_E2E_MQTT_LAST_RC=$rc
  return "$rc"
}

prod_e2e_mqtt_subscribe_once() {
  local topic="$1"
  local timeout_sec="$2"
  local evidence_label="$3"

  local sub_file="${PROD_E2E_RAW_DIR}/${evidence_label}.subscribe.txt"
  local log_file="${PROD_E2E_RAW_DIR}/${evidence_label}.mqtt.log"
  local meta_file="${PROD_E2E_RAW_DIR}/${evidence_label}.meta.json"

  local -a args=()
  prod_e2e_mqtt_client_args args
  args+=(-t "$topic" -C 1 -W "$timeout_sec" -q 1)

  local t0 t1 elapsed rc
  t0="$(prod_e2e_py -c 'import time; print(time.time())')"
  set +e
  mosquitto_sub "${args[@]}" >"$sub_file" 2>"$log_file"
  rc=$?
  set -e
  t1="$(prod_e2e_py -c 'import time; print(time.time())')"
  elapsed="$(prod_e2e_py -c "print(int((${t1} - ${t0}) * 1000))")"
  prod_e2e_mqtt_redact_log "$log_file" "${log_file}.redacted"
  mv "${log_file}.redacted" "$log_file"

  jq -nc \
    --arg topic "$topic" \
    --arg label "$evidence_label" \
    --argjson exit_code "$rc" \
    --argjson timeout "$timeout_sec" \
    --argjson elapsed_ms "$elapsed" \
    '{direction:"subscribe",topic:$topic,evidence_label:$label,exit_code:$exit_code,timeout_sec:$timeout,elapsed_ms:$elapsed_ms,qos:1}' >"$meta_file"
  PROD_E2E_MQTT_LAST_RC=$rc
  PROD_E2E_MQTT_LAST_SUB_FILE="$sub_file"
  export PROD_E2E_MQTT_LAST_RC PROD_E2E_MQTT_LAST_SUB_FILE
  return "$rc"
}

prod_e2e_mqtt_subscribe_background() {
  local topic="$1"
  local timeout_sec="$2"
  local evidence_label="$3"
  local sub_file="${PROD_E2E_RAW_DIR}/${evidence_label}.subscribe.txt"
  local log_file="${PROD_E2E_RAW_DIR}/${evidence_label}.mqtt.log"
  local -a args=()
  prod_e2e_mqtt_client_args args
  args+=(-t "$topic" -C 1 -W "$timeout_sec" -q 1)
  : >"$sub_file"
  mosquitto_sub "${args[@]}" >"$sub_file" 2>"$log_file" &
  PROD_E2E_MQTT_SUB_PID=$!
  export PROD_E2E_MQTT_SUB_PID
  PROD_E2E_MQTT_LAST_SUB_FILE="$sub_file"
  export PROD_E2E_MQTT_LAST_SUB_FILE
}

prod_e2e_mqtt_stop_subscriber() {
  [[ -n "${PROD_E2E_MQTT_SUB_PID:-}" ]] && kill "${PROD_E2E_MQTT_SUB_PID}" 2>/dev/null || true
  wait "${PROD_E2E_MQTT_SUB_PID:-}" 2>/dev/null || true
  PROD_E2E_MQTT_SUB_PID=""
}

prod_e2e_mqtt_execute_flow() {
  local flow_json="$1"
  local id label topic evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  label="$(echo "$flow_json" | jq -r '.label')"
  topic="$(echo "$flow_json" | jq -r '.topic // empty')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"

  if [[ "${PROD_E2E_DRY_RUN:-}" == "1" ]]; then
    prod_e2e_evidence_append_row "$id" "$label" "mqtt" "dry-run" "$evidence_label"
    return 0
  fi

  if ! command -v mosquitto_pub >/dev/null 2>&1 || ! command -v mosquitto_sub >/dev/null 2>&1; then
    prod_e2e_mqtt_ensure_clients || true
  fi
  if ! command -v mosquitto_pub >/dev/null 2>&1 || ! command -v mosquitto_sub >/dev/null 2>&1; then
    prod_e2e_fail_classify "a" "$id" "mosquitto_pub/mosquitto_sub not installed"
    prod_e2e_evidence_append_row "$id" "$label" "mqtt" "skip-no-mosquitto" "$evidence_label"
    return 1
  fi

  prod_e2e_mqtt_resolve_topics || {
    prod_e2e_mqtt_fail_hint "$id" "topic"
    return 1
  }

  topic="$(prod_e2e_render_template_string "$topic")"
  local payload
  payload="$(echo "$flow_json" | jq -c '.publish_template // {}')"
  payload="$(prod_e2e_render_template_string "$payload")"

  if ! prod_e2e_mqtt_publish_raw "$topic" "$payload" "$evidence_label"; then
    prod_e2e_evidence_append_row "$id" "$label" "mqtt" "fail" "$evidence_label"
    prod_e2e_evidence_append_mqtt_section "$id" "$evidence_label" "$topic"
    return 1
  fi
  prod_e2e_evidence_append_row "$id" "$label" "mqtt" "pass" "$evidence_label"
  prod_e2e_evidence_append_mqtt_section "$id" "$evidence_label" "$topic"
  return 0
}

prod_e2e_mqtt_run_flow() {
  local flow_json="$1"
  local handler
  handler="$(echo "$flow_json" | jq -r '.handler // empty')"
  if [[ -n "$handler" && "$handler" != "null" ]]; then
    prod_e2e_mqtt_dispatch "$flow_json"
  else
    prod_e2e_mqtt_execute_flow "$flow_json"
  fi
}
