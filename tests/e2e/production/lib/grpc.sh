#!/usr/bin/env bash
# shellcheck shell=bash
# gRPC flow executor via grpcurl — separate evidence section (not Postman).

prod_e2e_grpc_execute_flow() {
  local flow_json="$1"
  local id label service rpc evidence_label expected_code
  id="$(echo "$flow_json" | jq -r '.id')"
  label="$(echo "$flow_json" | jq -r '.label')"
  service="$(echo "$flow_json" | jq -r '.service')"
  rpc="$(echo "$flow_json" | jq -r '.rpc')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  expected_code="$(echo "$flow_json" | jq -r '.expected_code // "OK"')"

  if [[ "${PROD_E2E_DRY_RUN:-}" == "1" ]]; then
    prod_e2e_evidence_append_row "$id" "$label" "grpc" "dry-run" "$evidence_label"
    return 0
  fi

  if ! command -v grpcurl >/dev/null 2>&1; then
    prod_e2e_evidence_append_row "$id" "$label" "grpc" "skip-no-grpcurl" "$evidence_label"
    return 0
  fi

  local req_body resp_file log_file
  req_body="$(echo "$flow_json" | jq -c '.request_template // {}')"
  req_body="$(prod_e2e_render_template_string "$req_body")"
  req_file="${PROD_E2E_RAW_DIR}/${evidence_label}.request.json"
  resp_file="${PROD_E2E_RAW_DIR}/${evidence_label}.response.json"
  log_file="${PROD_E2E_RAW_DIR}/${evidence_label}.grpc.log"
  printf '%s\n' "$req_body" >"$req_file"

  local -a args=()
  if [[ "${GRPC_USE_REFLECTION:-false}" == "true" ]]; then
    args+=(-plaintext)
  else
    local proto_root="${GRPC_PROTO_ROOT:-${PROD_E2E_REPO_ROOT}/proto}"
    args+=(-plaintext -import-path "$proto_root")
    local p
    while IFS= read -r p; do
      [[ -n "$p" ]] || continue
      args+=(-proto "${p#"${proto_root}"/}")
    done < <(find "$proto_root" -type f -name '*.proto' 2>/dev/null | LC_ALL=C sort)
  fi
  if [[ -n "${machineToken:-}" ]]; then
    args+=(-H "authorization: Bearer ${machineToken}")
  fi
  if [[ -n "${machineId:-}" ]]; then
    args+=(-H "x-machine-id: ${machineId}")
  fi

  local full_rpc="${service}/${rpc}"
  local grpc_target="${GRPC_ADDR:-${E2E_PROD_GRPC_TARGET:-}}"
  set +e
  grpcurl "${args[@]}" -d @ "${grpc_target}" "${full_rpc}" <"$req_file" >"$resp_file" 2>"$log_file"
  local rc=$?
  set -e

  local status="pass"
  if [[ $rc -ne 0 ]]; then
    status="fail"
  fi
  prod_e2e_evidence_append_row "$id" "$label" "grpc" "$status" "$evidence_label"
  prod_e2e_evidence_append_grpc_section "$id" "$evidence_label" "${full_rpc}" "${expected_code}"
  [[ "$status" == "fail" ]] && return 1
  return 0
}
