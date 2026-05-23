#!/usr/bin/env bash
# shellcheck shell=bash
# Run id and E2E-PROD-{run_id} prefix helpers.

prod_e2e_ids_init() {
  : "${PROD_E2E_REPO_ROOT:?set PROD_E2E_REPO_ROOT}"
  if [[ -z "${PROD_E2E_RUN_ID:-}" ]]; then
    PROD_E2E_RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$-${RANDOM}"
  fi
  export PROD_E2E_RUN_ID
  export PROD_E2E_PREFIX="E2E-PROD-${PROD_E2E_RUN_ID}"
  export run_id="${PROD_E2E_RUN_ID}"
  export run_prefix="${PROD_E2E_PREFIX}"

  PROD_E2E_RUN_DIR="${PROD_E2E_RUN_DIR:-${PROD_E2E_REPO_ROOT}/.e2e-runs/production/${PROD_E2E_RUN_ID}}"
  export PROD_E2E_RUN_DIR
  PROD_E2E_RAW_DIR="${PROD_E2E_RUN_DIR}/raw"
  export PROD_E2E_RAW_DIR
  PROD_E2E_STATE_FILE="${PROD_E2E_RUN_DIR}/state.env"
  PROD_E2E_STATE_JSON="${PROD_E2E_RUN_DIR}/state.json"
  export PROD_E2E_STATE_FILE PROD_E2E_STATE_JSON
  mkdir -p "${PROD_E2E_RAW_DIR}"
  touch "${PROD_E2E_STATE_FILE}"

  if [[ -f "${PROD_E2E_STATE_FILE}" ]]; then
    # shellcheck disable=SC1090
    set -a
    while IFS= read -r line || [[ -n "$line" ]]; do
      [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] || continue
      eval "export ${line}"
    done <"${PROD_E2E_STATE_FILE}"
    set +a
  fi
}

prod_e2e_render_template_string() {
  local s="$1"
  local key val
  s="${s//\{\{run_id\}\}/${PROD_E2E_RUN_ID}}"
  s="${s//\{\{run_prefix\}\}/${PROD_E2E_PREFIX}}"
  s="${s//\{\{admin_email\}\}/${ADMIN_EMAIL:-}}"
  s="${s//\{\{admin_email_invalid_test\}\}/${ADMIN_EMAIL:-e2e-invalid@invalid.local}}"
  s="${s//\{\{admin_password\}\}/${ADMIN_PASSWORD:-}}"
  s="${s//\{\{media_sha256\}\}/${PROD_E2E_MEDIA_SHA256:-c414cd0e204de974f73753c7e28d7638e7b3691bb8b1a2bab6b25bb7fed7ce77}}"
  s="${s//\{\{mqtt_topic_prefix\}\}/${MQTT_TOPIC_PREFIX:-avf/prod}}"
  s="${s//\{\{webhook_event_id\}\}/${webhook_event_id:-${PROD_E2E_PREFIX}-wh}}"
  s="${s//\{\{operator_session_id\}\}/${operatorSessionId:-}}"
  s="${s//\{\{planogram_id\}\}/${planogramId:-}}"
  s="${s//\{\{planogram_revision\}\}/${planogramRevision:-0}}"
  s="${s//\{\{machineRefreshToken\}\}/${machineRefreshToken:-}}"
  s="${s//\{\{catalogVersion\}\}/${catalogVersion:-0}}"
  s="${s//\{\{mediaFingerprint\}\}/${mediaFingerprint:-initial}}"
  s="${s//\{\{currency\}\}/${currency:-VND}}"
  s="${s//\{\{slotIndex\}\}/${slotIndex:-1}}"
  if [[ -f "${PROD_E2E_STATE_FILE}" ]]; then
    while IFS='=' read -r key val; do
      [[ -n "$key" ]] || continue
      val="${val#\'}"; val="${val%\'}"
      s="${s//\{\{${key}\}\}/${val}}"
    done < <(grep -E '^[A-Za-z_][A-Za-z0-9_]*=' "${PROD_E2E_STATE_FILE}" 2>/dev/null || true)
  fi
  printf '%s' "$s"
}

prod_e2e_render_json_template() {
  local inline_or_file="$1"
  local raw
  if [[ -f "$inline_or_file" ]]; then
    raw="$(cat "$inline_or_file")"
  else
    raw="$inline_or_file"
  fi
  prod_e2e_render_template_string "$raw"
}

prod_e2e_is_e2e_resource_name() {
  local name="$1"
  [[ "$name" == E2E-PROD-* ]]
}
