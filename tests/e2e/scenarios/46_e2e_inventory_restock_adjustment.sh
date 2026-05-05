#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 8 / E2E-46: Inventory / restock / adjustment — delegates to WA-INV-11.

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

SID="E2E-46-inventory-restock"
start_step "phase8-${SID}"

MID="$(get_data machineId)"
ORG="$(get_data organizationId)"
IDS_JSON="$(jq -nc --arg m "${MID:-}" --arg o "${ORG:-}" '{machineId:$m,organizationId:$o}')"
APIS_JSON='["GET /v1/admin/machines/{id}/topology","inventory snapshot/events","stock adjustments (WA-INV-11)"]'
EXPECTED="Topology/slots readable; restock or adjustment applied; snapshot reflects change when API exposes it."
EVID_JSON="$(jq -nc --arg f "${E2E_RUN_DIR}/reports/wa-module-results.jsonl" '[$f]')"

set +e
bash "${E2E_SCENARIO_DIR}/11_web_admin_inventory_ops.sh"
ec=$?
set -e

if [[ "$ec" -ne 0 ]]; then
  ACTUAL="WA-INV-11 exit ${ec} — see ${E2E_RUN_DIR}/reports/wa-module-results.jsonl and rest/inv-*.meta.json"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "ADMIN_TOKEN; operator session; planogramId; e2e-remediation Phase 4"
  log_api_contract_issue "P2" "$SID" "46_e2e_inventory_restock_adjustment.sh" "wa-inv-delegate" "mixed" "WA-INV-11" "Delegated web-admin inventory scenario failed — restock/adjustment flow not proven in Phase 8" "Inventory regression signal lost" "Fix 11_web_admin_inventory_ops prerequisites; see remediation in phase8 record" "${E2E_RUN_DIR}/reports/wa-module-results.jsonl"
  end_step failed "E2E-46: ${ACTUAL}"
  exit 1
fi

ACTUAL="wa_inv_11_ok"
phase8_record "$SID" "pass" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" ""
end_step passed "E2E-46 inventory / restock completed"
log_no_improvement_findings "$SID" "46_e2e_inventory_restock_adjustment.sh" "phase8-inventory-wrapper"
e2e_flow_review_scenario_complete "$SID" "46_e2e_inventory_restock_adjustment.sh" "flow-review-complete" "inventory_phase8_reviewed"
exit 0
