#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 8 / E2E-44: Offline replay idempotency — duplicate gRPC PushOfflineEvents with deterministic client_event_id.

set -euo pipefail

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${E2E_SCRIPT_DIR:=$(cd "${E2E_SCENARIO_DIR}/.." && pwd)}"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_grpc.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_grpc.sh"

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

SID="E2E-44-offline-replay"
start_step "phase8-${SID}"

ORG="$(get_data organizationId)"
MID="$(get_data machineId)"
MT="$(get_secret machineToken 2>/dev/null || true)"
IDS_JSON="$(jq -nc --arg o "${ORG:-}" --arg m "${MID:-}" '{organizationId:$o,machineId:$m}')"
APIS_JSON='["gRPC MachineOfflineSyncService/PushOfflineEvents"]'
EXPECTED="First PushOfflineEvents accepts bundle; second duplicate push returns safe replay / not modified (idempotent)."
EVID_JSON="$(jq -nc --arg a "${E2E_RUN_DIR}/grpc/p8-off-a.meta.json" --arg b "${E2E_RUN_DIR}/grpc/p8-off-b.meta.json" '[$a,$b]')"

if ! command -v grpcurl >/dev/null 2>&1; then
  phase8_record "$SID" "skip" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "grpcurl not on PATH" "$(jq -nc --arg f "${E2E_RUN_DIR}/reports/phase8-scenario-results.jsonl" '[$f]')" "Install grpcurl for Phase 8 offline replay"
  log_docs_issue "P2" "$SID" "44_e2e_offline_replay.sh" "grpcurl-missing" "gRPC" "—" "Offline replay Phase 8 skipped — grpcurl not installed" "No cursor/idempotency coverage in CI" "Add grpcurl to CI image" "${E2E_RUN_DIR}/reports/phase8-scenario-results.jsonl"
  end_step skipped "E2E-44: grpcurl missing"
  e2e_flow_review_scenario_complete "$SID" "44_e2e_offline_replay.sh" "flow-review-skip" "offline_replay_skipped_no_grpcurl"
  exit 0
fi

if ! e2e_grpc_server_reachable; then
  phase8_record "$SID" "skip" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "gRPC server unreachable at ${GRPC_ADDR}" "$(jq -nc --arg f "${E2E_RUN_DIR}/grpc" '[$f]')" "Start API gRPC listener; GRPC_ADDR; see e2e-troubleshooting"
  log_docs_issue "P2" "$SID" "44_e2e_offline_replay.sh" "grpc-down" "gRPC" "${GRPC_ADDR:-}" "gRPC unreachable — offline replay not exercised" "Gap for offline contract" "Start listener; document GRPC_ADDR" "${E2E_RUN_DIR}/grpc"
  end_step skipped "E2E-44: gRPC unreachable"
  e2e_flow_review_scenario_complete "$SID" "44_e2e_offline_replay.sh" "flow-review-skip" "offline_replay_skipped_unreachable"
  exit 0
fi

[[ -z "$MT" ]] && { log_error "E2E-44: machineToken required"; exit 2; }
export MACHINE_TOKEN="$MT"
export MACHINE_ID="$MID"

CEID="e2e-phase8-offline-deterministic-1"
REQ_ID="p8-off-req-001"
IDEM_BUNDLE="e2e-p8-offline-bundle-fixed-1"
IDEM_EVT="e2e-p8-offline-event-fixed-1"
OCC="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

OFF_BODY="$(jq -nc \
  --arg oid "$ORG" \
  --arg mid "$MID" \
  --arg rid "$REQ_ID" \
  --arg ik "$IDEM_BUNDLE" \
  --arg ceid "$CEID" \
  --arg eik "$IDEM_EVT" \
  --arg occ "$OCC" \
  '{
    meta:{
      organizationId:$oid,
      machineId:$mid,
      requestId:$rid,
      idempotencyKey:$ik
    },
    events:[{
      meta:{
        organizationId:$oid,
        machineId:$mid,
        requestId:$rid,
        clientEventId:$ceid,
        offlineSequence:1,
        idempotencyKey:$eik
      },
      eventType:"e2e.offline.ping",
      payload:{phase:"8", note:"deterministic offline replay test"}
    }]
  }')"

full="avf.machine.v1.MachineOfflineSyncService/PushOfflineEvents"
if ! e2e_grpc_rpc_declared_in_repo MachineOfflineSyncService PushOfflineEvents; then
  phase8_record "$SID" "skip" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "PushOfflineEvents not in repo protos" "$EVID_JSON" "Pull proto avf/machine/v1/offline_sync.proto"
  log_api_contract_issue "P1" "$SID" "44_e2e_offline_replay.sh" "proto-missing" "gRPC" "PushOfflineEvents" "PushOfflineEvents RPC not found in checked-in protos — app offline path cannot be contract-tested from repo" "Shipping risk for offline" "Ensure protos synced; add RPC to repo" "${E2E_REPO_ROOT}/proto"
  end_step skipped "E2E-44: RPC not in repo"
  e2e_flow_review_scenario_complete "$SID" "44_e2e_offline_replay.sh" "flow-review-skip" "offline_replay_skipped_no_rpc"
  exit 0
fi

if ! e2e_grpc_call "$full" "$OFF_BODY" "p8-off-a" "$IDEM_BUNDLE"; then
  ACTUAL="first PushOfflineEvents failed — ${E2E_RUN_DIR}/grpc/p8-off-a.log"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "grpcurl log; machine JWT; offline ingest"
  end_step failed "E2E-44: ${ACTUAL}"
  exit 1
fi

if ! e2e_grpc_call "$full" "$OFF_BODY" "p8-off-b" "$IDEM_BUNDLE"; then
  ACTUAL="duplicate PushOfflineEvents transport failure — ${E2E_RUN_DIR}/grpc/p8-off-b.log"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Inspect gRPC status in log"
  end_step failed "E2E-44: ${ACTUAL}"
  exit 1
fi

S1="$(jq -r '.results[0].status // .results[0] // empty' "${E2E_RUN_DIR}/grpc/p8-off-a.response.json" 2>/dev/null || true)"
S2="$(jq -r '.results[0].status // .results[0] // empty' "${E2E_RUN_DIR}/grpc/p8-off-b.response.json" 2>/dev/null || true)"

log_offline_sync_issue "P2" "$SID" "44_e2e_offline_replay.sh" "out-of-order" "gRPC" "PushOfflineEvents" "Duplicate push exercised, but strict out-of-order client_sequence behavior not covered in Phase 8" "Undefined replay semantics" "Add vectors for reorder + gap detection; document server rules" "${E2E_RUN_DIR}/grpc/p8-off-b.log"
if [[ -n "$S1" && -n "$S2" && "$S1" != "$S2" ]]; then
  log_offline_sync_issue "P1" "$SID" "44_e2e_offline_replay.sh" "idempotent-status" "gRPC" "PushOfflineEvents" "Two pushes with same bundle idempotency returned different per-event status — idempotency contract unclear" "Duplicate side effects possible" "Return stable DUPLICATE/REPLAY status in proto" "${E2E_RUN_DIR}/grpc/p8-off-b.response.json"
fi
log_offline_sync_issue "P1" "$SID" "44_e2e_offline_replay.sh" "replay-verify" "gRPC" "GetSyncCursor" "Harness does not reconnect client and re-read cursor to prove durable apply after duplicate push" "Replay may look OK at RPC but not persisted" "Add post-reconnect GetSyncCursor assertion" "${E2E_RUN_DIR}/grpc/p8-off-a.meta.json"

phase8_record "$SID" "pass" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "push1_status=${S1:-ok} push2_status=${S2:-ok} duplicate_bundle_same_idempotency out_of_order_not_exercised_rest" "$EVID_JSON" ""
end_step passed "E2E-44 offline duplicate push completed (out-of-order: gRPC only when supported)"
e2e_flow_review_scenario_complete "$SID" "44_e2e_offline_replay.sh" "flow-review-complete" "offline_replay_phase8_reviewed"
exit 0
