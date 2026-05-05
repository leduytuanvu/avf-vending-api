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
  --readonly        Run read-only public GET smoke only (no Newman / Postman coverage in this mode)
  --fresh-data      Empty test-data.json in this run dir
  --reuse-data PATH Seed test-data.json from capture file
  -h, --help        Show help

Uses BASE_URL and optional ADMIN_TOKEN from tests/e2e/.env (see .env.example).

When not --readonly: runs Newman (tests/e2e/postman/run-newman.sh) when present, then Postman
coverage (coverage-from-postman.py) into reports/coverage-postman.json.

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

e2e_resolve_postman_paths() {
  : "${POSTMAN_COLLECTION:=docs/postman/avf-vending-api-function-path.postman_collection.json}"
  E2E_POSTMAN_COLL_ABS="${POSTMAN_COLLECTION}"
  [[ "${E2E_POSTMAN_COLL_ABS}" != /* ]] && E2E_POSTMAN_COLL_ABS="${E2E_REPO_ROOT}/${E2E_POSTMAN_COLL_ABS}"
  E2E_POSTMAN_MATRIX_ABS="${E2E_REPO_ROOT}/docs/testing/e2e-flow-coverage.md"
}

e2e_run_postman_coverage() {
  e2e_resolve_postman_paths
  if [[ ! -f "${E2E_POSTMAN_COLL_ABS}" ]]; then
    log_warn "Postman coverage skipped: collection not found at ${E2E_POSTMAN_COLL_ABS}"
    return 0
  fi
  local cov_out="${E2E_RUN_DIR}/reports/coverage-postman.json"
  mkdir -p "${E2E_RUN_DIR}/reports"
  local -a py=(python3)
  if ! command -v python3 >/dev/null 2>&1; then
    if command -v py >/dev/null 2>&1; then
      py=(py -3)
    else
      log_warn "Postman coverage skipped: python3 not found"
      return 0
    fi
  fi
  set +e
  "${py[@]}" "${SCRIPT_DIR}/postman/coverage-from-postman.py" \
    --collection "${E2E_POSTMAN_COLL_ABS}" \
    --matrix "${E2E_POSTMAN_MATRIX_ABS}" \
    --out "${cov_out}"
  local c=$?
  set -e
  if [[ "$c" -ne 0 ]]; then
    log_error "Postman coverage gate failed (exit ${c}) — see ${cov_out} and stderr above"
    return "$c"
  fi
  log_info "Postman coverage written: ${cov_out}"
  return 0
}

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
  cov_ec=0
  ne_ec=0
  _ne_allow="${E2E_ALLOW_WRITES:-true}"
  if [[ -f "${SCRIPT_DIR}/postman/run-newman.sh" ]]; then
    set +e
    E2E_ALLOW_WRITES="${_ne_allow}" bash "${SCRIPT_DIR}/postman/run-newman.sh"
    ne_ec=$?
    set -e
  fi
  if ! e2e_run_postman_coverage; then
    cov_ec=1
  fi

  if [[ -f "$SCENARIO" ]]; then
    set +e
    # shellcheck disable=SC1090
    source "$SCENARIO"
    ec=$?
    set -e
    if [[ "$ec" -ne 0 ]]; then
      end_step failed "REST scenario exit ${ec}"
      [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 1
      exit 1
    fi
  fi

  if [[ "$ne_ec" -ne 0 ]]; then
    end_step failed "Newman or prior REST step exit ${ne_ec} (see rest/newman-cli.log)"
    [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 1
    exit 1
  fi
  if [[ "$cov_ec" -ne 0 ]]; then
    end_step failed "Postman coverage gate exit ${cov_ec}"
    [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 1
    exit 1
  fi

  e2e_resolve_postman_paths
  if [[ -f "$SCENARIO" ]]; then
    end_step passed "REST scenarios + Postman Newman/coverage completed"
  elif [[ -f "${SCRIPT_DIR}/postman/run-newman.sh" ]] || [[ -f "${E2E_POSTMAN_COLL_ABS}" ]]; then
    end_step passed "Postman Newman/coverage phase completed"
  else
    rel_coll="${E2E_REPO_ROOT}/${POSTMAN_COLLECTION}"
    if [[ ! -f "$rel_coll" ]]; then
      log_warn "Postman collection path not found (set POSTMAN_COLLECTION): ${rel_coll}"
    fi
    e2e_skip "REST scenarios not implemented (${SCENARIO} missing); add postman/run-newman.sh or collection or use --readonly"
    end_step skipped "REST placeholder"
  fi
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
exit 0
