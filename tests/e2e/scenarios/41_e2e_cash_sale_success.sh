#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 8 / E2E-41: Cash sale success — delegates to VM-REST-04; optional audit list probe.

set -euo pipefail

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

phase8_record() {
  local scenario_id="$1" result="$2" ids_json="$3" apis_json="$4" expected_state="$5" actual_state="$6" evidence_json="$7" remediation="$8"
  mkdir -p "${E2E_RUN_DIR}/reports"
  jq -nc \
    --arg ts "$(now_utc)" \
    --arg scenario_id "$scenario_id" \
    --arg result "$result" \
    --argjson input_ids "$ids_json" \
    --argjson apis_topics_used "$apis_json" \
    --arg expected_state "$expected_state" \
    --arg actual_state "$actual_state" \
    --argjson evidence_files "$evidence_json" \
    --arg remediation "$remediation" \
    '{ts:$ts,scenario_id:$scenario_id,input_ids:$input_ids,apis_topics_used:$apis_topics_used,expected_state:$expected_state,actual_state:$actual_state,result:$result,evidence_files:$evidence_files,remediation:$remediation}' \
    >>"${E2E_RUN_DIR}/reports/phase8-scenario-results.jsonl"
}

SID="E2E-41-cash-sale-success"
start_step "phase8-${SID}"

MID="$(get_data machineId)"
ORG="$(get_data organizationId)"
PRODUCT_ID="$(get_data productId)"
IDS_JSON="$(jq -nc --arg m "${MID:-}" --arg o "${ORG:-}" --arg p "${PRODUCT_ID:-}" '{machineId:$m,organizationId:$o,productId:$p}')"
APIS_JSON='["POST /v1/commerce/cash-checkout","POST /v1/commerce/orders/{id}/vend/start","POST /v1/commerce/orders/{id}/vend/success","GET /v1/commerce/orders/{id}","GET /v1/machines/{id}/sale-catalog"]'
EXPECTED="Paid order; vend success; inventory decremented (when catalog exposes qty); optional audit non-5xx."
EVID_JSON="$(jq -nc \
  --arg a "${E2E_RUN_DIR}/rest/vm-cash-co.meta.json" \
  --arg b "${E2E_RUN_DIR}/rest/vm-vend-ok.meta.json" \
  --arg c "${E2E_RUN_DIR}/rest/vm-order-final.meta.json" '[$a,$b,$c]')"

if ! bash "${E2E_SCENARIO_DIR}/04_cash_sale_success_rest.sh"; then
  ACTUAL="VM-REST-04 failed — inspect ${E2E_RUN_DIR}/rest/vm-cash-co.response.json (HTTP and error envelope) and vm-cash-co.meta.json"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "See docs/testing/e2e-remediation-playbook.md (inventory insufficient / machine token)"
  end_step failed "E2E-41: ${ACTUAL}"
  exit 1
fi

AUDIT_NOTE="audit_skip_no_admin"
ADM="$(get_secret adminAccessToken 2>/dev/null || true)"
[[ -z "$ADM" ]] && ADM="${E2E_ADMIN_TOKEN:-}"
if [[ -n "$ADM" ]] && [[ -n "$ORG" && "$ORG" != "null" ]]; then
  export ADMIN_TOKEN="$ADM"
  Q_ORG="organization_id=$(printf '%s' "$ORG" | jq -sRr @uri)"
  code_a="$(e2e_http_get "p8-41-audit" "/v1/admin/audit/events?${Q_ORG}&limit=5&machineId=${MID}")"
  APIS_JSON="$(echo "$APIS_JSON" | jq -c '. + ["GET /v1/admin/audit/events"]')"
  EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/rest/p8-41-audit.meta.json" '. + [$f]')"
  if [[ "$code_a" =~ ^5 ]] || [[ "$code_a" == "0" ]]; then
    AUDIT_NOTE="audit_http_${code_a}_partial"
  elif [[ "$code_a" == "200" ]]; then
    AUDIT_NOTE="audit_http_200"
  else
    AUDIT_NOTE="audit_http_${code_a}_optional"
  fi
fi

ACTUAL="cash_sale_vm_rest_04_ok ${AUDIT_NOTE}"
phase8_record "$SID" "pass" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" ""
end_step passed "E2E-41 cash sale success completed"
exit 0
