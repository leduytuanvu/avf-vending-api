#!/usr/bin/env bash
# shellcheck shell=bash
# GRPC-21: Bootstrap, catalog snapshot/delta/ack, media manifest/delta/ack.

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_grpc.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_grpc.sh"

FLOW_ID="GRPC-21"
ec=0

ORG="$(get_data organizationId)"
MID="$(get_data machineId)"
MT="$(get_secret machineToken 2>/dev/null || true)"
[[ -z "${MT:-}" ]] && { log_error "GRPC-21: machine token required"; exit 2; }
export MACHINE_TOKEN="$MT"
export MACHINE_ID="$MID"

META="$(jq -nc --arg o "$ORG" --arg m "$MID" --arg rid "g21-$(date +%s)" \
  '{organizationId:$o, machineId:$m, requestId:$rid}')"
BOOT_BODY="$(jq -nc --argjson meta "$META" '{meta:$meta}')"

grpc_contract_try "$FLOW_ID" "get-bootstrap" MachineBootstrapService GetBootstrap "$BOOT_BODY" "g21-bootstrap" "" || ec=1

BOOT_RESP="${E2E_RUN_DIR}/grpc/g21-bootstrap.response.json"
CFG_REV=""
PG_VER=""
if [[ -f "$BOOT_RESP" ]]; then
  CFG_REV="$(jq -r '.runtimeHints.appliedMachineConfigRevision // empty' "$BOOT_RESP")"
  PG_VER="$(jq -r '.publishedPlanogramVersionId // empty' "$BOOT_RESP")"
fi

CHK_BODY="$(jq -nc --argjson meta "$META" --arg boot "e2e-$(date +%s)" \
  '{meta:$meta, bootId:$boot, networkState:"online", attributes:{source:"e2e-grpc"}}')"
grpc_contract_try "$FLOW_ID" "check-in" MachineBootstrapService CheckIn "$CHK_BODY" "g21-checkin" "g21-checkin" || ec=1

if [[ -n "$CFG_REV" ]] && [[ "$CFG_REV" =~ ^-?[0-9]+$ ]]; then
  ACKC_BODY="$(jq -nc --argjson meta "$META" --argjson cr "$CFG_REV" --arg pg "$PG_VER" \
    '{meta:$meta, acknowledgedConfigVersion:$cr, acknowledgedPlanogramVersionId:$pg}')"
  grpc_contract_try "$FLOW_ID" "ack-config" MachineBootstrapService AckConfigVersion "$ACKC_BODY" "g21-ack-config" "g21-ackcfg" || ec=1
else
  grpc_contract_skip "$FLOW_ID" "ack-config" MachineBootstrapService AckConfigVersion "no_acknowledged_config_revision_from_bootstrap"
fi

CAT_BODY="$(jq -nc \
  --arg m "$MID" \
  --argjson meta "$META" \
  '{machineId:$m, includeUnavailable:true, includeImages:true, meta:$meta}')"
grpc_contract_try "$FLOW_ID" "get-sale-catalog" MachineCatalogService GetSaleCatalog "$CAT_BODY" "g21-sale-cat" "g21-sc" || ec=1

SNAP="${E2E_RUN_DIR}/grpc/g21-sale-cat.response.json"
BASIS=""
if [[ -f "$SNAP" ]] && jq -e '.snapshot.catalogVersion' "$SNAP" >/dev/null 2>&1; then
  BASIS="$(jq -r '.snapshot.catalogVersion // empty' "$SNAP")"
fi
[[ -z "$BASIS" ]] && BASIS="0"

DELTA_BODY="$(jq -nc --arg m "$MID" --arg b "$BASIS" --argjson meta "$META" \
  '{machineId:$m, basisCatalogVersion:$b, meta:$meta}')"
grpc_contract_try "$FLOW_ID" "get-catalog-delta" MachineCatalogService GetCatalogDelta "$DELTA_BODY" "g21-cat-delta" "g21-cd" || ec=1

ACK_CAT_VER="$BASIS"
if [[ -f "$SNAP" ]]; then
  _v="$(jq -r '.snapshot.catalogVersion // empty' "$SNAP")"
  [[ -n "$_v" ]] && ACK_CAT_VER="$_v"
fi
ACKCAT_BODY="$(jq -nc --argjson meta "$META" --arg v "$ACK_CAT_VER" \
  '{meta:$meta, acknowledgedCatalogVersion:$v}')"
grpc_contract_try "$FLOW_ID" "ack-catalog-version" MachineCatalogService AckCatalogVersion "$ACKCAT_BODY" "g21-ack-cat" "g21-ac" || ec=1

MAN_BODY="$(jq -nc --arg m "$MID" --argjson meta "$META" '{machineId:$m, includeUnavailable:true, meta:$meta}')"
grpc_contract_try "$FLOW_ID" "get-media-manifest-catalog-svc" MachineCatalogService GetMediaManifest "$MAN_BODY" "g21-media-man-cat" "g21-mmc" || ec=1

MMAN_BODY="$(jq -nc --arg m "$MID" --argjson meta "$META" '{machineId:$m, includeUnavailable:true, meta:$meta}')"
grpc_contract_try "$FLOW_ID" "get-media-manifest-media-svc" MachineMediaService GetMediaManifest "$MMAN_BODY" "g21-media-man-media" "g21-mmm" || ec=1

MFP="$(jq -r '.mediaFingerprint // empty' "${E2E_RUN_DIR}/grpc/g21-media-man-media.response.json" 2>/dev/null)"
[[ -z "$MFP" ]] && MFP="$(jq -r '.mediaFingerprint // empty' "${E2E_RUN_DIR}/grpc/g21-media-man-cat.response.json" 2>/dev/null)"
[[ -z "$MFP" ]] && MFP="initial"

MDEL_BODY="$(jq -nc --arg m "$MID" --arg f "$MFP" --argjson meta "$META" \
  '{machineId:$m, basisMediaFingerprint:$f, meta:$meta, includeUnavailable:true}')"
grpc_contract_try "$FLOW_ID" "get-media-delta" MachineMediaService GetMediaDelta "$MDEL_BODY" "g21-media-delta" "g21-md" || ec=1

NEXT_FP="$(jq -r '.nextSyncToken // empty' "${E2E_RUN_DIR}/grpc/g21-media-delta.response.json" 2>/dev/null)"
[[ -n "$NEXT_FP" ]] && MFP="$NEXT_FP"
ACKM_BODY="$(jq -nc --argjson meta "$META" --arg f "$MFP" '{meta:$meta, acknowledgedMediaFingerprint:$f}')"
grpc_contract_try "$FLOW_ID" "ack-media-version" MachineMediaService AckMediaVersion "$ACKM_BODY" "g21-ack-media" "g21-am" || ec=1

exit "${ec}"
