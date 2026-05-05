#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 8 / E2E-43: Vend failure + refund path — delegates to VM-REST-06.

set -euo pipefail

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
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

SID="E2E-43-vend-failure-refund"
start_step "phase8-${SID}"

MID="$(get_data machineId)"
ORG="$(get_data organizationId)"
IDS_JSON="$(jq -nc --arg m "${MID:-}" --arg o "${ORG:-}" '{machineId:$m,organizationId:$o}')"
APIS_JSON='["POST /v1/commerce/cash-checkout","POST .../vend/start","POST .../vend/failure","GET .../orders/{id}","POST .../refunds"]'
EXPECTED="Paid order; vend failure recorded; refund POST accepted or explicitly skipped with HTTP reason; final GET order state."
EVID_JSON="$(jq -nc \
  --arg a "${E2E_RUN_DIR}/rest/vm-fail-vfail.meta.json" \
  --arg b "${E2E_RUN_DIR}/rest/vm-fail-refund.meta.json" \
  --arg c "${E2E_RUN_DIR}/rest/vm-fail-order2.meta.json" '[$a,$b,$c]')"

if ! bash "${E2E_SCENARIO_DIR}/06_vend_failure_refund_rest.sh"; then
  ACTUAL="VM-REST-06 exited non-zero — see ${E2E_RUN_DIR}/rest/vm-fail-co.meta.json and vm-fail-vfail.response.json"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Review commerce policy; E2E_ALLOW_WRITES; inventory; e2e-remediation-playbook"
  end_step failed "E2E-43: ${ACTUAL}"
  exit 1
fi

ACTUAL="vend_failure_refund_vm_rest_06_ok (refund may be skip row in va-rest log if route returns non-200)"
phase8_record "$SID" "pass" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" ""
log_flow_design_issue "P1" "$SID" "43_e2e_vend_failure_refund.sh" "refund-inventory-parity" "REST+gRPC" "vend failure + refund" "Phase 8 does not exhaustively prove inventory + payment ledger invariants after vend failure and refund across services" "Residual risk if server-side guards incomplete" "Add contract tests for forbidden refund states; assert inventory restoration" "${E2E_RUN_DIR}/rest/vm-fail-order2.meta.json"
end_step passed "E2E-43 vend failure / refund completed"
e2e_flow_review_scenario_complete "$SID" "43_e2e_vend_failure_refund.sh" "flow-review-complete" "vend_failure_refund_phase8_reviewed"
exit 0
