#!/usr/bin/env bash
# shellcheck shell=bash
# Reporting: summary.md, remediation.md, coverage.json, console summary.

e2e_events_path() {
  echo "${E2E_RUN_DIR}/events.jsonl"
}

e2e_stats_json() {
  local ev="${E2E_RUN_DIR}/events.jsonl"
  if [[ ! -f "$ev" ]] || [[ ! -s "$ev" ]]; then
    jq -nc '{total:0,passed:0,failed:0,skipped:0}'
    return 0
  fi
  jq -s '{
      total: length,
      passed: map(select(.status=="passed")) | length,
      failed: map(select(.status=="failed")) | length,
      skipped: map(select(.status=="skipped")) | length
    }' "${ev}"
}

e2e_python_run() {
  if command -v python3 >/dev/null 2>&1; then
    python3 "$@"
    return $?
  fi
  if command -v py >/dev/null 2>&1; then
    py -3 "$@"
    return $?
  fi
  return 127
}

e2e_write_report_context_json() {
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  mkdir -p "${E2E_RUN_DIR}/reports"
  local mqtt_b="${MQTT_HOST:-127.0.0.1}:${MQTT_PORT:-1883}"
  local prod_json=false
  [[ -n "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" ]] && prod_json=true
  jq -nc \
    --arg base "${BASE_URL:-}" \
    --arg grpc "${GRPC_ADDR:-}" \
    --arg mqtt "$mqtt_b" \
    --arg writes "${E2E_ALLOW_WRITES:-}" \
    --argjson prod "$prod_json" \
    --arg reuse "${E2E_REUSE_DATA:-false}" \
    --arg dfile "${E2E_DATA_FILE:-}" \
    '{
      baseUrl: $base,
      grpcAddr: $grpc,
      mqttBroker: $mqtt,
      allowWrites: $writes,
      productionConfirmationSet: $prod,
      reuseData: $reuse,
      reuseDataSource: $dfile
    }' \
    >"${E2E_RUN_DIR}/reports/e2e-report-context.json"
}

e2e_generate_summary_md() {
  local out="${E2E_RUN_DIR}/reports/summary.md"
  mkdir -p "${E2E_RUN_DIR}/reports"
  local stats
  stats="$(e2e_stats_json)"
  local -a metas=()
  local f
  if [[ -d "${E2E_RUN_DIR}/rest" ]]; then
    mapfile -t metas < <(find "${E2E_RUN_DIR}/rest" -maxdepth 1 -name '*.meta.json' -print 2>/dev/null | LC_ALL=C sort)
  fi
  {
    echo "# E2E run summary"
    echo
    echo "- **Generated:** $(now_utc)"
    echo "- **Run directory:** \`${E2E_RUN_DIR}\`"
    echo "- **Target:** ${E2E_TARGET:-local}"
    echo "- **Test data (public):** \`${E2E_RUN_DIR}/test-data.json\`"
    echo "- **Secrets (local only):** \`${E2E_RUN_DIR}/secrets.private.json\`"
    echo
    echo "## Step counts"
    echo
    echo "\`\`\`json"
    echo "$stats" | jq .
    echo "\`\`\`"
    echo
    echo "## Events"
    echo
    if [[ -f "$(e2e_events_path)" ]]; then
      echo "\`\`\`"
      cat "$(e2e_events_path)"
      echo "\`\`\`"
    else
      echo "_(no events recorded)_"
    fi
    echo
    echo "## REST endpoints exercised"
    echo
    if [[ ${#metas[@]} -eq 0 ]]; then
      echo "_(no \`rest/*.meta.json\` files — no HTTP capture this run)_"
    else
      echo "| Step | Method | Path | HTTP | ms | Result |"
      echo "|------|--------|------|------|-----|--------|"
      for f in "${metas[@]}"; do
        if jq -e '.path' "$f" >/dev/null 2>&1; then
          jq -r '"| `\(.step)` | \(.method) | `\(.path)` | \(.httpStatus) | \(.elapsedMs) | \(.result) |"' "$f"
        fi
      done
      echo
      echo "Per-call artifacts live under \`${E2E_RUN_DIR}/rest/\`: \`*.request.json\`, \`*.response.body\`, \`*.response.headers.txt\`, \`*.meta.json\`."
    fi
  } >"${out}"
}

e2e_generate_remediation_md() {
  local out="${E2E_RUN_DIR}/reports/remediation.md"
  mkdir -p "${E2E_RUN_DIR}/reports"
  {
    echo "# E2E remediation hints"
    echo
    echo "Derived from **failed** events in this run. See \`docs/testing/e2e-remediation-playbook.md\` for full procedures."
    echo
    echo "Run directory: \`${E2E_RUN_DIR}\`"
    echo
  } >"${out}"

  local ev="${E2E_RUN_DIR}/events.jsonl"
  if [[ ! -f "$ev" ]]; then
    echo "_No events file — nothing to remediate._" >>"${out}"
    return 0
  fi

  local any_failed
  any_failed="$(jq -e 'select(.status=="failed")' "${ev}" >/dev/null 2>&1 && echo yes || echo no)"
  if [[ "$any_failed" == "no" ]]; then
    echo "**No failed steps.**" >>"${out}"
    return 0
  fi

  echo "## Failed steps" >>"${out}"
  echo >>"${out}"
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    local step msg
    step="$(echo "$line" | jq -r '.step')"
    msg="$(echo "$line" | jq -r '.message')"
    echo "- **${step}:** ${msg}" >>"${out}"
  done < <(jq -c 'select(.status=="failed")' "${ev}" 2>/dev/null || true)
}

e2e_generate_coverage_json() {
  local out="${E2E_RUN_DIR}/reports/coverage.json"
  mkdir -p "${E2E_RUN_DIR}/reports"
  local stats
  stats="$(e2e_stats_json)"
  local events='[]'
  if [[ -f "$(e2e_events_path)" ]] && [[ -s "$(e2e_events_path)" ]]; then
    events="$(jq -s '.' "$(e2e_events_path)")"
  fi
  jq -nc \
    --arg gen "$(now_utc)" \
    --arg dir "$E2E_RUN_DIR" \
    --arg data "${E2E_RUN_DIR}/test-data.json" \
    --arg evfile "$(e2e_events_path)" \
    --argjson stats "$stats" \
    --argjson events "$events" \
    '{
      generatedAt:$gen,
      runDir:$dir,
      testDataFile:$data,
      eventsFile:$evfile,
      counts:$stats,
      events:$events,
      coverage: { note: "minimal fallback — run merge-events.py for full merge" }
    }' >"${out}"
}

e2e_merge_flow_review_coverage_fragments() {
  local cov="${E2E_RUN_DIR}/reports/coverage.json"
  [[ -f "$cov" ]] || return 0
  local st="${E2E_RUN_DIR}/reports/flow-review-static.json"
  local lv="${E2E_RUN_DIR}/reports/flow-review-live.json"
  [[ -f "$st" || -f "$lv" ]] || return 0
  local st_json lv_json
  if [[ -f "$st" ]] && jq empty "$st" >/dev/null 2>&1; then
    st_json="$(cat "$st")"
  else
    st_json='{}'
  fi
  if [[ -f "$lv" ]] && jq empty "$lv" >/dev/null 2>&1; then
    lv_json="$(cat "$lv")"
  else
    lv_json='{}'
  fi
  local tmp
  tmp="$(mktemp)"
  if jq --argjson s "$st_json" --argjson l "$lv_json" '.flowReview={static:$s, live:$l}' "$cov" >"$tmp" 2>/dev/null; then
    mv "$tmp" "$cov"
  else
    rm -f "$tmp"
    log_warn "e2e_merge_flow_review_coverage_fragments: jq merge failed"
  fi
}

e2e_print_console_summary() {
  local exit_code="${1:-0}"
  local stats
  stats="$(e2e_stats_json)"
  local total passed failed skipped
  total="$(echo "$stats" | jq -r '.total')"
  passed="$(echo "$stats" | jq -r '.passed')"
  failed="$(echo "$stats" | jq -r '.failed')"
  skipped="$(echo "$stats" | jq -r '.skipped')"
  echo "" >&2
  echo "========== E2E summary ==========" >&2
  echo "Exit code:        ${exit_code}" >&2
  echo "Total events:    ${total}" >&2
  echo "Passed:          ${passed}" >&2
  echo "Failed:          ${failed}" >&2
  echo "Skipped:         ${skipped}" >&2
  echo "Run directory:   ${E2E_RUN_DIR}" >&2
  echo "Test data file:  ${E2E_RUN_DIR}/test-data.json" >&2
  echo "Reports:         ${E2E_RUN_DIR}/reports/" >&2
  if [[ -s "${E2E_RUN_DIR}/improvement-findings.jsonl" ]] 2>/dev/null; then
    local _fc _p0 _p1
    _fc="$(wc -l <"${E2E_RUN_DIR}/improvement-findings.jsonl" | tr -d ' ')"
    _p0="$(jq -s '[.[] | select(.severity=="P0")] | length' "${E2E_RUN_DIR}/improvement-findings.jsonl" 2>/dev/null || echo 0)"
    _p1="$(jq -s '[.[] | select(.severity=="P1")] | length' "${E2E_RUN_DIR}/improvement-findings.jsonl" 2>/dev/null || echo 0)"
    echo "Flow review:     improvement-findings.jsonl (${_fc} row(s); P0=${_p0} P1=${_p1}) → improvement-summary.md" >&2
  else
    echo "Flow review:     (no improvement rows)" >&2
  fi
  if [[ "${exit_code}" -ne 0 ]] || [[ "${failed}" != "0" ]]; then
    echo "On failure: open reports/remediation.md and reports/summary.md under the run directory above." >&2
  fi
  echo "=================================" >&2
}

e2e_finalize_reports() {
  local exit_code="${1:-0}"
  export E2E_REPORT_DONE=1

  if [[ -n "${E2E_RUN_DIR:-}" ]]; then
    mkdir -p "${E2E_RUN_DIR}/reports"
    : >>"${E2E_RUN_DIR}/improvement-findings.jsonl"
  fi

  local _lib _tools _repo
  _lib="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  _tools="$(cd "${_lib}/../tools" && pwd)"
  _repo="$(cd "${_lib}/../../.." && pwd)"

  e2e_write_report_context_json

  if e2e_python_run "${_tools}/merge-events.py" --run-dir "${E2E_RUN_DIR}"; then
    :
  else
    log_warn "merge-events.py failed — continuing with partial reports"
    e2e_generate_coverage_json
  fi

  if ! e2e_python_run "${_tools}/generate-summary.py" --run-dir "${E2E_RUN_DIR}" --repo-root "${_repo}"; then
    log_warn "generate-summary.py failed — bash summary fallback"
  fi
  if ! e2e_python_run "${_tools}/generate-remediation.py" --run-dir "${E2E_RUN_DIR}" --playbook "${_repo}/docs/testing/e2e-remediation-playbook.md"; then
    log_warn "generate-remediation.py failed — bash remediation fallback"
  fi

  [[ -f "${E2E_RUN_DIR}/reports/summary.md" ]] || e2e_generate_summary_md
  [[ -f "${E2E_RUN_DIR}/reports/remediation.md" ]] || e2e_generate_remediation_md
  [[ -f "${E2E_RUN_DIR}/reports/coverage.json" ]] || e2e_generate_coverage_json

  e2e_merge_flow_review_coverage_fragments

  if ! e2e_python_run "${_tools}/generate-improvement-summary.py" --run-dir "${E2E_RUN_DIR}" --repo-root "${_repo}"; then
    log_warn "generate-improvement-summary.py failed"
  fi
  if ! e2e_python_run "${_tools}/generate-optimization-backlog.py" --run-dir "${E2E_RUN_DIR}"; then
    log_warn "generate-optimization-backlog.py failed"
  fi
  if ! e2e_python_run "${_tools}/generate-flow-scorecard.py" --run-dir "${E2E_RUN_DIR}" --repo-root "${_repo}"; then
    log_warn "generate-flow-scorecard.py failed"
  fi

  if [[ -s "${E2E_RUN_DIR}/improvement-findings.jsonl" ]]; then
    {
      echo ""
      echo "## Related flow improvement findings"
      echo ""
      echo "Non-blocking debt (and configured P0/P1 gates) lives in **\`improvement-findings.jsonl\`**, **\`improvement-summary.md\`**, **\`optimization-backlog.md\`**. See **\`docs/testing/e2e-remediation-playbook.md\`**."
      echo ""
    } >>"${E2E_RUN_DIR}/reports/remediation.md"
  fi

  # Mirrors at run root (reports/ remains canonical; copies for CI / triage globs)
  for f in summary.md remediation.md coverage.json improvement-summary.md optimization-backlog.md flow-review-scorecard.json; do
    if [[ -f "${E2E_RUN_DIR}/reports/${f}" ]]; then
      cp -f "${E2E_RUN_DIR}/reports/${f}" "${E2E_RUN_DIR}/${f}"
    fi
  done
  if [[ -f "${E2E_RUN_DIR}/reports/e2e-junit.xml" ]]; then
    cp -f "${E2E_RUN_DIR}/reports/e2e-junit.xml" "${E2E_RUN_DIR}/junit.xml"
  fi

  local review_ec=0
  e2e_flow_review_exit_gate || review_ec=1
  local combined="${exit_code}"
  if [[ "${review_ec}" -ne 0 ]]; then
    combined=1
  fi

  e2e_print_console_summary "${combined}"
  return "${combined}"
}
