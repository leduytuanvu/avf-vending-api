#!/usr/bin/env bash
# shellcheck shell=bash
# Flow improvement logger: non-fatal findings → improvement-findings.jsonl
# Requires e2e_common.sh sourced first (now_utc, jq, E2E_RUN_DIR, log_*)

e2e_improvement_findings_path() {
  echo "${E2E_RUN_DIR}/improvement-findings.jsonl"
}

# Returns 0 when flow review logging is enabled (append functions are active).
improvement_enabled() {
  [[ "${E2E_ENABLE_FLOW_REVIEW:-true}" == "true" ]]
}

e2e_new_finding_id() {
  printf 'IMPR-%s-%05d' "$(date -u +%Y%m%dT%H%M%S)" "${RANDOM}"
}

# Severity: P0 money/inventory/prod block/unsafe payment; P1 E2E/pilot/major mismatch;
# P2 complex/docs/ergonomics/manual workaround; P3 cleanup/naming.
e2e_flow_review_validate_severity() {
  case "${1:-}" in
    P0 | P1 | P2 | P3) echo "${1}" ;;
    *)
      log_warn "improvement finding: invalid severity '${1:-}', coercing to P3"
      echo "P3"
      ;;
  esac
}

e2e_flow_review_validate_category() {
  local c="${1:-}"
  case "${c}" in
    flow_design | api_contract | response_shape | missing_field | missing_endpoint | unnecessary_complexity | idempotency | retry_semantics | offline_sync | protocol_mismatch | rest_grpc_mismatch | mqtt_contract | docs_gap | postman_gap | test_data_gap | production_safety | performance | flaky | observability | cleanup | security | unknown)
      echo "${c}"
      ;;
    request_shape)
      # Align legacy helper with schema enum (request-shape debt is API contract).
      echo "api_contract"
      ;;
    *)
      log_warn "improvement finding: invalid category '${c}', using unknown"
      echo "unknown"
      ;;
  esac
}

e2e_default_suggested_owner_for_category() {
  case "$1" in
    api_contract | response_shape | missing_field | missing_endpoint | idempotency | retry_semantics | offline_sync | protocol_mismatch | rest_grpc_mismatch | mqtt_contract | production_safety | security)
      echo "backend"
      ;;
    docs_gap | postman_gap)
      echo "docs"
      ;;
    test_data_gap | flow_design | unnecessary_complexity | flaky | performance | cleanup)
      echo "qa"
      ;;
    observability)
      echo "devops"
      ;;
    unknown)
      echo "unknown"
      ;;
    *)
      echo "unknown"
      ;;
  esac
}

# Args: finding_id severity category flow_id scenario_id step_name protocol endpoint_or_rpc_or_topic
#       symptom impact recommendation evidence_file [suggested_owner] [status]
append_improvement_finding() {
  improvement_enabled || return 0
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  local finding_id="$1" severity="$2" category="$3" flow_id="$4" scenario_id="$5" step_name="$6" protocol="$7" endpoint_or_rpc_or_topic="$8" symptom="$9" impact="${10}" recommendation="${11}" evidence_file="${12}" suggested_owner="${13:-}" status="${14:-open}"

  severity="$(e2e_flow_review_validate_severity "${severity}")"
  category="$(e2e_flow_review_validate_category "${category}")"
  [[ -z "${suggested_owner// /}" ]] && suggested_owner="$(e2e_default_suggested_owner_for_category "${category}")"

  local out created_at_utc
  out="$(e2e_improvement_findings_path)"
  mkdir -p "$(dirname "${out}")"
  created_at_utc="$(now_utc)"

  case "${status}" in
    open | acknowledged | closed) ;;
    *)
      log_warn "improvement finding: invalid status '${status}', using open"
      status="open"
      ;;
  esac

  jq -nc \
    --arg finding_id "${finding_id}" \
    --arg created_at_utc "${created_at_utc}" \
    --arg severity "${severity}" \
    --arg category "${category}" \
    --arg flow_id "${flow_id}" \
    --arg scenario_id "${scenario_id}" \
    --arg step_name "${step_name}" \
    --arg protocol "${protocol}" \
    --arg endpoint_or_rpc_or_topic "${endpoint_or_rpc_or_topic}" \
    --arg symptom "${symptom}" \
    --arg impact "${impact}" \
    --arg recommendation "${recommendation}" \
    --arg evidence_file "${evidence_file}" \
    --arg suggested_owner "${suggested_owner}" \
    --arg status "${status}" \
    '{
      finding_id:$finding_id,
      created_at_utc:$created_at_utc,
      severity:$severity,
      category:$category,
      flow_id:$flow_id,
      scenario_id:$scenario_id,
      step_name:$step_name,
      protocol:$protocol,
      endpoint_or_rpc_or_topic:$endpoint_or_rpc_or_topic,
      symptom:$symptom,
      impact:$impact,
      recommendation:$recommendation,
      evidence_file:$evidence_file,
      suggested_owner:$suggested_owner,
      status:$status
    }' >>"${out}" 2>/dev/null || log_warn "append_improvement_finding: jq append failed"
}

# --- Wrappers: severity flow_id scenario_id step_name protocol endpoint symptom impact recommendation evidence_file ---

_append_imp() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}" "${11}" "" "open"
}

log_flow_design_issue() {
  _append_imp "$1" "flow_design" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_api_contract_issue() {
  _append_imp "$1" "api_contract" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_response_shape_issue() {
  _append_imp "$1" "response_shape" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_request_shape_issue() {
  append_improvement_finding "$(e2e_new_finding_id)" "$1" "api_contract" "$2" "$3" "$4" "$5" "$6" "[request shape] $7" "$8" "$9" "${10}" "" "open"
}

log_missing_endpoint_issue() {
  _append_imp "$1" "missing_endpoint" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_missing_field_issue() {
  _append_imp "$1" "missing_field" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_idempotency_issue() {
  _append_imp "$1" "idempotency" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_retry_issue() {
  _append_imp "$1" "retry_semantics" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_offline_sync_issue() {
  _append_imp "$1" "offline_sync" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_performance_issue() {
  _append_imp "$1" "performance" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_flaky_issue() {
  _append_imp "$1" "flaky" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_docs_gap() {
  _append_imp "$1" "docs_gap" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

# Alias: satisfy scenario lint that requires `log_*_issue` / append_improvement_finding naming.
log_docs_issue() {
  log_docs_gap "$@"
}

log_postman_gap() {
  _append_imp "$1" "postman_gap" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_test_data_gap() {
  _append_imp "$1" "test_data_gap" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_data_setup_issue() {
  log_test_data_gap "$@"
}

log_production_safety_issue() {
  _append_imp "$1" "production_safety" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_security_safety_issue() {
  log_production_safety_issue "$@"
}

log_protocol_mismatch() {
  _append_imp "$1" "protocol_mismatch" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_rest_grpc_mismatch() {
  _append_imp "$1" "rest_grpc_mismatch" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_rest_grpc_issue() {
  log_rest_grpc_mismatch "$@"
}

log_protocol_issue() {
  log_protocol_mismatch "$@"
}

log_mqtt_contract_issue() {
  _append_imp "$1" "mqtt_contract" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_unnecessary_complexity_issue() {
  _append_imp "$1" "unnecessary_complexity" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_observability_issue() {
  _append_imp "$1" "observability" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_security_issue() {
  _append_imp "$1" "security" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_cleanup_issue() {
  _append_imp "$1" "cleanup" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

log_unknown_improvement() {
  _append_imp "$1" "unknown" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" "${10}"
}

# Audit row when a scope explicitly had nothing to log (aggregated under flow_id _e2e_review_marker).
# Prefer calling this from scenarios when you want a durable jsonl record; e2e_flow_review_scenario_complete only writes test-events.
# Args: flow_id scenario_id [step_name]
log_no_improvement_findings() {
  improvement_enabled || return 0
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  local flow_id="${1:?flow_id required}" scenario_id="${2:?scenario_id required}" step_name="${3:-flow-review-complete}"
  local ev="${E2E_RUN_DIR}/test-events.jsonl"
  [[ -f "${ev}" ]] || ev="${E2E_RUN_DIR}/reports/summary.md"
  append_improvement_finding \
    "E2E-NO-FINDINGS-$(printf '%05d' "${RANDOM}")" \
    "P3" \
    "unknown" \
    "_e2e_review_marker" \
    "${scenario_id}" \
    "${step_name}" \
    "mixed" \
    "n/a" \
    "No actionable improvement findings logged for covered flow ${flow_id}." \
    "Explicit clear marker for triage; not treated as product debt. See flow-review-scorecard flow _e2e_review_marker." \
    "Omit this helper if you prefer an empty jsonl for silent passes." \
    "${ev}" \
    "qa" \
    "closed"
}

# Explicit marker: scenario reviewed the flow (test-events only). For a JSONL audit row, call log_no_improvement_findings.
# Args: flow_id scenario_basename step_name [message]
e2e_flow_review_scenario_complete() {
  improvement_enabled || return 0
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  local flow_id="${1:?}" scen="${2:?}" step="${3:-flow-review-complete}" msg="${4:-no_improvement_findings_logged_this_scenario}"
  if declare -F e2e_append_test_event >/dev/null 2>&1; then
    e2e_append_test_event "${flow_id}" "${step}" "mixed" "e2e-flow-review" "pass" "${msg}" "$(jq -nc --arg sid "${scen}" '{scenario_id:$sid, flow_review_no_findings:true}')"
  elif declare -F append_event_jsonl >/dev/null 2>&1; then
    append_event_jsonl "flow-review:${scen}" "passed" "${msg}"
  fi
}

# Slow HTTP: bump counter, log P3 perf; third hit in run → P2 flaky.
e2e_flow_review_note_http_slow() {
  improvement_enabled || return 0
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  local step="$1" ms="$2" path="$3" method="${4:-GET}"
  local warn="${E2E_WARN_SLOW_MS:-1500}"
  [[ "${ms}" =~ ^[0-9]+$ ]] || return 0
  ((ms < warn)) && return 0
  mkdir -p "${E2E_RUN_DIR}/reports"
  local safe="${step//\//_}"
  local cf="${E2E_RUN_DIR}/reports/.slow_${safe}"
  local c=0
  [[ -f "${cf}" ]] && read -r c <"${cf}" || true
  [[ -z "${c// /}" ]] && c=0
  c=$((c + 1))
  echo "${c}" >"${cf}"
  log_performance_issue "P3" "http-slow" "http-${step}" "${step}" "REST" "${method} ${path}" \
    "HTTP ${method} ${path} took ${ms}ms (warn ${warn}ms)" \
    "Slower automation and operator feedback" \
    "Profile API; batch or cache; document SLOs" \
    "${E2E_RUN_DIR}/rest/${step}.meta.json"
  if [[ "${c}" -ge 3 ]]; then
    log_flaky_issue "P2" "http-slow-repeat" "http-${step}" "${step}" "REST" "${method} ${path}" \
      "Step exceeded slow threshold ${c} times in one run" \
      "Timing volatility" \
      "Stabilize env or split assertions" \
      "${E2E_RUN_DIR}/rest/${step}.meta.json"
  fi
}

e2e_flow_review_exit_gate() {
  improvement_enabled || return 0
  local f
  f="$(e2e_improvement_findings_path)"
  [[ -f "${f}" ]] && [[ -s "${f}" ]] || return 0
  local p0 p1
  p0="$(jq -s '[.[] | select(.severity == "P0" and ((.finding_id // "") | test("^E2E-NO-FINDINGS") | not))] | length' "${f}" 2>/dev/null || echo 0)"
  p1="$(jq -s '[.[] | select(.severity == "P1" and ((.finding_id // "") | test("^E2E-NO-FINDINGS") | not))] | length' "${f}" 2>/dev/null || echo 0)"
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
