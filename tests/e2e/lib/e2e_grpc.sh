#!/usr/bin/env bash
# shellcheck shell=bash
# gRPC helpers via grpcurl. Requires e2e_common.sh (E2E_REPO_ROOT, now_utc), BASE_URL optional, E2E_RUN_DIR.

e2e_grpc_log_dir() {
  echo "${E2E_RUN_DIR}/grpc"
}

# Default import root to repo proto/ when unset or invalid.
e2e_grpc_resolve_proto_root() {
  local candidate="${GRPC_PROTO_ROOT:-}"
  if [[ -z "$candidate" ]] || [[ ! -d "${candidate}/avf/machine/v1" ]]; then
    candidate="${E2E_REPO_ROOT}/proto"
  fi
  if [[ ! -d "${candidate}/avf/machine/v1" ]]; then
    log_error "e2e_grpc: no machine protos under GRPC_PROTO_ROOT=${candidate} (set GRPC_PROTO_ROOT to the directory containing avf/machine/v1)"
    return 2
  fi
  GRPC_PROTO_ROOT="$candidate"
  export GRPC_PROTO_ROOT
  return 0
}

# Append grpcurl flags into nameref array: plaintext, import-path, and each -proto.
e2e_grpc_append_proto_flags() {
  local -n __grpc_args="${1}"
  if [[ "${GRPC_USE_REFLECTION:-false}" == "true" ]]; then
    __grpc_args+=("-plaintext")
    return 0
  fi
  e2e_grpc_resolve_proto_root || return 2
  __grpc_args+=("-plaintext" "-import-path" "${GRPC_PROTO_ROOT}")
  local p
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    __grpc_args+=("-proto" "${p#"${GRPC_PROTO_ROOT}"/}")
  done < <(find "${GRPC_PROTO_ROOT}" -type f -name '*.proto' 2>/dev/null | LC_ALL=C sort)
}

# True if service block and rpc Method( exist in this repo under proto/avf/machine/v1/.
e2e_grpc_rpc_declared_in_repo() {
  local service="$1"
  local rpc="$2"
  e2e_grpc_resolve_proto_root || return 1
  local dir="${GRPC_PROTO_ROOT}/avf/machine/v1"
  [[ -d "$dir" ]] || return 1
  local f
  f="$(grep -l "service ${service}" "${dir}"/*.proto 2>/dev/null | head -n1)"
  [[ -n "$f" ]] || return 1
  grep -q "rpc ${rpc}(" "$f"
}

e2e_grpc_append_metadata_flags() {
  local -n __grpc_meta="${1}"
  local idem="${2:-}"
  if [[ -n "${MACHINE_TOKEN:-}" ]]; then
    __grpc_meta+=("-H" "authorization: Bearer ${MACHINE_TOKEN}")
  fi
  if [[ -n "${MACHINE_ID:-}" ]] && [[ "${GRPC_SEND_MACHINE_ID_HEADER:-true}" == "true" ]]; then
    __grpc_meta+=("-H" "x-machine-id: ${MACHINE_ID}")
  fi
  if [[ -n "$idem" ]]; then
    __grpc_meta+=("-H" "idempotency-key: ${idem}")
  fi
}

# Probe that something listens on GRPC_ADDR (best-effort when reflection is off).
e2e_grpc_tcp_open() {
  local host port
  host="${GRPC_ADDR%%:*}"
  port="${GRPC_ADDR##*:}"
  [[ -n "$host" ]] || return 1
  [[ -n "$port" ]] || return 1
  if command -v timeout >/dev/null 2>&1; then
    timeout 2 bash -c "echo >/dev/tcp/${host}/${port}" >/dev/null 2>&1
    return $?
  fi
  bash -c "echo >/dev/tcp/${host}/${port}" >/dev/null 2>&1
}

# Returns 0 if server appears reachable.
e2e_grpc_server_reachable() {
  if [[ "${GRPC_USE_REFLECTION:-false}" == "true" ]]; then
    local -a args=()
    e2e_grpc_append_proto_flags args || return 1
    set +e
    grpcurl "${args[@]}" -max-time 5 "${GRPC_ADDR}" list >/dev/null 2>&1
    local c=$?
    set -e
    return "$c"
  fi
  e2e_grpc_resolve_proto_root || return 1
  e2e_grpc_tcp_open
}

grpc_contract_record() {
  local flow_id="$1"
  local step="$2"
  local method="$3"
  local status="$4"
  local msg="$5"
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  mkdir -p "${E2E_RUN_DIR}/reports"
  local jl="${E2E_RUN_DIR}/reports/grpc-contract-results.jsonl"
  jq -nc \
    --arg ts "$(now_utc)" \
    --arg flow_id "$flow_id" \
    --arg step "$step" \
    --arg method "$method" \
    --arg status "$status" \
    --arg msg "$msg" \
    '{ts:$ts,flow_id:$flow_id,step:$step,method:$method,status:$status,message:$msg}' >>"${jl}"
}

# e2e_grpc_call MACHINE full_method json_file_stem json_payload [idempotency_key]
# Writes request/response/log/meta under grpc/. Returns grpcurl exit code (0 ok).
e2e_grpc_call() {
  local full_method="$1"
  local payload_json="$2"
  local output_stem="$3"
  local idempotency_key="${4:-}"

  require_cmd grpcurl

  local dir
  dir="$(e2e_grpc_log_dir)"
  mkdir -p "$dir" "${E2E_RUN_DIR}/reports"

  local req="${dir}/${output_stem}.request.json"
  local resp="${dir}/${output_stem}.response.json"
  local logf="${dir}/${output_stem}.log"
  local meta="${dir}/${output_stem}.meta.json"

  printf '%s' "${payload_json}" >"${req}"

  local -a args=()
  e2e_grpc_append_proto_flags args || return 2
  args+=("-d" "@${req}")
  e2e_grpc_append_metadata_flags args "${idempotency_key}"

  local t0 t1 elapsed ge result
  t0="$(python3 -c 'import time; print(time.time())')"
  echo "### $(now_utc) grpcurl ${GRPC_ADDR} ${full_method}" >>"${logf}"
  set +e
  grpcurl "${args[@]}" -max-time 60 "${GRPC_ADDR}" "${full_method}" >"${resp}" 2>>"${logf}"
  ge=$?
  set -e
  echo "### grpcurl exit ${ge}" >>"${logf}"
  t1="$(python3 -c 'import time; print(time.time())')"
  elapsed="$(python3 -c "print(int((${t1} - ${t0}) * 1000))")"
  result="error"
  [[ "$ge" -eq 0 ]] && result="ok"
  jq -nc \
    --arg m "$full_method" \
    --arg st "$output_stem" \
    --argjson ec "$ge" \
    --arg res "$result" \
    --argjson ms "$elapsed" \
    '{method:$m,step:$st,grpcurlExit:$ec,result:$res,elapsedMs:$ms}' >"${meta}"
  return "$ge"
}

# Public RPCs (activation / token refresh without bearer). Optional idempotency-key header only.
e2e_grpc_call_unauthenticated() {
  local full_method="$1"
  local payload_json="$2"
  local output_stem="$3"
  local idempotency_key="${4:-}"

  require_cmd grpcurl

  local dir
  dir="$(e2e_grpc_log_dir)"
  mkdir -p "$dir" "${E2E_RUN_DIR}/reports"

  local req="${dir}/${output_stem}.request.json"
  local resp="${dir}/${output_stem}.response.json"
  local logf="${dir}/${output_stem}.log"
  local meta="${dir}/${output_stem}.meta.json"

  printf '%s' "${payload_json}" >"${req}"

  local -a args=()
  e2e_grpc_append_proto_flags args || return 2
  args+=("-d" "@${req}")
  if [[ -n "$idempotency_key" ]]; then
    args+=("-H" "idempotency-key: ${idempotency_key}")
  fi

  local t0 t1 elapsed ge result
  t0="$(python3 -c 'import time; print(time.time())')"
  echo "### $(now_utc) grpcurl (unauthenticated) ${GRPC_ADDR} ${full_method}" >>"${logf}"
  set +e
  grpcurl "${args[@]}" -max-time 60 "${GRPC_ADDR}" "${full_method}" >"${resp}" 2>>"${logf}"
  ge=$?
  set -e
  echo "### grpcurl exit ${ge}" >>"${logf}"
  t1="$(python3 -c 'import time; print(time.time())')"
  elapsed="$(python3 -c "print(int((${t1} - ${t0}) * 1000))")"
  result="error"
  [[ "$ge" -eq 0 ]] && result="ok"
  jq -nc \
    --arg m "$full_method" \
    --arg st "$output_stem" \
    --argjson ec "$ge" \
    --arg res "$result" \
    --argjson ms "$elapsed" \
    '{method:$m,step:$st,grpcurlExit:$ec,result:$res,elapsedMs:$ms}' >"${meta}"
  return "$ge"
}

grpc_contract_try_unauth() {
  local flow_id="$1"
  local step="$2"
  local svc="$3"
  local rpc="$4"
  local payload="$5"
  local stem="$6"
  local idem="${7:-}"
  local full="avf.machine.v1.${svc}/${rpc}"

  if ! e2e_grpc_rpc_declared_in_repo "$svc" "$rpc"; then
    grpc_contract_record "$flow_id" "$step" "$full" "skip" "method_not_in_repo"
    e2e_append_test_event "$flow_id" "$step" "gRPC" "$full" "skipped" "method_not_in_repo" "{}"
    return 0
  fi
  set +e
  e2e_grpc_call_unauthenticated "$full" "$payload" "$stem" "$idem"
  local c=$?
  set -e
  if [[ "$c" -eq 0 ]]; then
    grpc_contract_record "$flow_id" "$step" "$full" "pass" "ok"
    e2e_append_test_event "$flow_id" "$step" "gRPC" "$full" "pass" "ok" "{}"
    return 0
  fi
  grpc_contract_record "$flow_id" "$step" "$full" "fail" "grpcurl_exit_${c} — see grpc/${stem}.log"
  e2e_append_test_event "$flow_id" "$step" "gRPC" "$full" "fail" "grpcurl_exit_${c}" "{}"
  return "$c"
}

# grpc_contract_try flow step ServiceName RpcName json stem [idempotency_key]
# skip if RPC not in repo; otherwise call and record pass/fail.
grpc_contract_try() {
  local flow_id="$1"
  local step="$2"
  local svc="$3"
  local rpc="$4"
  local payload="$5"
  local stem="$6"
  local idem="${7:-}"
  local full="avf.machine.v1.${svc}/${rpc}"

  if ! e2e_grpc_rpc_declared_in_repo "$svc" "$rpc"; then
    grpc_contract_record "$flow_id" "$step" "$full" "skip" "method_not_in_repo"
    e2e_append_test_event "$flow_id" "$step" "gRPC" "$full" "skipped" "method_not_in_repo" "{}"
    return 0
  fi

  set +e
  e2e_grpc_call "$full" "$payload" "$stem" "$idem"
  local c=$?
  set -e
  if [[ "$c" -eq 0 ]]; then
    grpc_contract_record "$flow_id" "$step" "$full" "pass" "ok"
    e2e_append_test_event "$flow_id" "$step" "gRPC" "$full" "pass" "ok" "{}"
    return 0
  fi
  grpc_contract_record "$flow_id" "$step" "$full" "fail" "grpcurl_exit_${c} — see grpc/${stem}.log"
  e2e_append_test_event "$flow_id" "$step" "gRPC" "$full" "fail" "grpcurl_exit_${c}" "{}"
  return "$c"
}

grpc_contract_skip() {
  local flow_id="$1"
  local step="$2"
  local svc="$3"
  local rpc="$4"
  local reason="$5"
  local full="avf.machine.v1.${svc}/${rpc}"
  grpc_contract_record "$flow_id" "$step" "$full" "skip" "$reason"
  e2e_append_test_event "$flow_id" "$step" "gRPC" "$full" "skipped" "$reason" "{}"
  if [[ "${E2E_ENABLE_FLOW_REVIEW:-true}" == "true" ]] && declare -F log_api_contract_issue >/dev/null 2>&1; then
    case "$reason" in
      *method_not_in_repo*)
        log_api_contract_issue "P3" "$flow_id" "$flow_id" "$step" "gRPC" "$full" \
          "RPC not declared in repo proto (skip: ${reason})" \
          "Cannot validate server implementation from harness proto set" \
          "Add proto stubs or document intentional omission" \
          "${E2E_RUN_DIR}/reports/grpc-contract-results.jsonl"
        ;;
      *unimplemented*|*UNIMPLEMENTED*|*not_implemented*)
        log_api_contract_issue "P1" "$flow_id" "$flow_id" "$step" "gRPC" "$full" \
          "RPC reported unimplemented or blocked (${reason})" \
          "Vending app flow may be broken in production path" \
          "Implement RPC or return structured unavailable; update matrix" \
          "${E2E_RUN_DIR}/grpc/${step}.log"
        ;;
    esac
  fi
}
