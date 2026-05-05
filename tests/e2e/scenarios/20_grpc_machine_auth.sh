#!/usr/bin/env bash
# shellcheck shell=bash
# GRPC-20: Machine activation / token refresh (production vending app path).

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_grpc.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_grpc.sh"

FLOW_ID="GRPC-20"
ec=0

grpc_contract_skip "$FLOW_ID" "claim-parallel-activation-svc" MachineActivationService ClaimActivation \
  "harness_uses_MachineAuthService_ClaimActivation_only"

_mid="$(get_data machineId)"
_mt="$(get_secret machineToken 2>/dev/null || true)"
if [[ -n "${_mt:-}" ]] && [[ -n "${_mid:-}" ]] && [[ "${_mid}" != "null" ]]; then
  grpc_contract_skip "$FLOW_ID" "claim-activation" MachineAuthService ClaimActivation "reuse_existing_machine_token"
  if e2e_grpc_rpc_declared_in_repo MachineTokenService RefreshMachineToken; then
    _rt="$(get_secret machineRefreshToken 2>/dev/null || true)"
    if [[ -n "${_rt:-}" ]]; then
      export MACHINE_ID="$_mid"
      export MACHINE_TOKEN="$_mt"
      RBODY="$(jq -nc --arg r "$_rt" '{refreshToken:$r}')"
      grpc_contract_try_unauth "$FLOW_ID" "refresh-public" MachineTokenService RefreshMachineToken "$RBODY" "g20-refresh-public" "g20-rt-pub" || ec=1
      _nr="$(jq -r '.accessToken // empty' "${E2E_RUN_DIR}/grpc/g20-refresh-public.response.json" 2>/dev/null)"
      [[ -n "$_nr" ]] && e2e_save_token machineToken "$_nr" && export MACHINE_TOKEN="$_nr"
      _rt_refresh="$(get_secret machineRefreshToken 2>/dev/null || true)"
      [[ -n "${_rt_refresh:-}" ]] && _rt="$_rt_refresh"
      ABODY="$(jq -nc --arg r "$_rt" '{refresh:{refreshToken:$r}}')"
      grpc_contract_try "$FLOW_ID" "refresh-auth-wrapped" MachineAuthService RefreshMachineToken "$ABODY" "g20-refresh-auth" "g20-rt-auth" || ec=1
      _nr2="$(jq -r '.refresh.accessToken // .refresh.access_token // empty' "${E2E_RUN_DIR}/grpc/g20-refresh-auth.response.json" 2>/dev/null)"
      [[ -n "$_nr2" ]] && e2e_save_token machineToken "$_nr2" && export MACHINE_TOKEN="$_nr2"
    else
      grpc_contract_skip "$FLOW_ID" "refresh-public" MachineTokenService RefreshMachineToken "no_machine_refresh_token_in_secrets"
      grpc_contract_skip "$FLOW_ID" "refresh-auth-wrapped" MachineAuthService RefreshMachineToken "no_machine_refresh_token_in_secrets"
    fi
  fi
  SCEN="20_grpc_machine_auth.sh"
  if [[ "${GRPC_USE_REFLECTION:-false}" != "true" ]]; then
    log_docs_gap "P2" "$FLOW_ID" "$SCEN" "grpc-entry" "gRPC" "${GRPC_ADDR:-}" "Server reflection off — operators need documented GRPC_ADDR and proto import root" "Harder local integration" "Document ports; enable reflection in dev or publish proto bundle" "${E2E_RUN_DIR}/grpc/g20-claim.meta.json"
  fi
  if [[ "${ec}" -eq 0 ]]; then
    e2e_flow_review_scenario_complete "$FLOW_ID" "$SCEN" "flow-review-complete" "grpc_auth_token_reuse_ok"
  fi
  exit "${ec}"
fi

AC="${E2E_ACTIVATION_CODE:-}"
[[ -z "${AC}" || "${AC}" == "null" ]] && AC="$(get_data activationCodePlain)"
if [[ -z "${AC}" || "${AC}" == "null" ]]; then
  grpc_contract_skip "$FLOW_ID" "claim-activation" MachineAuthService ClaimActivation "no_activation_code_set_E2E_ACTIVATION_CODE_or_activationCodePlain"
  log_docs_gap "P2" "$FLOW_ID" "20_grpc_machine_auth.sh" "activation-input" "gRPC" "MachineAuthService/ClaimActivation" "No activation code available — full gRPC claim path not exercised in this run" "Coverage gap for pilots" "Document E2E_ACTIVATION_CODE and secrets wiring" "${E2E_RUN_DIR}/test-data.json"
  e2e_flow_review_scenario_complete "$FLOW_ID" "20_grpc_machine_auth.sh" "flow-review-skip" "grpc_claim_skipped_no_code"
  exit 0
fi

SER="${E2E_DEVICE_FINGERPRINT_SERIAL:-e2e-grpc-serial}"
CLAIM_BODY="$(jq -nc --arg c "$AC" --arg s "$SER" \
  '{claim:{activationCode:$c, deviceFingerprint:{serialNumber:$s, packageName:"e2e.grpc.contract", versionName:"1.0", versionCode:1}}}')"
grpc_contract_try_unauth "$FLOW_ID" "claim-activation" MachineAuthService ClaimActivation "$CLAIM_BODY" "g20-claim" "g20-claim" || ec=1

RESP="${E2E_RUN_DIR}/grpc/g20-claim.response.json"
if [[ -f "$RESP" ]] && jq -e '.claim' "$RESP" >/dev/null 2>&1; then
  ACCESS="$(jq -r '.claim.accessToken // empty' "$RESP")"
  REFRESH="$(jq -r '.claim.refreshToken // empty' "$RESP")"
  MID="$(jq -r '.claim.machineId // empty' "$RESP")"
  ORG="$(jq -r '.claim.organizationId // empty' "$RESP")"
  SITE="$(jq -r '.claim.siteId // empty' "$RESP")"
  [[ -n "$ACCESS" ]] && e2e_save_token machineToken "$ACCESS"
  [[ -n "$REFRESH" ]] && e2e_save_token machineRefreshToken "$REFRESH"
  [[ -n "$MID" ]] && e2e_set_data machineId "$MID"
  [[ -n "$ORG" ]] && e2e_set_data organizationId "$ORG"
  [[ -n "$SITE" ]] && e2e_set_data siteId "$SITE"
  export MACHINE_TOKEN="$ACCESS"
  export MACHINE_ID="$MID"
fi

_rt="$(get_secret machineRefreshToken 2>/dev/null || true)"
if [[ -n "${_rt:-}" ]] && [[ "${ec}" -eq 0 ]]; then
  export MACHINE_ID="$(get_data machineId)"
  export MACHINE_TOKEN="$(get_secret machineToken 2>/dev/null || true)"
  RBODY="$(jq -nc --arg r "$_rt" '{refreshToken:$r}')"
  grpc_contract_try_unauth "$FLOW_ID" "refresh-public" MachineTokenService RefreshMachineToken "$RBODY" "g20-refresh-public" "g20-rt-pub" || ec=1
  _nr="$(jq -r '.accessToken // empty' "${E2E_RUN_DIR}/grpc/g20-refresh-public.response.json" 2>/dev/null)"
  [[ -n "$_nr" ]] && e2e_save_token machineToken "$_nr" && export MACHINE_TOKEN="$_nr"
  _rt2="$(get_secret machineRefreshToken 2>/dev/null || true)"
  [[ -n "${_rt2:-}" ]] && _rt="$_rt2"
  ABODY="$(jq -nc --arg r "$_rt" '{refresh:{refreshToken:$r}}')"
  grpc_contract_try "$FLOW_ID" "refresh-auth-wrapped" MachineAuthService RefreshMachineToken "$ABODY" "g20-refresh-auth" "g20-rt-auth" || ec=1
  _nr2="$(jq -r '.refresh.accessToken // empty' "${E2E_RUN_DIR}/grpc/g20-refresh-auth.response.json" 2>/dev/null)"
  [[ -n "$_nr2" ]] && e2e_save_token machineToken "$_nr2"
fi

SCEN="20_grpc_machine_auth.sh"
if [[ "${GRPC_USE_REFLECTION:-false}" != "true" ]]; then
  log_docs_gap "P2" "$FLOW_ID" "$SCEN" "grpc-entry" "gRPC" "${GRPC_ADDR:-}" "gRPC uses proto files from GRPC_PROTO_ROOT when reflection disabled — document standard dev ports/paths" "Integration friction" "Publish connection guide; enable reflection in non-prod" "${E2E_RUN_DIR}/grpc/g20-claim.meta.json"
fi
if [[ "${ec}" -eq 0 ]]; then
  e2e_flow_review_scenario_complete "$FLOW_ID" "$SCEN" "flow-review-complete" "grpc_auth_claim_path_ok"
fi

exit "${ec}"
