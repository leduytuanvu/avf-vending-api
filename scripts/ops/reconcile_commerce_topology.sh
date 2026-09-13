#!/usr/bin/env bash
# Idempotent commerce topology repair for machines missing cabinet/slot_layout/current configs.
# Usage:
#   ./scripts/ops/reconcile_commerce_topology.sh --machine-id 01a089ec-c7bb-7e0d-83a9-6f599f061f12 [--dry-run] [--api-base https://api.ldtv.dev]
set -euo pipefail

MACHINE_ID=""
DRY_RUN=false
API_BASE="${API_BASE:-https://api.ldtv.dev}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --machine-id) MACHINE_ID="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    --api-base) API_BASE="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$MACHINE_ID" ]]; then
  echo "Usage: $0 --machine-id <uuid> [--dry-run] [--api-base URL]" >&2
  exit 1
fi

echo "=== Read-only diagnostics (run against production DB separately) ==="
cat <<EOF
SELECT id, code, active_layout_id FROM machines WHERE id = '${MACHINE_ID}';
SELECT count(*) AS cabinets FROM machine_cabinets WHERE machine_id = '${MACHINE_ID}';
SELECT count(*) AS slot_layouts FROM machine_slot_layouts WHERE machine_id = '${MACHINE_ID}';
SELECT count(*) AS current_configs FROM machine_slot_configs WHERE machine_id = '${MACHINE_ID}' AND is_current = true;
SELECT count(*) AS snapshots FROM machine_layout_snapshot_history WHERE machine_id = '${MACHINE_ID}';
EOF

if [[ -z "${ADMIN_BEARER_TOKEN:-}" ]]; then
  echo "Set ADMIN_BEARER_TOKEN to call admin reconcile API." >&2
  exit 1
fi

BODY='{}'
if [[ "$DRY_RUN" == "true" ]]; then
  BODY='{"dryRun":true}'
fi

echo "=== POST ${API_BASE}/v1/admin/machines/${MACHINE_ID}/commerce-topology/reconcile dryRun=${DRY_RUN} ==="
reconcile_json="$(curl -sS -X POST \
  -H "Authorization: Bearer ${ADMIN_BEARER_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "${BODY}" \
  "${API_BASE}/v1/admin/machines/${MACHINE_ID}/commerce-topology/reconcile")"
echo "${reconcile_json}" | jq .

if [[ "$DRY_RUN" == "true" ]]; then
  echo "Dry-run complete — re-run without --dry-run to apply."
  exit 0
fi

echo "=== Verify commerce readiness via layout library ==="
library_json="$(curl -sS \
  -H "Authorization: Bearer ${ADMIN_BEARER_TOKEN}" \
  "${API_BASE}/v1/admin/machines/${MACHINE_ID}/layouts")"
echo "${library_json}" | jq '.commerceReadiness, .activeLayoutId'

needs_reconcile="$(echo "${library_json}" | jq -r '.commerceReadiness.needsReconcile // true')"
if [[ "${needs_reconcile}" == "true" ]]; then
  echo "WARN: commerceReadiness.needsReconcile is still true after reconcile" >&2
  exit 2
fi

echo "Commerce topology reconcile verified for machine ${MACHINE_ID}."
echo "Next: run scripts/ops/e2e_layout_config_acceptance.sh with the same credentials."
