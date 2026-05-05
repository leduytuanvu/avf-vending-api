#!/usr/bin/env bash
# shellcheck shell=bash
# REST / Postman-oriented local checks (scenarios optional).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

E2E_REST_READONLY="false"

e2e_rest_runner_parse() {
  local -a passthrough=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --readonly)
        E2E_REST_READONLY="true"
        export E2E_REST_READONLY
        shift
        ;;
      *)
        passthrough+=("$1")
        shift
        ;;
    esac
  done
  e2e_capture_inherited_data_flags
  e2e_parse_common_args "${passthrough[@]}"
}

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-rest-local.sh [options]

Options:
  --readonly        Run read-only public GET smoke only (no writes)
  --fresh-data      Empty test-data.json in this run dir
  --reuse-data PATH Seed test-data.json from capture file
  -h, --help        Show help

Uses BASE_URL and optional ADMIN_TOKEN from tests/e2e/.env (see .env.example).

Example (read-only smoke):
  BASE_URL=http://127.0.0.1:8080 E2E_TARGET=local E2E_ALLOW_WRITES=false \\
    ./tests/e2e/run-rest-local.sh --readonly
EOF
}

e2e_rest_runner_parse "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

require_cmd jq curl python3

new_run_dir
e2e_write_run_meta "run-rest-local"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  e2e_data_initialize
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] || cleanup_trap_register

# shellcheck source=lib/e2e_http.sh
source "${SCRIPT_DIR}/lib/e2e_http.sh"

start_step "rest-local-suite"

if [[ "${E2E_REST_READONLY}" == "true" ]]; then
  set +e
  # shellcheck disable=SC1090
  source "${SCRIPT_DIR}/scenarios/00_rest_readonly_smoke.sh"
  ec_ro=$?
  set -e
  if [[ "$ec_ro" -eq 0 ]]; then
    end_step passed "REST read-only smoke completed"
  else
    end_step failed "REST read-only smoke exit ${ec_ro}"
    [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 1
    exit 1
  fi
else
  SCENARIO="${SCRIPT_DIR}/scenarios/rest_local.sh"
  if [[ -f "$SCENARIO" ]]; then
    set +e
    # shellcheck disable=SC1090
    source "$SCENARIO"
    ec=$?
    set -e
    if [[ "$ec" -eq 0 ]]; then
      end_step passed "REST scenarios completed"
    else
      end_step failed "REST scenario exit ${ec}"
      [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 1
      exit 1
    fi
  else
    rel_coll="${E2E_REPO_ROOT}/${POSTMAN_COLLECTION}"
    if [[ ! -f "$rel_coll" ]]; then
      log_warn "Postman collection path not found (set POSTMAN_COLLECTION): ${rel_coll}"
    fi
    e2e_skip "REST scenarios not implemented (${SCENARIO} missing); use --readonly for smoke or import OpenAPI/Postman"
    end_step skipped "REST placeholder"
  fi
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
exit 0
