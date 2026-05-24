#!/usr/bin/env bash
# shellcheck shell=bash
# gRPC flow executor via grpcurl — evidence under .e2e-runs/production/<runId>/raw/

# shellcheck source=grpc_common.sh
source "$(dirname "${BASH_SOURCE[0]}")/grpc_common.sh"

prod_e2e_grpc_call_raw() {
  local full_method="$1"
  local req_body="$2"
  local evidence_label="$3"
  local auth="${4:-machine}"
  local idempotency_key="${5:-}"

  local req_file="${PROD_E2E_RAW_DIR}/${evidence_label}.request.json"
  local resp_file="${PROD_E2E_RAW_DIR}/${evidence_label}.response.json"
  local log_file="${PROD_E2E_RAW_DIR}/${evidence_label}.grpc.log"
  local meta_file="${PROD_E2E_RAW_DIR}/${evidence_label}.meta.json"
  local hdr_file="${PROD_E2E_RAW_DIR}/${evidence_label}.metadata.txt"
  local log_redacted="${PROD_E2E_RAW_DIR}/${evidence_label}.grpc.redacted.log"

  local req_send_file="${PROD_E2E_RAW_DIR}/${evidence_label}.request.send.json"
  printf '%s\n' "$req_body" >"$req_send_file"
  prod_e2e_redact_file "$req_send_file" "$req_file"

  local -a args=()
  prod_e2e_grpc_proto_args args || return 1

  case "$auth" in
    machine)
      local tok
      tok="$(prod_e2e_grpc_machine_token)"
      [[ -n "$tok" ]] || {
        echo "gRPC ${evidence_label}: missing machine token" >&2
        return 1
      }
      args+=(-H "authorization: Bearer ${tok}")
      if [[ -n "${machineId:-${MACHINE_ID:-}}" ]]; then
        args+=(-H "x-machine-id: ${machineId:-${MACHINE_ID}}")
      fi
      ;;
    none) ;;
    *)
      echo "unknown gRPC auth mode: ${auth}" >&2
      return 1
      ;;
  esac

  if [[ -n "$idempotency_key" ]]; then
    args+=(-H "idempotency-key: ${idempotency_key}")
  fi

  args+=(-d @)
  local grpc_target="${GRPC_ADDR:-${E2E_PROD_GRPC_TARGET:-}}"
  [[ -n "$grpc_target" ]] || {
    echo "gRPC target unset (E2E_PROD_GRPC_TARGET / GRPC_ADDR)" >&2
    return 1
  }

  local t0 t1 elapsed rc
  t0="$(prod_e2e_py -c 'import time; print(time.time())')"
  : >"$hdr_file"
  {
    echo "target=${grpc_target}"
    echo "method=${full_method}"
    echo "auth=${auth}"
    echo "idempotency_key=${idempotency_key:-<none>}"
  } >>"$hdr_file"

  set +e
  grpcurl "${args[@]}" -max-time "${GRPC_MAX_TIME:-120}" "${grpc_target}" "${full_method}" \
    <"$req_send_file" >"$resp_file" 2>"$log_file"
  rc=$?
  set -e
  rm -f "$req_send_file"
  t1="$(prod_e2e_py -c 'import time; print(time.time())')"
  elapsed="$(prod_e2e_py -c "print(int((${t1} - ${t0}) * 1000))")"

  prod_e2e_grpc_redact_log "$log_file" "$log_redacted"
  mv "$log_redacted" "$log_file"
  local resp_capture="${resp_file}.capture.json"
  if [[ -f "$resp_file" && -s "$resp_file" ]]; then
    cp "$resp_file" "$resp_capture"
    prod_e2e_redact_file "$resp_file" "${resp_file}.redacted"
    mv "${resp_file}.redacted" "$resp_file"
  else
    rm -f "$resp_capture"
  fi

  local trace_id request_id grpc_status error_code
  trace_id="$(jq -r '.meta.traceId // .meta.trace_id // empty' "$resp_file" 2>/dev/null || true)"
  request_id="$(jq -r '.meta.requestId // .meta.request_id // empty' "$resp_file" 2>/dev/null || true)"
  grpc_status="$(jq -r '.meta.status // empty' "$resp_file" 2>/dev/null || true)"
  error_code="$(jq -r '.meta.errorCode // .meta.error_code // empty' "$resp_file" 2>/dev/null || true)"

  jq -nc \
    --arg method "$full_method" \
    --arg label "$evidence_label" \
    --arg target "$grpc_target" \
    --arg auth "$auth" \
    --argjson exit_code "$rc" \
    --argjson elapsed_ms "$elapsed" \
    --arg trace_id "$trace_id" \
    --arg request_id "$request_id" \
    --arg grpc_status "$grpc_status" \
    --arg error_code "$error_code" \
    '{
      method: $method,
      evidence_label: $label,
      target: $target,
      auth: $auth,
      grpcurl_exit: $exit_code,
      elapsed_ms: $elapsed_ms,
      trace_id: $trace_id,
      request_id: $request_id,
      meta_status: $grpc_status,
      error_code: $error_code
    }' >"$meta_file"

  PROD_E2E_GRPC_LAST_RC=$rc
  if [[ -f "$resp_capture" ]]; then
    PROD_E2E_GRPC_LAST_RESP="$resp_capture"
  else
    PROD_E2E_GRPC_LAST_RESP="$resp_file"
  fi
  export PROD_E2E_GRPC_LAST_RC PROD_E2E_GRPC_LAST_RESP
  return "$rc"
}

prod_e2e_grpc_execute_flow() {
  local flow_json="$1"
  local id label service rpc evidence_label expected_code auth
  id="$(echo "$flow_json" | jq -r '.id')"
  label="$(echo "$flow_json" | jq -r '.label')"
  service="$(echo "$flow_json" | jq -r '.service')"
  rpc="$(echo "$flow_json" | jq -r '.rpc')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  expected_code="$(echo "$flow_json" | jq -r '.expected_code // "OK"')"
  auth="$(echo "$flow_json" | jq -r '.auth // "machine"')"

  local skip_if skip_if_empty
  skip_if="$(echo "$flow_json" | jq -r '.skip_if_env // empty')"
  if [[ -n "$skip_if" && -n "${!skip_if:-}" ]]; then
    prod_e2e_evidence_append_row "$id" "$label" "grpc" "skipped" "$evidence_label"
    return 0
  fi
  skip_if_empty="$(echo "$flow_json" | jq -r '.skip_if_empty // empty')"
  if [[ -n "$skip_if_empty" && -z "${!skip_if_empty:-}" ]]; then
    prod_e2e_evidence_append_row "$id" "$label" "grpc" "skipped" "$evidence_label"
    return 0
  fi

  if [[ "${PROD_E2E_DRY_RUN:-}" == "1" ]]; then
    prod_e2e_evidence_append_row "$id" "$label" "grpc" "dry-run" "$evidence_label"
    return 0
  fi

  if ! command -v grpcurl >/dev/null 2>&1; then
    prod_e2e_fail_classify "a" "$id" "grpcurl not installed"
    prod_e2e_evidence_append_row "$id" "$label" "grpc" "skip-no-grpcurl" "$evidence_label"
    return 1
  fi

  if [[ "$rpc" == "RefreshMachineToken" ]]; then
    prod_e2e_state_reload_key machineRefreshToken || true
  fi

  local req_body idem_key
  idem_key="$(echo "$flow_json" | jq -r '.idempotency_key // empty')"
  idem_key="$(prod_e2e_render_template_string "$idem_key")"
  req_body="$(echo "$flow_json" | jq -c '.request_template // {}')"
  req_body="$(prod_e2e_render_template_string "$req_body")"
  local inject_meta=1
  if echo "$flow_json" | jq -e '.inject_meta == false' >/dev/null 2>&1; then
    inject_meta=0
  fi
  if [[ "$inject_meta" -eq 1 ]]; then
    if echo "$req_body" | jq -e '.meta == null or (.meta | type == "object" and length == 0)' >/dev/null 2>&1; then
      local meta_json
      meta_json="$(prod_e2e_grpc_meta_json "${PROD_E2E_PREFIX}-${evidence_label}" "$idem_key")"
      req_body="$(echo "$req_body" | jq --argjson m "$meta_json" '. + {meta: $m}')"
    elif [[ -n "$idem_key" ]] && echo "$req_body" | jq -e '.meta != null' >/dev/null 2>&1; then
      req_body="$(echo "$req_body" | jq --arg ik "$idem_key" '.meta += {idempotencyKey: $ik}')"
    fi
  fi

  local full_method
  full_method="$(prod_e2e_grpc_full_method "$service" "$rpc")"

  if ! prod_e2e_grpc_call_raw "$full_method" "$req_body" "$evidence_label" "$auth" "$idem_key"; then
    local grpc_err grpc_meta="${PROD_E2E_RAW_DIR}/${evidence_label}.meta.json"
    grpc_err="$(grep -E 'Code:|Message:' "${PROD_E2E_RAW_DIR}/${evidence_label}.grpc.log" 2>/dev/null | tr '\n' ' ' | sed 's/[[:space:]]\+/ /g' || true)"
    prod_e2e_fail_classify "a" "$id" "method=${full_method} grpcurl_exit=${PROD_E2E_GRPC_LAST_RC:-1} ${grpc_err:-grpc call failed}"
    prod_e2e_evidence_append_row "$id" "$label" "grpc" "fail" "$evidence_label"
    prod_e2e_evidence_append_grpc_section "$id" "$evidence_label" "$full_method" "ERROR"
    [[ -f "$grpc_meta" ]] && jq -c '{grpcurl_exit, method, error_code, meta_status}' "$grpc_meta" 2>/dev/null >&2 || true
    return 1
  fi

  local capture assertions capture_body
  capture_body="${PROD_E2E_GRPC_LAST_RESP:-${PROD_E2E_RAW_DIR}/${evidence_label}.response.json}"
  capture="$(echo "$flow_json" | jq -c '.capture // null')"
  prod_e2e_capture_from_body "$capture_body" "$capture"

  assertions="$(echo "$flow_json" | jq -c '.assertions // []')"
  if [[ "$assertions" != "[]" && "$assertions" != "null" ]]; then
    while IFS= read -r a; do
      [[ -n "$a" ]] || continue
      prod_e2e_assert_body_file "${PROD_E2E_RAW_DIR}/${evidence_label}.response.json" "$id" "$a" || {
        prod_e2e_evidence_append_row "$id" "$label" "grpc" "fail" "$evidence_label"
        prod_e2e_evidence_append_grpc_section "$id" "$evidence_label" "$full_method" "ASSERT_FAIL"
        return 1
      }
    done < <(echo "$assertions" | jq -c '.[]')
  fi

  prod_e2e_evidence_append_row "$id" "$label" "grpc" "pass" "$evidence_label"
  prod_e2e_evidence_append_grpc_section "$id" "$evidence_label" "$full_method" "${expected_code}"
  return 0
}

prod_e2e_grpc_run_flow() {
  local flow_json="$1"
  local handler
  handler="$(echo "$flow_json" | jq -r '.handler // empty')"
  if [[ -n "$handler" && "$handler" != "null" ]]; then
    prod_e2e_grpc_dispatch "$flow_json"
  else
    prod_e2e_grpc_execute_flow "$flow_json"
  fi
}
