#!/usr/bin/env bash
# shellcheck shell=bash
# Flow improvement logger: non-fatal findings → improvement-findings.jsonl
# Requires e2e_common.sh sourced first (now_utc, jq, E2E_RUN_DIR, log_*)

e2e_improvement_findings_path() {
  echo "${E2E_RUN_DIR}/improvement-findings.jsonl"
}

e2e_new_finding_id() {
  printf 'IMPR-%s-%05d' "$(date -u +%Y%m%dT%H%M%S)" "${RANDOM}"
}

# Append one finding (all string fields via --arg).
# Args: finding_id severity category flow_id scenario_id step_name protocol endpoint symptom impact recommendation evidence_file [status]
append_improvement_finding() {
  [[ "${E2E_ENABLE_FLOW_REVIEW:-true}" == "true" ]] || return 0
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  local finding_id="$1" severity="$2" category="$3" flow_id="$4" scenario_id="$5" step_name="$6" protocol="$7" endpoint_or_rpc_or_topic="$8" symptom="$9" impact="${10}" recommendation="${11}" evidence_file="${12}" status="${13:-open}"
  local out
  out="$(e2e_improvement_findings_path)"
  mkdir -p "$(dirname "$out")"
  local ts
  ts="$(now_utc)"
  jq -nc \
    --arg ts "$ts" \
    --arg finding_id "$finding_id" \
    --arg severity "$severity" \
    --arg category "$category" \
    --arg flow_if "$flow_id" \
    --arg scenario_id "$scenario_id" \
    --arg step_name "$step_name" \
    --arg protocol "$protocol" \
    --arg endpoint_or_rpc_or_topic "$endpoint_or_rpc_or_topic" \
    --arg symptom "$symptom" \
    --arg impact "$impact" \
    --arg recommendation "$recommendation" \
    --arg evidence_file "$evidence_file" \
    --arg status "$status" \
    '{
      ts:$ts,
      finding_id:$finding_id,
      severity:$severity,
      category:$category,
      flow_id:$flow_if,
      scenario_id:$scenario_id,
      step_name:$step_name,
      protocol:$protocol,
      endpoint_or_rpc_or_topic:$endpoint_or_rpc_or_topic,
      symptom:$symptom,
      impact:$impact,
      recommendation:$recommendation,
      evidence_file:$evidence_file,
      status:$status
    }' >>"${out}" 2>/dev/null || log_warn "append_improvement_finding: jq append failed"
}

# --- Wrappers: severity flow_id scenario_id step_name protocol endpoint symptom impact recommendation evidence_file ---

log_flow_design_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "flow_design" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_api_contract_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "api_contract" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_response_shape_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "response_shape" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_request_shape_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "request_shape" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_missing_endpoint_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "missing_endpoint" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_missing_field_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "missing_field" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_idempotency_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "idempotency" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_retry_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "retry_semantics" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_offline_sync_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "offline_sync" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_performance_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "performance" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_flaky_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "flaky" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_docs_gap() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "docs_gap" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_postman_gap() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "postman_gap" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_security_safety_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "production_safety" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_data_setup_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "test_data_gap" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_protocol_mismatch() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "protocol_mismatch" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_rest_grpc_mismatch() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "rest_grpc_mismatch" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_mqtt_contract_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "mqtt_contract" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_unnecessary_complexity_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "unnecessary_complexity" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_observability_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "observability" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_security_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "security" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_cleanup_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "cleanup" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_unknown_improvement() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "unknown" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

# Explicit marker: scenario reviewed the flow; no additional rows added on this path.
# Args: flow_id scenario_basename step_name [message]
e2e_flow_review_scenario_complete() {
  [[ "${E2E_ENABLE_FLOW_REVIEW:-true}" == "true" ]] || return 0
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  local flow_id="${1:?}" scen="${2:?}" step="${3:-flow-review-complete}" msg="${4:-no_improvement_findings_logged_this_scenario}"
  if declare -F e2e_append_test_event >/dev/null 2>&1; then
    e2e_append_test_event "$flow_id" "$step" "mixed" "e2e-flow-review" "pass" "$msg" "$(jq -nc --arg sid "$scen" '{scenario_id:$sid, flow_review_no_findings:true}')"
  elif declare -F append_event_jsonl >/dev/null 2>&1; then
    append_event_jsonl "flow-review:${scen}" "passed" "$msg"
  fi
}

# Slow HTTP: bump counter, log P3 perf; third hit in run → P2 flaky.
e2e_flow_review_note_http_slow() {
  [[ "${E2E_ENABLE_FLOW_REVIEW:-true}" == "true" ]] || return 0
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  local step="$1" ms="$2" path="$3" method="${4:-GET}"
  local warn="${E2E_WARN_SLOW_MS:-1500}"
  [[ "$ms" =~ ^[0-9]+$ ]] || return 0
  ((ms < warn)) && return 0
  mkdir -p "${E2E_RUN_DIR}/reports"
  local safe="${step//\//_}"
  local cf="${E2E_RUN_DIR}/reports/.slow_${safe}"
  local c=0
  [[ -f "$cf" ]] && read -r c <"$cf" || true
  [[ -z "${c// /}" ]] && c=0
  c=$((c + 1))
  echo "$c" >"$cf"
  log_performance_issue "P3" "http-slow" "http-${step}" "${step}" "REST" "${method} ${path}" \
    "HTTP ${method} ${path} took ${ms}ms (warn ${warn}ms)" \
    "Slower automation and operator feedback" \
    "Profile API; batch or cache; document SLOs" \
    "${E2E_RUN_DIR}/rest/${step}.meta.json"
  if [[ "$c" -ge 3 ]]; then
    log_flaky_issue "P2" "http-slow-repeat" "http-${step}" "${step}" "REST" "${method} ${path}" \
      "Step exceeded slow threshold ${c} times in one run" \
      "Timing volatility" \
      "Stabilize env or split assertions" \
      "${E2E_RUN_DIR}/rest/${step}.meta.json"
  fi
}

e2e_flow_review_exit_gate() {
  [[ "${E2E_ENABLE_FLOW_REVIEW:-true}" == "true" ]] || return 0
  local f
  f="$(e2e_improvement_findings_path)"
  [[ -f "$f" ]] && [[ -s "$f" ]] || return 0
  local p0 p1
  p0="$(jq -s '[.[] | select(.severity == "P0")] | length' "$f" 2>/dev/null || echo 0)"
  p1="$(jq -s '[.[] | select(.severity == "P1")] | length' "$f" 2>/dev/null || echo 0)"
  if [[ "${E2E_FAIL_ON_P0_FINDINGS:-true}" == "true" ]] && [[ "${p0:-0}" -gt 0 ]]; then
    log_error "Flow review: ${p0} P0 improvement finding(s) — see improvement-summary.md / optimization-backlog.md"
    return 1
  fi
  if [[ "${E2E_FAIL_ON_P1_FINDINGS:-false}" == "true" ]] && [[ "${p1:-0}" -gt 0 ]]; then
    log_error "Flow review: ${p1} P1 improvement finding(s) — see improvement-summary.md"
    return 1
  fi
  return 0
}
