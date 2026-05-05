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
      events:$events
    }' >"${out}"
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
  echo "=================================" >&2
}

e2e_finalize_reports() {
  local exit_code="${1:-0}"
  export E2E_REPORT_DONE=1
  e2e_generate_summary_md
  e2e_generate_remediation_md
  e2e_generate_coverage_json
  e2e_print_console_summary "${exit_code}"
}
