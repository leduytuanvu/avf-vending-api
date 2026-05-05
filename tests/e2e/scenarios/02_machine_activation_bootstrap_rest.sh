#!/usr/bin/env bash
# shellcheck shell=bash
# VM-REST-02: Public activation claim + machine-scoped bootstrap (REST mirror of app bootstrap).
# Production field app uses gRPC/MQTT; this is lab/QA only.

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

FLOW_ID="VM-REST-02"
VA_LOG="${E2E_RUN_DIR}/reports/va-rest-results.jsonl"
mkdir -p "${E2E_RUN_DIR}/reports"

va_record() {
  local step="$1" endpoint="$2" status="$3" http="$4" msg="$5"
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  jq -nc \
    --arg ts "$(now_utc)" \
    --arg flow "$FLOW_ID" \
    --arg step "$step" \
    --arg ep "$endpoint" \
    --arg st "$status" \
    --argjson http "${http:-0}" \
    --arg msg "$msg" \
    '{ts:$ts,flow_id:$flow,step:$step,endpoint:$ep,status:$st,httpStatus:$http,message:$msg}' >>"$VA_LOG"
  e2e_append_test_event "$FLOW_ID" "$step" "REST" "$endpoint" "$status" "$msg" "{}"
}

ANY_FAIL=0

MID="$(get_data machineId)"
MT="$(get_secret machineToken 2>/dev/null || true)"
if [[ -n "${E2E_SKIP_ACTIVATION_CLAIM:-}" ]] && [[ "${E2E_SKIP_ACTIVATION_CLAIM}" == "1" ]] && [[ -n "$MT" ]] && [[ -n "$MID" ]] && [[ "$MID" != "null" ]]; then
  export ADMIN_TOKEN="$MT"
  va_record "activation-claim" "—" "skip" "0" "E2E_SKIP_ACTIVATION_CLAIM=1 and machineToken present"
elif [[ -n "$MT" ]] && [[ -n "$MID" ]] && [[ "$MID" != "null" ]]; then
  export ADMIN_TOKEN="$MT"
  va_record "activation-claim" "POST /v1/setup/activation-codes/claim" "skip" "0" "reuse secrets.machineToken + test-data.machineId"
else
  ACODE="${E2E_ACTIVATION_CODE:-}"
  [[ -z "$ACODE" ]] && ACODE="$(get_secret activationCodePlain 2>/dev/null || true)"
  if [[ -z "$ACODE" ]]; then
    log_error "VM-REST-02: set E2E_ACTIVATION_CODE or secrets.activationCodePlain, or reuse-data with machineToken"
    va_record "activation-claim" "POST /v1/setup/activation-codes/claim" "fail" "0" "no activation code"
    exit 2
  fi
  FP_SER="${E2E_DEVICE_FINGERPRINT_SERIAL:-e2e-serial-$(date +%s)-${RANDOM}}"
  CLAIM_BODY="$(jq -nc \
    --arg code "$ACODE" \
    --arg sn "$FP_SER" \
    '{
      activationCode:$code,
      deviceFingerprint:{
        serialNumber:$sn,
        androidId:("android-"+$sn),
        manufacturer:"E2E",
        model:"LabDevice",
        packageName:"com.avf.vending.e2e",
        versionName:"1.0.0",
        versionCode:1
      }
    }')"
  code="$(e2e_http_post_json_anon "vm-claim" "/v1/setup/activation-codes/claim" "$CLAIM_BODY")"
  if [[ "$code" != "200" ]]; then
    ANY_FAIL=1
    va_record "activation-claim" "POST /v1/setup/activation-codes/claim" "fail" "$code" "HTTP $code — see rest/vm-claim.response.json"
    exit "$ANY_FAIL"
  fi
  MID="$(e2e_jq_resp vm-claim -r '.machineId // empty')"
  MT="$(e2e_jq_resp vm-claim -r '.machineToken // empty')"
  OID="$(e2e_jq_resp vm-claim -r '.organizationId // empty')"
  SID="$(e2e_jq_resp vm-claim -r '.siteId // empty')"
  [[ -n "$MID" ]] || { log_error "VM-REST-02: no machineId in claim response"; exit 2; }
  [[ -n "$MT" ]] || { log_error "VM-REST-02: no machineToken in claim response"; exit 2; }
  save_token machineToken "$MT"
  e2e_set_data machineId "$MID"
  [[ -n "$OID" ]] && e2e_set_data organizationId "$OID"
  [[ -n "$SID" ]] && e2e_set_data siteId "$SID"
  export ADMIN_TOKEN="$MT"
  va_record "activation-claim" "POST /v1/setup/activation-codes/claim" "pass" "$code" "machineId=$MID"
fi

path="/v1/setup/machines/${MID}/bootstrap"
code="$(e2e_http_get "vm-bootstrap" "$path")"
if [[ "$code" != "200" ]]; then
  ANY_FAIL=1
  va_record "machine-bootstrap" "GET /v1/setup/machines/{id}/bootstrap" "fail" "$code" "HTTP $code"
  exit "$ANY_FAIL"
fi

# Persist fingerprints / hints when present (public test-data only — no secrets)
CFG_VER="$(jq -r '.runtimeHints.appliedMachineConfigRevision // .machine.commandSequence // empty' "${E2E_RUN_DIR}/rest/vm-bootstrap.response.json" 2>/dev/null | head -n1)"
CAT_N="$(jq -r '.catalog.products | length // 0' "${E2E_RUN_DIR}/rest/vm-bootstrap.response.json" 2>/dev/null)"
[[ -n "$CFG_VER" ]] && e2e_set_data bootstrapConfigRevision "$CFG_VER"
e2e_set_data bootstrapCatalogProductCount "${CAT_N:-0}"
va_record "machine-bootstrap" "GET /v1/setup/machines/{id}/bootstrap" "pass" "$code" "products=${CAT_N:-0} configRev=${CFG_VER:-n/a}"

exit 0
