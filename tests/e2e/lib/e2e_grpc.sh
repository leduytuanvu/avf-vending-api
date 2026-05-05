#!/usr/bin/env bash
# shellcheck shell=bash
# gRPC helpers via grpcurl. Requires e2e_common.sh, GRPC_ADDR, E2E_RUN_DIR.

e2e_grpc_log_dir() {
  echo "${E2E_RUN_DIR}/grpc"
}

e2e_grpc_call() {
  local method="$1"
  local payload_json="$2"
  local output_name="$3"
  require_cmd grpcurl

  local dir
  dir="$(e2e_grpc_log_dir)"
  mkdir -p "$dir"

  local req="${dir}/${output_name}.request.json"
  local resp="${dir}/${output_name}.response.json"
  local logf="${dir}/${output_name}.log"

  printf '%s' "${payload_json}" >"${req}"

  local -a args=()
  args+=("-d" "@${req}")

  if [[ "${GRPC_USE_REFLECTION:-false}" == "true" ]]; then
    args+=("-plaintext")
  else
    if [[ -z "${GRPC_PROTO_ROOT:-}" ]]; then
      log_error "GRPC_USE_REFLECTION=false but GRPC_PROTO_ROOT is empty"
      return 2
    fi
    args+=("-plaintext" "-import-path" "${GRPC_PROTO_ROOT}")
    while IFS= read -r proto; do
      [[ -n "$proto" ]] || continue
      local rel="${proto#"${GRPC_PROTO_ROOT}"/}"
      args+=("-proto" "${rel}")
    done < <(find "${GRPC_PROTO_ROOT}" -type f -name '*.proto' 2>/dev/null | LC_ALL=C sort)
  fi

  if [[ -n "${MACHINE_TOKEN:-}" ]]; then
    args+=("-H" "authorization: Bearer ${MACHINE_TOKEN}")
  fi

  {
    echo "### $(now_utc) grpcurl ${GRPC_ADDR} ${method}" >>"${logf}"
    set +e
    grpcurl "${args[@]}" "${GRPC_ADDR}" "${method}" >"${resp}" 2>>"${logf}"
    local ec=$?
    set -e
    echo "### exit ${ec}" >>"${logf}"
    return "$ec"
  }
}
