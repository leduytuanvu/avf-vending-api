#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 6: machine gRPC contract flows (grpcurl + repo protos or reflection).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-grpc-local.sh [options]

Machine app production-path RPCs (see scenarios/20_grpc_*.sh … 24_grpc_*.sh).

Environment (tests/e2e/.env):
  GRPC_ADDR                 host:port (default 127.0.0.1:9090)
  GRPC_USE_REFLECTION       true: grpcurl uses server reflection (list services).
  GRPC_PROTO_ROOT           Import root for .proto files when reflection is false.
                            Defaults to \`\$REPO_ROOT/proto\` if that contains \`avf/machine/v1\`.
  MACHINE_TOKEN             Bearer for authenticated machine RPCs (or claim in 20).
  MACHINE_ID                Optional; also sent as \`x-machine-id: ...\` when set.
  GRPC_SEND_MACHINE_ID_HEADER  true/false (default true).
  E2E_ACTIVATION_CODE        Used by 20 when no MACHINE_TOKEN in secrets.

  --fresh-data / --reuse-data PATH / -h

Artifacts: .e2e-runs/run-*/grpc/*.request.json, *.response.json, *.log, *.meta.json
           reports/grpc-contract-results.jsonl, reports/grpc-contract-summary.md
EOF
}

e2e_capture_inherited_data_flags
e2e_parse_common_args "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

: "${GRPC_PROTO_ROOT:=${E2E_REPO_ROOT}/proto}"
export GRPC_PROTO_ROOT GRPC_ADDR GRPC_USE_REFLECTION MACHINE_TOKEN MACHINE_ID GRPC_SEND_MACHINE_ID_HEADER

require_cmd jq bash

new_run_dir
e2e_write_run_meta "run-grpc-local"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  e2e_data_initialize
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] || cleanup_trap_register

start_step "grpc-local-suite"
ec=0

if ! command -v grpcurl >/dev/null 2>&1; then
  e2e_skip "grpcurl not installed — install grpcurl for Phase 6 machine contract tests"
  end_step skipped "grpcurl missing"
  [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
  exit 0
fi

# shellcheck source=lib/e2e_grpc.sh
source "${SCRIPT_DIR}/lib/e2e_grpc.sh"

if ! e2e_grpc_server_reachable; then
  log_error "gRPC server not reachable at ${GRPC_ADDR} (TCP closed or reflection list failed). Start the API with gRPC enabled; see docs/testing/e2e-troubleshooting.md#gRPC-machine-tests."
  end_step failed "gRPC unreachable at ${GRPC_ADDR}"
  ec=1
  [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit "${ec}"
  # shellcheck source=lib/e2e_report.sh
  source "${SCRIPT_DIR}/lib/e2e_report.sh"
  e2e_finalize_reports "${ec}"
  exit "${ec}"
fi

refresh_grpc_machine_env() {
  export MACHINE_ID="$(get_data machineId)"
  export MACHINE_TOKEN="$(get_secret machineToken 2>/dev/null || true)"
}

run_grpc_scenario() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    log_error "missing scenario ${path}"
    return 1
  fi
  set +e
  bash "$path"
  local c=$?
  set -e
  refresh_grpc_machine_env
  if [[ "$c" -ne 0 ]]; then
    log_error "$(basename "$path") exited ${c}"
    return "$c"
  fi
  return 0
}

grpc_scenarios=(
  "${SCRIPT_DIR}/scenarios/20_grpc_machine_auth.sh"
  "${SCRIPT_DIR}/scenarios/21_grpc_bootstrap_catalog_media.sh"
  "${SCRIPT_DIR}/scenarios/22_grpc_commerce_cash_sale.sh"
  "${SCRIPT_DIR}/scenarios/23_grpc_inventory_telemetry_offline.sh"
  "${SCRIPT_DIR}/scenarios/24_grpc_command_update_status.sh"
)

refresh_grpc_machine_env

for scen in "${grpc_scenarios[@]}"; do
  if ! run_grpc_scenario "$scen"; then
    ec=1
  fi
done

if [[ "$ec" -eq 0 ]]; then
  end_step passed "gRPC machine contract scenarios completed"
else
  end_step failed "gRPC contract scenarios exit ${ec}"
fi

grpc_write_contract_summary() {
  local out="${E2E_RUN_DIR}/reports/grpc-contract-summary.md"
  local jl="${E2E_RUN_DIR}/reports/grpc-contract-results.jsonl"
  mkdir -p "${E2E_RUN_DIR}/reports"
  {
    echo "# gRPC machine contract summary"
    echo
    echo "Generated: $(now_utc)"
    echo
    if [[ ! -f "$jl" ]] || [[ ! -s "$jl" ]]; then
      echo "_(no grpc-contract-results.jsonl)_"
    else
      jq -s '
        {
          pass: (map(select(.status=="pass")) | length),
          fail: (map(select(.status=="fail")) | length),
          skip: (map(select(.status=="skip")) | length)
        }
      ' "$jl" | jq -r '
        "| Result | Count |",
        "|--------|-------|",
        "| pass | \(.pass) |",
        "| fail | \(.fail) |",
        "| skip | \(.skip) |"
      '
      echo
      echo "## By flow"
      echo
      jq -s -r '
        group_by(.flow_id)[]
        | ("### " + (.[0].flow_id) + "\n\n"
          + "| step | method | status | note |\n"
          + "|------|--------|--------|------|\n"
          + (map("| \(.step) | `\(.method)` | **\(.status)** | \(.message) |\n") | join("")))
      ' "$jl"
    fi
  } >"${out}"
}

grpc_write_contract_summary

if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  # shellcheck source=lib/e2e_report.sh
  source "${SCRIPT_DIR}/lib/e2e_report.sh"
  e2e_finalize_reports "${ec}"
  sm="${E2E_RUN_DIR}/reports/summary.md"
  if [[ -f "$sm" ]] && [[ -f "${E2E_RUN_DIR}/reports/grpc-contract-summary.md" ]]; then
    {
      echo ""
      echo "## gRPC machine contract (Phase 6)"
      echo ""
      echo "See \`reports/grpc-contract-summary.md\` and \`reports/grpc-contract-results.jsonl\`."
      echo ""
      cat "${E2E_RUN_DIR}/reports/grpc-contract-summary.md"
      echo ""
    } >>"$sm"
  fi
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit "${ec}"
exit "${ec}"
