#!/usr/bin/env bash
# Read-only verification for planogram leave-sync evidence on production (or any env).
#
# Usage:
#   ./scripts/ops/verify-planogram-leave-sync-server.sh \
#     --machine-id 01a0a7e5-3c68-7895-b526-bcb6504bccfb \
#     [--layout-id 01a0a7e5-3caf-7380-a5d0-f0d14a5cf6dc] \
#     [--expected-revision 2] \
#     [--snapshot-id b3b43ee9-0bdd-460b-b6ff-4a996ba8093f] \
#     [--expected-capture-sequence 2] \
#     [--api-base https://api.ldtv.dev]
#
# With ADMIN_USERNAME+ADMIN_PASSWORD (or ADMIN_BEARER_TOKEN), also queries admin layout library.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../e2e/lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

MACHINE_ID=""
LAYOUT_ID=""
EXPECTED_REVISION=""
SNAPSHOT_ID=""
EXPECTED_SEQUENCE=""
API_BASE="${API_BASE:-https://api.ldtv.dev}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --machine-id) MACHINE_ID="$2"; shift 2 ;;
    --layout-id) LAYOUT_ID="$2"; shift 2 ;;
    --expected-revision) EXPECTED_REVISION="$2"; shift 2 ;;
    --snapshot-id) SNAPSHOT_ID="$2"; shift 2 ;;
    --expected-capture-sequence) EXPECTED_SEQUENCE="$2"; shift 2 ;;
    --api-base) API_BASE="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$MACHINE_ID" ]]; then
  echo "Usage: $0 --machine-id <uuid> [options]" >&2
  exit 1
fi

echo "=== Planogram leave-sync server audit (read-only SQL) ==="
cat <<EOF
-- Machine + active layout
SELECT id, code, active_layout_id FROM machines WHERE id = '${MACHINE_ID}';

-- Layout revision (machine_layouts.layout_revision)
SELECT id, machine_id, layout_key, layout_revision, updated_at
FROM machine_layouts
WHERE machine_id = '${MACHINE_ID}'
ORDER BY updated_at DESC
LIMIT 5;

-- Device-reported generation (local machine state ack path)
SELECT machine_id, layout_id, device_generation, latest_capture_sequence, reported_at
FROM machine_layout_device_state
WHERE machine_id = '${MACHINE_ID}';

-- Snapshot history (newest first)
SELECT snapshot_id, layout_id, capture_sequence, snapshot_reason, captured_at
FROM machine_layout_snapshot_history
WHERE machine_id = '${MACHINE_ID}'
ORDER BY captured_at DESC
LIMIT 10;

-- Storefront catalog source (GetCatalogSnapshot / machine_slot_configs)
SELECT COUNT(*) AS assigned_configs
FROM machine_slot_configs msc
JOIN machine_cabinets mc ON mc.id = msc.machine_cabinet_id
WHERE mc.machine_id = '${MACHINE_ID}'
  AND msc.is_current = true
  AND msc.product_id IS NOT NULL;

-- Named layout plane (admin GetMachineLayoutDetail)
SELECT COUNT(*) AS assigned_layout_slots
FROM machine_layout_slots mls
JOIN machine_layouts ml ON ml.id = mls.layout_id
WHERE ml.machine_id = '${MACHINE_ID}'
  AND mls.product_id IS NOT NULL;
EOF

if [[ -n "$LAYOUT_ID" ]]; then
  cat <<EOF

-- Selected layout only
SELECT snapshot_id, capture_sequence, snapshot_reason, captured_at
FROM machine_layout_snapshot_history
WHERE machine_id = '${MACHINE_ID}' AND layout_id = '${LAYOUT_ID}'
ORDER BY capture_sequence DESC
LIMIT 5;
EOF
fi

if [[ -n "$SNAPSHOT_ID" ]]; then
  cat <<EOF

-- Specific snapshot from device log
SELECT snapshot_id, machine_id, layout_id, capture_sequence, snapshot_reason, captured_at
FROM machine_layout_snapshot_history
WHERE snapshot_id = '${SNAPSHOT_ID}';
EOF
fi

if [[ -n "$EXPECTED_REVISION" ]]; then
  echo ""
  echo "Expected layout_revision (from device PLANOGRAM_LEAVE_LAYOUT_SYNC_RESULT): ${EXPECTED_REVISION}"
fi
if [[ -n "$EXPECTED_SEQUENCE" ]]; then
  echo "Expected capture_sequence (from TECH_SNAPSHOT_CAPTURED): ${EXPECTED_SEQUENCE}"
fi

if [[ -z "${ADMIN_BEARER_TOKEN:-}" ]]; then
  : "${ADMIN_USERNAME:=${E2E_PROD_ADMIN_USERNAME:-}}"
  : "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-${ADMIN_PASSWORD:-}}}"
  if [[ -n "${ADMIN_USERNAME:-}" && -n "${ADMIN_PASSWORD:-}" ]]; then
    ADMIN_BEARER_TOKEN="$(e2e_admin_token)" || true
  fi
fi

if [[ -z "${ADMIN_BEARER_TOKEN:-}" ]]; then
  echo ""
  echo "Admin API: skipped (set ADMIN_BEARER_TOKEN or ADMIN_USERNAME+ADMIN_PASSWORD to verify via REST)."
  exit 0
fi

export BASE_URL="${API_BASE%/}"
e2e_require_cmd curl jq

echo ""
echo "=== GET ${API_BASE}/v1/admin/machines/${MACHINE_ID}/layouts ==="
library_json="$(curl -sS \
  -H "Authorization: Bearer ${ADMIN_BEARER_TOKEN}" \
  "${API_BASE}/v1/admin/machines/${MACHINE_ID}/layouts")"
echo "${library_json}" | jq '{activeLayoutId, commerceReadiness, layouts: [.layouts[]? | {id, layoutKey, layoutRevision, updatedAt}]}'

if [[ -n "$LAYOUT_ID" ]]; then
  echo ""
  echo "=== GET layout detail ${LAYOUT_ID} ==="
  detail_json="$(curl -sS \
    -H "Authorization: Bearer ${ADMIN_BEARER_TOKEN}" \
    "${API_BASE}/v1/admin/machines/${MACHINE_ID}/layouts/${LAYOUT_ID}")"
  echo "${detail_json}" | jq '{id, layoutRevision, slotCount: (.slots | length), historyCount: (.snapshotHistory | length)?}'
fi
