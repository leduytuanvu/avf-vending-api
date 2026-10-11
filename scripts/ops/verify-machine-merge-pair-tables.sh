#!/usr/bin/env bash
# Read-only check: lane-level merges vs named-layout merge rows (post ghép / layout_pull QA).
# Usage: ./scripts/ops/verify-machine-merge-pair-tables.sh --machine-id <uuid> [--layout-id <uuid>]
set -euo pipefail

MACHINE_ID=""
LAYOUT_ID=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --machine-id) MACHINE_ID="$2"; shift 2 ;;
    --layout-id) LAYOUT_ID="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$MACHINE_ID" ]]; then
  echo "machine-id required" >&2
  exit 1
fi

PSQL="${PSQL:-psql}"
DB_URL="${DATABASE_URL:-}"

if [[ -z "$DB_URL" ]]; then
  echo "Set DATABASE_URL for postgres" >&2
  exit 1
fi

echo "=== machine_lane_merge_pairs (active) machine_id=$MACHINE_ID ==="
"$PSQL" "$DB_URL" -v ON_ERROR_STOP=1 -c "
SELECT left_slot_code, right_slot_code, layout_key, layout_revision, revision, merged_at
FROM machine_lane_merge_pairs
WHERE machine_id = '$MACHINE_ID'::uuid AND split_at IS NULL
ORDER BY left_slot_code;
"

if [[ -n "$LAYOUT_ID" ]]; then
  echo "=== machine_layout_merge_pairs layout_id=$LAYOUT_ID ==="
  "$PSQL" "$DB_URL" -v ON_ERROR_STOP=1 -c "
SELECT left_slot_code, right_slot_code
FROM machine_layout_merge_pairs
WHERE layout_id = '$LAYOUT_ID'::uuid
ORDER BY left_slot_code;
"
else
  echo "=== machine_layout_merge_pairs (all layouts for machine) ==="
  "$PSQL" "$DB_URL" -v ON_ERROR_STOP=1 -c "
SELECT m.layout_id, ml.name, p.left_slot_code, p.right_slot_code
FROM machine_layout_merge_pairs p
JOIN machine_layouts ml ON ml.id = p.layout_id
WHERE ml.machine_id = '$MACHINE_ID'::uuid
ORDER BY ml.name, p.left_slot_code;
"
fi
