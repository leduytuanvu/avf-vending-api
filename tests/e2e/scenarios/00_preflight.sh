#!/usr/bin/env bash
# shellcheck shell=bash
# Preflight: toolchain, required env, and core health/version HTTP checks.

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"

require_cmd bash curl jq
e2e_require_python
append_event_jsonl "preflight:tooling" "passed" "bash curl jq python3"

if ! command -v mosquitto_pub >/dev/null 2>&1 || ! command -v mosquitto_sub >/dev/null 2>&1; then
  log_error "mosquitto_pub and mosquitto_sub are required on PATH for local E2E (MQTT phase + scenario E2E-45). Install: winget install --id EclipseFoundation.Mosquitto -e"
  append_event_jsonl "preflight:mqtt-clients" "failed" "mosquitto clients missing"
  exit 1
fi
append_event_jsonl "preflight:mqtt-clients" "passed" "mosquitto_pub mosquitto_sub"

for _o in newman grpcurl; do
  if command -v "${_o}" >/dev/null 2>&1; then
    append_event_jsonl "preflight:opt:${_o}" "passed" "on PATH"
    log_info "optional tool present: ${_o}"
  else
    append_event_jsonl "preflight:opt:${_o}" "skipped" "not on PATH"
    log_warn "optional tool not on PATH: ${_o} (later phases may skip)"
  fi
done

require_env BASE_URL E2E_TARGET E2E_ALLOW_WRITES
append_event_jsonl "preflight:env" "passed" "BASE_URL E2E_TARGET E2E_ALLOW_WRITES"
log_info "BASE_URL=${BASE_URL} E2E_TARGET=${E2E_TARGET} E2E_ALLOW_WRITES=${E2E_ALLOW_WRITES}"

for path in "/health/live" "/health/ready" "/version"; do
  step="preflight-$(echo "${path#/}" | tr '/' '-')"
  if ! e2e_http_get_capture "$step" "$path" "required" "false"; then
    fail_step "preflight HTTP failed: ${path}"
    exit 1
  fi
done

append_event_jsonl "preflight:api" "passed" "health/live health/ready version"

FLOW_ID="PF-PREFLIGHT"
log_no_improvement_findings "$FLOW_ID" "00_preflight.sh" "flow-review-complete"
e2e_flow_review_scenario_complete "$FLOW_ID" "00_preflight.sh" "flow-review-complete" "toolchain_and_public_health_ok_no_scenario_findings"

return 0 2>/dev/null || exit 0
