#!/usr/bin/env bash
# shellcheck shell=bash
# Preflight: toolchain, required env, and core health/version HTTP checks.

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"

require_cmd bash curl jq python3
append_event_jsonl "preflight:tooling" "passed" "bash curl jq python3"

for _o in newman grpcurl mosquitto_pub mosquitto_sub; do
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

e2e_flow_review_scenario_complete "PF-PREFLIGHT" "00_preflight.sh" "flow-review-complete" "toolchain_and_public_health_ok_no_scenario_findings"

exit 0
