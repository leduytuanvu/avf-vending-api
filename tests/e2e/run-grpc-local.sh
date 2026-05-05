#!/usr/bin/env bash
# shellcheck shell=bash
# gRPC scenarios using grpcurl (optional proto root or reflection).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-grpc-local.sh [options]

Environment:
  GRPC_ADDR              host:port (default 127.0.0.1:9090)
  GRPC_USE_REFLECTION    true/false
  GRPC_PROTO_ROOT        import path when reflection is false
  MACHINE_TOKEN          optional bearer for machine calls

  --fresh-data / --reuse-data PATH / -h
EOF
}

e2e_capture_inherited_data_flags
e2e_parse_common_args "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

require_cmd jq

new_run_dir
e2e_write_run_meta "run-grpc-local"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  e2e_data_initialize
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] || cleanup_trap_register

start_step "grpc-local-suite"
if ! command -v grpcurl >/dev/null 2>&1; then
  e2e_skip "grpcurl not installed"
  end_step skipped "grpcurl missing"
  [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
  exit 0
fi

# shellcheck source=lib/e2e_grpc.sh
source "${SCRIPT_DIR}/lib/e2e_grpc.sh"

SCENARIO="${SCRIPT_DIR}/scenarios/grpc_local.sh"
if [[ -f "$SCENARIO" ]]; then
  set +e
  # shellcheck disable=SC1090
  source "$SCENARIO"
  ec=$?
  set -e
  if [[ "$ec" -eq 0 ]]; then
    end_step passed "gRPC scenarios completed"
  else
    end_step failed "gRPC scenario exit ${ec}"
    [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 1
    exit 1
  fi
else
  if [[ "${GRPC_USE_REFLECTION:-false}" != "true" ]] && [[ -z "${GRPC_PROTO_ROOT:-}" ]]; then
    log_warn "GRPC_USE_REFLECTION=false and GRPC_PROTO_ROOT empty — configure before real gRPC calls"
  fi
  e2e_skip "gRPC scenarios not implemented (${SCENARIO} missing)"
  end_step skipped "gRPC placeholder"
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
exit 0
