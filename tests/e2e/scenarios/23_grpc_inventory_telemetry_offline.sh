#!/usr/bin/env bash
# shellcheck shell=bash
# GRPC-23: Inventory delta/snapshot, telemetry batch / critical event, offline sync, sync cursor, reconcile.

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_grpc.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_grpc.sh"

FLOW_ID="GRPC-23"
ec=0

ORG="$(get_data organizationId)"
MID="$(get_data machineId)"
MT="$(get_secret machineToken 2>/dev/null || true)"
[[ -z "${MT:-}" ]] && { log_error "GRPC-23: machine token required"; exit 2; }
export MACHINE_TOKEN="$MT"
export MACHINE_ID="$MID"

META="$(jq -nc --arg o "$ORG" --arg m "$MID" --arg rid "g23-$(date +%s)" \
  '{organizationId:$o, machineId:$m, requestId:$rid}')"

TS="$(date +%s)"
DELTA_BODY="$(jq -nc \
  --arg ik "g23-delta-${TS}" \
  --arg ce "g23-ce-d-${TS}" \
  --argjson meta "$META" \
  '{context:{idempotencyKey:$ik, clientEventId:$ce}, meta:$meta, reason:"machine_reconcile", lines:[]}')"
grpc_contract_try "$FLOW_ID" "push-inventory-delta" MachineInventoryService PushInventoryDelta "$DELTA_BODY" "g23-inv-delta" "g23-delta-${TS}" || ec=1

SNAP_BODY="$(jq -nc --argjson meta "$META" '{meta:$meta}')"
grpc_contract_try "$FLOW_ID" "get-inventory-snapshot" MachineInventoryService GetInventorySnapshot "$SNAP_BODY" "g23-inv-snap" "" || ec=1

TS2="$(date +%s)"
TEL_BODY="$(jq -nc \
  --arg ik "g23-tb-${TS2}" \
  --argjson meta "$META" \
  --arg eid "g23-ev-${TS2}" \
  '{context:{idempotencyKey:$ik}, meta:$meta, events:[{eventId:$eid, eventType:"e2e.ping", occurredAt:"2024-01-01T00:00:00Z", bootId:"e2e-boot", clientSequence:1}]}')"
grpc_contract_try "$FLOW_ID" "push-telemetry-batch" MachineTelemetryService PushTelemetryBatch "$TEL_BODY" "g23-tel-batch" "g23-tb-${TS2}" || ec=1

TS3="$(date +%s)"
CRIT_BODY="$(jq -nc \
  --arg ik "g23-ce-${TS3}" \
  --argjson meta "$META" \
  --arg eid "g23-crit-${TS3}" \
  '{context:{idempotencyKey:$ik}, meta:$meta,
    event:{eventId:$eid, eventType:"e2e.critical", occurredAt:"2024-01-01T00:00:01Z"},
    severity:"warn"}')"
grpc_contract_try "$FLOW_ID" "push-critical-event" MachineTelemetryService PushCriticalEvent "$CRIT_BODY" "g23-crit" "g23-ce-${TS3}" || ec=1

OFF_BODY="$(jq -nc --argjson meta "$META" '{meta:$meta, events:[]}')"
grpc_contract_try "$FLOW_ID" "push-offline-events" MachineOfflineSyncService PushOfflineEvents "$OFF_BODY" "g23-offline" "" || ec=1

CUR_BODY="$(jq -nc --argjson meta "$META" '{meta:$meta}')"
grpc_contract_try "$FLOW_ID" "get-sync-cursor" MachineOfflineSyncService GetSyncCursor "$CUR_BODY" "g23-cursor" "" || ec=1

REC_BODY='{"idempotencyKeys":[]}'
grpc_contract_try "$FLOW_ID" "reconcile-events" MachineTelemetryService ReconcileEvents "$REC_BODY" "g23-recon" "" || ec=1

SCEN="23_grpc_inventory_telemetry_offline.sh"
log_offline_sync_issue "P2" "$FLOW_ID" "$SCEN" "sync-cursor" "gRPC" "GetSyncCursor/PushOfflineEvents" "Harness sends empty offline bundle — strict sequence/cursor/idempotency of client_event_id not fully proven here" "Duplicate or gap risk in production" "Add tests that assert duplicate client_event_id returns idempotent replay signal" "${E2E_RUN_DIR}/grpc/g23-cursor.response.json"
if [[ "${GRPC_USE_REFLECTION:-false}" != "true" ]]; then
  log_docs_gap "P2" "$FLOW_ID" "$SCEN" "grpc-entry" "gRPC" "${GRPC_ADDR:-}" "Offline/telemetry methods need documented metadata (authorization, x-machine-id)" "Client misconfiguration" "Document required gRPC metadata matrix" "${E2E_RUN_DIR}/grpc/g23-inv-delta.meta.json"
fi
if [[ "${ec}" -eq 0 ]]; then
  e2e_flow_review_scenario_complete "$FLOW_ID" "$SCEN" "flow-review-complete" "grpc_inventory_telemetry_ok"
fi

exit "${ec}"
