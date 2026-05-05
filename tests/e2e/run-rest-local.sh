#!/usr/bin/env bash
# shellcheck shell=bash
# REST / Postman-oriented local checks (scenarios optional).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-rest-local.sh [options]

Options:
  --fresh-data        Empty test-data.json in this run dir
  --reuse-data PATH   Seed test-data.json from capture file
  -h, --help          Show help

Uses BASE_URL and ADMIN_TOKEN from tests/e2e/.env (see .env.example).
EOF
}

e2e_capture_inherited_data_flags
e2e_parse_common_args "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

require_cmd jq curl

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
  e2e_skip "REST scenarios not implemented (${SCENARIO} missing); import OpenAPI/Postman separately"
  end_step skipped "REST placeholder"
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
exit 0
