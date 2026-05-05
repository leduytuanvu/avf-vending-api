#!/usr/bin/env bash
# shellcheck shell=bash
# GRPC-24: OTA/update assignment, status report, diagnostic bundle result.

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_grpc.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_grpc.sh"

FLOW_ID="GRPC-24"
ec=0

ORG="$(get_data organizationId)"
MID="$(get_data machineId)"
MT="$(get_secret machineToken 2>/dev/null || true)"
[[ -z "${MT:-}" ]] && { log_error "GRPC-24: machine token required"; exit 2; }
export MACHINE_TOKEN="$MT"
export MACHINE_ID="$MID"

META="$(jq -nc --arg o "$ORG" --arg m "$MID" --arg rid "g24-$(date +%s)" \
  '{organizationId:$o, machineId:$m, requestId:$rid}')"

GU_BODY="$(jq -nc --argjson meta "$META" '{meta:$meta}')"
grpc_contract_try "$FLOW_ID" "get-assigned-update" MachineCommandService GetAssignedUpdate "$GU_BODY" "g24-assigned" "" || ec=1

CID="$(jq -r '.update.campaignId // empty' "${E2E_RUN_DIR}/grpc/g24-assigned.response.json" 2>/dev/null)"
if [[ -n "${CID:-}" ]]; then
  RUS_BODY="$(jq -nc --argjson meta "$META" --arg c "$CID" \
    '{meta:$meta, campaignId:$c, status:"downloaded", errorMessage:""}')"
  grpc_contract_try "$FLOW_ID" "report-update-status" MachineCommandService ReportUpdateStatus "$RUS_BODY" "g24-rpt-upd" "g24-rus" || ec=1
else
  grpc_contract_skip "$FLOW_ID" "report-update-status" MachineCommandService ReportUpdateStatus "no_assigned_update_in_get_assigned_response"
fi

DIAG_BODY="$(jq -nc --argjson meta "$META" \
  '{meta:$meta, requestId:"e2e-grpc-diag-req", storageKey:"e2e/diag/object",
    storageProvider:"test", contentType:"application/zip", sizeBytes:1, sha256Hex:"00"}')"
grpc_contract_try "$FLOW_ID" "report-diagnostic-bundle" MachineCommandService ReportDiagnosticBundleResult "$DIAG_BODY" "g24-diag" "g24-diag" || ec=1

BID="$(jq -r '.bundleId // empty' "${E2E_RUN_DIR}/grpc/g24-diag.response.json" 2>/dev/null)"
[[ -n "${BID:-}" ]] && e2e_set_data grpcDiagnosticBundleId "$BID"

exit "${ec}"
