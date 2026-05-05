#!/usr/bin/env bash
# shellcheck shell=bash
# Shared E2E harness utilities. Source from runners only after E2E_SCRIPT_DIR is set.

E2E_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${E2E_SCRIPT_DIR:=$(cd "${E2E_LIB_DIR}/.." && pwd)}"
E2E_REPO_ROOT="$(cd "${E2E_SCRIPT_DIR}/../.." && pwd)"

e2e_strict_mode() {
  set -euo pipefail
}

load_env() {
  local env_file="${1:-}"
  if [[ -z "${env_file}" ]]; then
    env_file="${E2E_ENV_FILE:-}"
  fi
  if [[ -z "${env_file}" ]]; then
    env_file="${E2E_SCRIPT_DIR}/.env"
  fi
  if [[ -f "$env_file" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$env_file"
    set +a
  fi

  : "${BASE_URL:=http://127.0.0.1:8080}"
  : "${GRPC_ADDR:=127.0.0.1:9090}"
  : "${MQTT_HOST:=127.0.0.1}"
  : "${MQTT_PORT:=1883}"
  : "${POSTMAN_COLLECTION:=docs/postman/avf-vending-api-function-path.postman_collection.json}"
  : "${POSTMAN_ENV:=docs/postman/avf-local.postman_environment.json}"
  : "${E2E_TARGET:=local}"
  : "${E2E_ALLOW_WRITES:=true}"
  : "${E2E_REUSE_DATA:=false}"
  : "${E2E_ENABLE_FLOW_REVIEW:=true}"
  : "${E2E_WARN_SLOW_MS:=1500}"
  : "${E2E_FAIL_ON_P0_FINDINGS:=true}"
  : "${E2E_FAIL_ON_P1_FINDINGS:=false}"
  : "${E2E_GENERATE_OPTIMIZATION_BACKLOG:=true}"

  e2e_target_safety_guard
}

e2e_target_safety_guard() {
  if [[ "${E2E_TARGET}" == "production" && "${E2E_ALLOW_WRITES}" == "true" ]]; then
    if [[ "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" ]]; then
      echo "FATAL: E2E_TARGET=production with E2E_ALLOW_WRITES=true requires" >&2
      echo "      E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" >&2
      exit 2
    fi
  fi
}

require_cmd() {
  local c
  for c in "$@"; do
    if ! command -v "$c" >/dev/null 2>&1; then
      echo "FATAL: required command not found: $c" >&2
      exit 127
    fi
  done
}

require_env() {
  local v
  for v in "$@"; do
    if [[ -z "${!v:-}" ]]; then
      echo "FATAL: required environment variable not set: $v" >&2
      exit 2
    fi
  done
}

# Prefer Windows `py -3` when `python3` is missing or is a non-functional Store stub.
e2e_python() {
  if command -v py >/dev/null 2>&1 && py -3 -c "import sys" >/dev/null 2>&1; then
    py -3 "$@"
    return $?
  fi
  if command -v python3 >/dev/null 2>&1 && python3 -c "import sys" >/dev/null 2>&1; then
    python3 "$@"
    return $?
  fi
  echo "FATAL: no working Python 3 (install from python.org or use \`py -3\`; on Windows disable App Execution Aliases for python.exe/python3.exe if they open the Store stub)" >&2
  return 127
}

e2e_require_python() {
  e2e_python -c "import sys" >/dev/null 2>&1 || exit 127
}

now_utc() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

new_run_dir() {
  if [[ -n "${E2E_RUN_DIR:-}" ]] && [[ -d "${E2E_RUN_DIR}" ]]; then
    mkdir -p "${E2E_RUN_DIR}/rest" "${E2E_RUN_DIR}/grpc" "${E2E_RUN_DIR}/mqtt" "${E2E_RUN_DIR}/reports"
    : "${E2E_EVENTS_FILE:=${E2E_RUN_DIR}/events.jsonl}"
    : >>"${E2E_RUN_DIR}/improvement-findings.jsonl"
    return 0
  fi
  local base="${E2E_REPO_ROOT}/.e2e-runs"
  mkdir -p "$base"
  E2E_RUN_DIR="${base}/run-$(date -u +%Y%m%dT%H%M%SZ)-$$-${RANDOM}"
  mkdir -p "${E2E_RUN_DIR}/rest" "${E2E_RUN_DIR}/grpc" "${E2E_RUN_DIR}/mqtt" "${E2E_RUN_DIR}/reports"
  export E2E_RUN_DIR
  : "${E2E_EVENTS_FILE:=${E2E_RUN_DIR}/events.jsonl}"
  : >>"${E2E_RUN_DIR}/improvement-findings.jsonl"
}

log_info() {
  echo "[e2e][INFO]  $(now_utc) $*" >&2
}

log_warn() {
  echo "[e2e][WARN]  $(now_utc) $*" >&2
}

log_error() {
  echo "[e2e][ERROR] $(now_utc) $*" >&2
}

E2E_CURRENT_STEP=""

start_step() {
  E2E_CURRENT_STEP="$1"
  log_info "STEP START: ${E2E_CURRENT_STEP}"
}

end_step() {
  local status="$1"
  shift || true
  local msg="${*:-}"
  append_event_jsonl "${E2E_CURRENT_STEP:-unknown}" "$status" "$msg"
  log_info "STEP END: ${E2E_CURRENT_STEP:-unknown} -> ${status}"
}

append_event_jsonl() {
  local step="$1"
  local status="$2"
  local message="${3:-}"
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  : "${E2E_EVENTS_FILE:=${E2E_RUN_DIR}/events.jsonl}"
  local line
  line="$(
    jq -nc \
      --arg ts "$(now_utc)" \
      --arg step "$step" \
      --arg st "$status" \
      --arg msg "$message" \
      '{ts:$ts,step:$step,status:$st,message:$msg}'
  )"
  echo "$line" >>"${E2E_EVENTS_FILE}"
}

fail_step() {
  append_event_jsonl "${E2E_CURRENT_STEP:-unknown}" "failed" "$*"
  log_error "$*"
  return 1
}

mask_secret() {
  local s="${1:-}"
  local n="${#s}"
  if [[ "$n" -le 8 ]]; then
    echo "***"
    return
  fi
  echo "${s:0:4}***${s:n-4:4}"
}

safe_json_string() {
  jq -n --arg s "$1" '$s'
}

cleanup_trap_register() {
  trap 'e2e_cleanup_trap_handler' EXIT
}

e2e_cleanup_trap_handler() {
  local ec=$?
  if [[ "${E2E_REPORT_DONE:-0}" != "1" ]] && [[ -n "${E2E_RUN_DIR:-}" ]] && [[ -f "${E2E_SCRIPT_DIR:-}/lib/e2e_report.sh" ]]; then
    # shellcheck disable=SC1090
    source "${E2E_SCRIPT_DIR}/lib/e2e_report.sh"
    e2e_finalize_reports "$ec" || true
  fi
  return "$ec"
}

e2e_skip() {
  log_warn "SKIP: $*"
}

e2e_write_run_meta() {
  local runner="${1:-unknown}"
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  local meta="${E2E_RUN_DIR}/run-meta.json"
  if [[ ! -f "$meta" ]]; then
    jq -nc \
      --arg ts "$(now_utc)" \
      --arg runner "$runner" \
      --arg root "$E2E_REPO_ROOT" \
      --argjson pid "$$" \
      --arg target "${E2E_TARGET:-local}" \
      '{
        startedAt:$ts,
        runner:$runner,
        repoRoot:$root,
        pid:$pid,
        e2eTarget:$target
      }' >"$meta"
  else
    local tmp
    tmp="$(mktemp)"
    jq --arg ts "$(now_utc)" \
      --arg runner "$runner" \
      --argjson pid "$$" \
      '.lastPhaseAt=$ts | .lastRunner=$runner | .lastPid=$pid' "$meta" >"$tmp" && mv "$tmp" "$meta"
  fi
}

# --reuse-data path | --fresh-data | --help
# Remaining argv stored in E2E_EXTRA_ARGS (array).
e2e_parse_common_args() {
  E2E_CLI_REUSE_DATA_PATH=""
  E2E_CLI_FRESH_DATA="false"
  E2E_SHOW_HELP="false"
  E2E_EXTRA_ARGS=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --reuse-data)
        [[ $# -ge 2 ]] || { echo "FATAL: --reuse-data requires a path" >&2; exit 2; }
        E2E_CLI_REUSE_DATA_PATH="$2"
        shift 2
        ;;
      --fresh-data) E2E_CLI_FRESH_DATA="true"; shift ;;
      --help | -h) E2E_SHOW_HELP="true"; shift ;;
      *)
        E2E_EXTRA_ARGS+=("$1")
        shift
        ;;
    esac
  done

  if [[ "${E2E_SHOW_HELP}" == "true" ]]; then
    if declare -F e2e_print_help >/dev/null 2>&1; then
      e2e_print_help
    else
      e2e_print_help_run_all
    fi
    exit 0
  fi

  export E2E_CLI_FRESH_DATA

  if [[ -n "$E2E_CLI_REUSE_DATA_PATH" ]]; then
    E2E_REUSE_DATA="true"
    E2E_DATA_FILE="$E2E_CLI_REUSE_DATA_PATH"
  fi
  if [[ "${E2E_CLI_FRESH_DATA}" == "true" ]]; then
    E2E_REUSE_DATA="false"
    E2E_DATA_FILE=""
  fi

  export E2E_REUSE_DATA
  export E2E_DATA_FILE

  return 0
}

e2e_capture_inherited_data_flags() {
  E2E_INHERITED_REUSE_DATA="${E2E_REUSE_DATA-}"
  E2E_INHERITED_DATA_FILE="${E2E_DATA_FILE-}"
}

# After load_env in a child process: .env may clobber reuse flags from the orchestrator.
e2e_restore_inherited_data_flags_if_needed() {
  [[ "${E2E_IN_PARENT:-0}" == "1" ]] || return 0
  if [[ -n "${E2E_CLI_REUSE_DATA_PATH:-}" ]] || [[ "${E2E_CLI_FRESH_DATA:-false}" == "true" ]]; then
    return 0
  fi
  E2E_REUSE_DATA="${E2E_INHERITED_REUSE_DATA:-false}"
  E2E_DATA_FILE="${E2E_INHERITED_DATA_FILE:-}"
  export E2E_REUSE_DATA E2E_DATA_FILE
}

e2e_print_help_run_all() {
  cat <<EOF
Usage: ./tests/e2e/run-all-local.sh [options] [-- extra args passed to phase runners]

Options:
  --fresh-data              Start with empty test-data.json
  --reuse-data PATH         Load capture JSON into test-data.json
  -h, --help                Show this help

Environment: copy tests/e2e/.env.example -> tests/e2e/.env
Artifacts:    \$REPO_ROOT/.e2e-runs/run-*

Phase runners (same options):
  ./tests/e2e/run-rest-local.sh
  ./tests/e2e/run-web-admin-flows.sh
  ./tests/e2e/run-vending-app-flows.sh
  ./tests/e2e/run-grpc-local.sh
  ./tests/e2e/run-mqtt-local.sh
EOF
}

# shellcheck disable=SC1091
source "${E2E_LIB_DIR}/e2e_flow_review.sh"
