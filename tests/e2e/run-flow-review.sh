#!/usr/bin/env bash
# shellcheck shell=bash
# Non-destructive flow review: static repo analysis and/or read-only probes with reused test-data.
# Never enables writes — safe for E2E_TARGET=production when only public/machine GETs are used.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export E2E_SCRIPT_DIR="${SCRIPT_DIR}"

# Refuse writes for this entrypoint (load_env defaults ALLOW_WRITES true — keep pre-set false).
export E2E_ALLOW_WRITES=false
export E2E_FLOW_REVIEW_RUNNER=1

# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"

e2e_flow_review_parse_args() {
  E2E_FR_MODE=""
  E2E_FR_REUSE_PATH=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --static-only)
        E2E_FR_MODE="static"
        shift
        ;;
      --reuse-data)
        [[ $# -ge 2 ]] || {
          echo "FATAL: --reuse-data requires a path to test-data.json" >&2
          exit 2
        }
        E2E_FR_MODE="reuse"
        E2E_FR_REUSE_PATH="$2"
        shift 2
        ;;
      -h | --help)
        e2e_flow_review_print_help
        exit 0
        ;;
      *)
        echo "FATAL: unknown argument: $1" >&2
        e2e_flow_review_print_help >&2
        exit 2
        ;;
    esac
  done
}

e2e_flow_review_print_help() {
  cat <<EOF
Usage:
  ./tests/e2e/run-flow-review.sh --static-only
  ./tests/e2e/run-flow-review.sh --reuse-data path/to/test-data.json

Non-destructive flow review only:
  - E2E_ALLOW_WRITES stays false (no POST/PUT/PATCH/DELETE from this harness).
  - Safe to aim at production for read-only checks when credentials allow.

Options:
  --static-only       Repo-only checks (Postman, matrix, protos, scenarios, MQTT docs).
  --reuse-data PATH   Same static pass, then read-only REST/gRPC checks using captured JSON.
  -h, --help          This help.

Environment: see tests/e2e/.env.example (BASE_URL, GRPC_ADDR, ADMIN_TOKEN optional,
  MACHINE_TOKEN / secrets.private.json for machine-scoped GETs).

Artifacts: .e2e-runs/run-*/reports/{summary.md,coverage.json,improvement-summary.md,
  optimization-backlog.md,flow-review-scorecard.json,...}
EOF
}

e2e_flow_review_parse_args "$@"

if [[ -z "${E2E_FR_MODE:-}" ]]; then
  echo "FATAL: specify --static-only or --reuse-data PATH" >&2
  e2e_flow_review_print_help >&2
  exit 2
fi

e2e_strict_mode
load_env

export E2E_ALLOW_WRITES=false

require_cmd jq curl
e2e_require_python

new_run_dir
e2e_write_run_meta "run-flow-review"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
# shellcheck source=lib/e2e_report.sh
source "${SCRIPT_DIR}/lib/e2e_report.sh"

if [[ "${E2E_FR_MODE}" == "reuse" ]]; then
  E2E_REUSE_DATA="true"
  E2E_DATA_FILE="${E2E_FR_REUSE_PATH}"
  export E2E_REUSE_DATA E2E_DATA_FILE
fi
e2e_data_initialize

cleanup_trap_register

log_info "flow-review: mode=${E2E_FR_MODE} E2E_TARGET=${E2E_TARGET:-local} ALLOW_WRITES=${E2E_ALLOW_WRITES} RUN_DIR=${E2E_RUN_DIR}"

if [[ "${E2E_FR_MODE}" == "static" ]]; then
  bash "${SCRIPT_DIR}/scenarios/90_flow_review_static.sh"
else
  bash "${SCRIPT_DIR}/scenarios/90_flow_review_static.sh"
  bash "${SCRIPT_DIR}/scenarios/91_flow_review_existing_data.sh"
fi

e2e_finalize_reports 0
