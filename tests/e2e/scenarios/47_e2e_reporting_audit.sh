#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 8 / E2E-47: Reporting / audit / finance — delegates to WA-RPT-10.

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

SID="E2E-47-reporting-audit"
start_step "phase8-${SID}"

MID="$(get_data machineId)"
ORG="$(get_data organizationId)"
IDS_JSON="$(jq -nc --arg m "${MID:-}" --arg o "${ORG:-}" '{machineId:$m,organizationId:$o}')"
APIS_JSON='["GET /v1/admin/finance/daily-close","GET .../commerce/reconciliation","GET /v1/admin/audit/events","GET .../artifacts"]'
EXPECTED="Reporting routes non-5xx or explicit skip on 403; finance/audit lists return JSON when permitted."
EVID_JSON="$(jq -nc --arg f "${E2E_RUN_DIR}/reports/wa-module-results.jsonl" '[$f]')"

set +e
bash "${E2E_SCENARIO_DIR}/10_reporting_audit_reconciliation.sh"
ec=$?
set -e

if [[ "$ec" -ne 0 ]]; then
  ACTUAL="WA-RPT-10 exit ${ec} — see ${E2E_RUN_DIR}/reports/wa-module-results.jsonl and rest/rpt-*.meta.json"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "ADMIN_TOKEN roles finance/audit.read; e2e-remediation Phase 4"
  end_step failed "E2E-47: ${ACTUAL}"
  exit 1
fi

ACTUAL="wa_rpt_10_ok"
phase8_record "$SID" "pass" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" ""
end_step passed "E2E-47 reporting / audit completed"
exit 0
