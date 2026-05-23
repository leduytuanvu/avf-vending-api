#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT flow executor via mosquitto_pub — separate evidence section (not Postman).

prod_e2e_mqtt_execute_flow() {
  local flow_json="$1"
  local id label topic evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  label="$(echo "$flow_json" | jq -r '.label')"
  topic="$(echo "$flow_json" | jq -r '.topic')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  topic="$(prod_e2e_render_template_string "$topic")"

  if [[ "${PROD_E2E_DRY_RUN:-}" == "1" ]]; then
    prod_e2e_evidence_append_row "$id" "$label" "mqtt" "dry-run" "$evidence_label"
    return 0
  fi

  if ! command -v mosquitto_pub >/dev/null 2>&1; then
    prod_e2e_evidence_append_row "$id" "$label" "mqtt" "skip-no-mosquitto" "$evidence_label"
    return 0
  fi

  local payload
  payload="$(echo "$flow_json" | jq -c '.publish_template // {}')"
  payload="$(prod_e2e_render_template_string "$payload")"
  local pub_file="${PROD_E2E_RAW_DIR}/${evidence_label}.publish.json"
  local log_file="${PROD_E2E_RAW_DIR}/${evidence_label}.mqtt.log"
  printf '%s\n' "$payload" >"$pub_file"

  local -a pub_args=(-h "${MQTT_HOST:-}" -p "${MQTT_PORT:-8883}" -t "$topic" -m "$payload" -q 1)
  if [[ -n "${MQTT_USERNAME:-}" ]]; then
    pub_args+=(-u "${MQTT_USERNAME}" -P "${MQTT_PASSWORD:-}")
  fi
  if [[ "${MQTT_USE_TLS:-false}" == "true" && -n "${MQTT_CAFILE:-}" ]]; then
    pub_args+=(--cafile "${MQTT_CAFILE}")
  fi

  set +e
  mosquitto_pub "${pub_args[@]}" >"$log_file" 2>&1
  local rc=$?
  set -e

  local status="pass"
  [[ $rc -eq 0 ]] || status="fail"
  prod_e2e_evidence_append_row "$id" "$label" "mqtt" "$status" "$evidence_label"
  prod_e2e_evidence_append_mqtt_section "$id" "$evidence_label" "$topic"
  [[ "$status" == "fail" ]] && return 1
  return 0
}
