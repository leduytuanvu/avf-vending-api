#!/usr/bin/env bash
# shellcheck shell=bash
# Web admin: Phase 3 setup (01) + optional Phase 4 business flows (10–13).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-web-admin-flows.sh [options]

  Modes:
    --setup-only   Run only Phase 3 setup (WA-SETUP-01) — default
    --full         Run setup then catalog / inventory / support / reporting flows (10–13)

  Data (same as other E2E runners):
    --fresh-data              Empty run test-data.json before setup
    --reuse-data PATH         Copy captured JSON (e.g. Phase 3 test-data.json) into this run

  Phase 4 expects organizationId, machineId, productId (and optional slot/planogram fields)
  from test-data.json — typically produced by setup in the same run or via --reuse-data.

  Environment: tests/e2e/.env (see .env.example)
    E2E_ALLOW_WRITES, BASE_URL, ADMIN_TOKEN or login + E2E_ORGANIZATION_ID, E2E_TARGET, etc.

  Artifacts: .e2e-runs/run-*/rest/*.request.json|*.response.json|*.meta.json,
    test-events.jsonl, reports/wa-module-results.jsonl (Phase 4 detail),
    reports/summary.md, reports/remediation.md
EOF
}

# Strip --setup-only / --full before common parser (unknown args land in E2E_EXTRA_ARGS).
argv=()
WA_MODE="setup-only"
for arg in "$@"; do
  case "$arg" in
    --setup-only) WA_MODE="setup-only" ;;
    --full) WA_MODE="full" ;;
    *) argv+=("$arg") ;;
  esac
done

e2e_capture_inherited_data_flags
e2e_parse_common_args "${argv[@]}"
load_env
e2e_restore_inherited_data_flags_if_needed

export BASE_URL GRPC_ADDR E2E_TARGET E2E_ALLOW_WRITES E2E_PRODUCTION_WRITE_CONFIRMATION
export E2E_REUSE_DATA E2E_DATA_FILE E2E_CLI_FRESH_DATA
export ADMIN_TOKEN ADMIN_EMAIL ADMIN_PASSWORD E2E_ORGANIZATION_ID
export E2E_SEED_FILE E2E_WEB_ADMIN_SKIP_BRAND E2E_WEB_ADMIN_SKIP_TAG

require_cmd jq curl
e2e_require_python

new_run_dir
e2e_write_run_meta "run-web-admin-flows"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  e2e_data_initialize
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] || cleanup_trap_register

start_step "web-admin-flows"
ec=0
SETUP="${SCRIPT_DIR}/scenarios/01_web_admin_setup.sh"

run_bash_scenario() {
  local label="$1"
  local path="$2"
  if [[ ! -f "$path" ]]; then
    e2e_skip "${label} missing (${path})"
    return 0
  fi
  set +e
  bash "$path"
  local c=$?
  set -e
  if [[ "$c" -ne 0 ]]; then
    log_error "${label} exited ${c}"
    return "$c"
  fi
  return 0
}

if [[ -f "$SETUP" ]]; then
  if ! run_bash_scenario "web-admin-setup" "$SETUP"; then
    ec=1
  fi
else
  e2e_skip "web admin setup not implemented (${SETUP} missing)"
  ec=1
fi

if [[ "$ec" -eq 0 ]] && [[ "$WA_MODE" == "full" ]]; then
  phase4_scenarios=(
    "${SCRIPT_DIR}/scenarios/12_web_admin_catalog_ops.sh"
    "${SCRIPT_DIR}/scenarios/11_web_admin_inventory_ops.sh"
    "${SCRIPT_DIR}/scenarios/13_web_admin_support_ops.sh"
    "${SCRIPT_DIR}/scenarios/10_reporting_audit_reconciliation.sh"
  )
  for scen in "${phase4_scenarios[@]}"; do
    if ! run_bash_scenario "$(basename "$scen")" "$scen"; then
      ec=1
    fi
  done
fi

if [[ "$ec" -eq 0 ]]; then
  end_step passed "web admin flows completed (mode=${WA_MODE})"
else
  end_step failed "web admin flows exit ${ec} (mode=${WA_MODE})"
fi

wa4_append_reports() {
  local sm="$1"
  local rmf="$2"
  local wl="${E2E_RUN_DIR}/reports/wa-module-results.jsonl"
  [[ -f "$wl" ]] && [[ -s "$wl" ]] || return 0

  {
    echo ""
    echo "## Web admin flow results by module"
    echo ""
    echo "Structured JSONL: \`reports/wa-module-results.jsonl\` (expected vs actual, file paths)."
    echo ""
    jq -s -r '
      group_by(.module)[]
      | (
          "## Module: " + (.[0].module) + "\n\n"
          + "| step | endpoint | status | HTTP | expected | actual |\n"
          + "|------|----------|--------|------|----------|--------|\n"
          + (map("| \(.step) | `\(.endpoint)` | **\(.status)** | \(.httpStatus) | \(.expected) | \(.actual) |\n") | join(""))
        )
    ' "$wl"
    echo ""
  } >>"$sm"

  {
    echo ""
    echo "## Web admin business flows — failed checks"
    echo ""
    any=0
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      st="$(echo "$line" | jq -r '.status')"
      [[ "$st" != "fail" ]] && continue
      any=1
      ep="$(echo "$line" | jq -r '.endpoint')"
      step="$(echo "$line" | jq -r '.step')"
      http="$(echo "$line" | jq -r '.httpStatus')"
      rem="$(echo "$line" | jq -r '.remediation')"
      rsp="$(echo "$line" | jq -r '.responsePath')"
      echo "- **\`${ep}\`** (\`${step}\`) → HTTP ${http}. **Likely fix:** ${rem} Artifact: \`${rsp}\`."
    done <"$wl"
    [[ "$any" -eq 0 ]] && echo "_No failing Phase 4 rows (or only setup failed — see events above)._"
    echo ""
  } >>"$rmf"
}

if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  # shellcheck source=lib/e2e_report.sh
  source "${SCRIPT_DIR}/lib/e2e_report.sh"
  e2e_finalize_reports "${ec}"
  fr=$?
  [[ "${fr}" -ne 0 ]] && ec="${fr}"
  sm="${E2E_RUN_DIR}/reports/summary.md"
  rmf="${E2E_RUN_DIR}/reports/remediation.md"
  if [[ -f "$sm" ]]; then
    {
      echo ""
      echo "## Web admin setup steps (WA-SETUP-01)"
      echo ""
      echo "Automated by \`scenarios/01_web_admin_setup.sh\`. See \`test-events.jsonl\`."
      echo ""
    } >>"$sm"
    wa4_append_reports "$sm" "$rmf"
  fi
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit "${ec}"
exit "${ec}"
