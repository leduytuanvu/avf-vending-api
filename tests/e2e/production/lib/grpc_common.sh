#!/usr/bin/env bash
# shellcheck shell=bash
# Shared grpcurl helpers for production gRPC E2E.

prod_e2e_grpc_proto_args() {
  local -n _out="$1"
  if [[ "${GRPC_USE_REFLECTION:-false}" == "true" ]]; then
    if [[ "${GRPC_USE_PLAINTEXT:-false}" == "true" ]]; then
      _out+=(-plaintext)
    fi
    return 0
  fi
  local proto_root="${GRPC_PROTO_ROOT:-${PROD_E2E_REPO_ROOT}/proto}"
  [[ -d "${proto_root}/avf/machine/v1" ]] || {
    echo "missing machine protos under ${proto_root}" >&2
    return 1
  }
  if [[ "${GRPC_USE_PLAINTEXT:-false}" == "true" ]]; then
    _out+=(-plaintext)
  fi
  _out+=(-import-path "$proto_root")
  local p
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    _out+=(-proto "${p#"${proto_root}"/}")
  done < <(find "$proto_root" -type f -name '*.proto' 2>/dev/null | LC_ALL=C sort)
}

prod_e2e_grpc_full_method() {
  local service="$1"
  local rpc="$2"
  if [[ "$service" == avf.machine.v1.* ]]; then
    printf '%s/%s' "$service" "$rpc"
  else
    printf 'avf.machine.v1.%s/%s' "$service" "$rpc"
  fi
}

prod_e2e_grpc_now_rfc3339() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

prod_e2e_grpc_meta_json() {
  local request_id="${1:-${PROD_E2E_PREFIX}-grpc}"
  local idem_key="${2:-}"
  local client_event="${3:-}"
  jq -nc \
    --arg mid "${machineId:-${MACHINE_ID:-}}" \
    --arg rid "$request_id" \
    --arg ik "$idem_key" \
    --arg ce "$client_event" \
    --arg ts "$(prod_e2e_grpc_now_rfc3339)" \
    --arg av "${PROD_E2E_GRPC_APP_VERSION:-e2e-prod-grpc/1}" \
    '{
      machineId: $mid,
      requestId: $rid,
      occurredAt: $ts,
      appVersion: $av
    }
    + (if $ik != "" then {idempotencyKey: $ik} else {} end)
    + (if $ce != "" then {clientEventId: $ce} else {} end)'
}

prod_e2e_grpc_idem_context() {
  local stem="$1"
  local ts
  ts="$(prod_e2e_grpc_now_rfc3339)"
  jq -nc \
    --arg ik "${PROD_E2E_PREFIX}-${stem}-ik" \
    --arg ce "${PROD_E2E_PREFIX}-${stem}-ce" \
    --arg ts "$ts" \
    '{idempotencyKey: $ik, clientEventId: $ce, clientCreatedAt: $ts}'
}

prod_e2e_grpc_machine_token() {
  if declare -F prod_e2e_state_reload_key >/dev/null 2>&1; then
    prod_e2e_state_reload_key machineToken || true
  fi
  printf '%s' "${machineToken:-${MACHINE_TOKEN:-}}"
}

prod_e2e_grpc_redact_log() {
  local in_file="$1"
  local out_file="$2"
  if [[ ! -f "$in_file" ]]; then
    : >"$out_file"
    return 0
  fi
  sed -E \
    -e 's/(authorization:[[:space:]]*Bearer[[:space:]]+)[^[:space:]]+/\1<redacted>/gi' \
    -e 's/("accessToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
    -e 's/("refreshToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
    -e 's/("machineToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
    "$in_file" >"$out_file" 2>/dev/null || cp "$in_file" "$out_file"
}
