#!/usr/bin/env bash
# Post-deploy acceptance checks for layout/config consistency incident fix.
# Usage:
#   ADMIN_BEARER_TOKEN=... MACHINE_ID=01a089ec-... ./scripts/ops/e2e_layout_config_acceptance.sh
# Optional:
#   API_BASE=https://api.ldtv.dev LAYOUT_ID=01a096ed-... MACHINE_JWT=...
set -euo pipefail

MACHINE_ID="${MACHINE_ID:-01a089ec-c7bb-7e0d-83a9-6f599f061f12}"
LAYOUT_ID="${LAYOUT_ID:-01a096ed-f77b-75c3-ba0b-4eda79639029}"
API_BASE="${API_BASE:-https://api.ldtv.dev}"

pass=0
fail=0

check() {
  local name="$1"
  shift
  if "$@"; then
    echo "PASS: $name"
    pass=$((pass + 1))
  else
    echo "FAIL: $name" >&2
    fail=$((fail + 1))
  fi
}

echo "=== AVF layout/config acceptance (machine=${MACHINE_ID}) ==="

if [[ -z "${ADMIN_BEARER_TOKEN:-}" ]]; then
  echo "WARN: ADMIN_BEARER_TOKEN unset â€” admin API checks skipped" >&2
else
  library_json="$(curl -sS \
    -H "Authorization: Bearer ${ADMIN_BEARER_TOKEN}" \
    "${API_BASE}/v1/admin/machines/${MACHINE_ID}/layouts")"

  check "layout library returns activeLayoutId" \
    bash -c "echo '${library_json}' | jq -e '.activeLayoutId | length > 0' >/dev/null"

  check "commerce readiness needsReconcile=false" \
    bash -c "echo '${library_json}' | jq -e '.commerceReadiness.needsReconcile == false' >/dev/null"

  check "current slot configs present" \
    bash -c "echo '${library_json}' | jq -e '(.commerceReadiness.currentSlotConfigCount // 0) > 0' >/dev/null"

  history_json="$(curl -sS \
    -H "Authorization: Bearer ${ADMIN_BEARER_TOKEN}" \
    "${API_BASE}/v1/admin/machines/${MACHINE_ID}/layouts/${LAYOUT_ID}/history?limit=1&offset=0")"

  check "layout history endpoint reachable" \
    bash -c "echo '${history_json}' | jq -e '.items != null' >/dev/null"
fi

if [[ -n "${MACHINE_JWT:-}" ]]; then
  check "machine merge-pairs route not 404" \
    bash -c "code=\$(curl -sS -o /dev/null -w '%{http_code}' \
      -H \"Authorization: Bearer ${MACHINE_JWT}\" \
      \"${API_BASE}/v1/machines/${MACHINE_ID}/planogram/merge-pairs\"); \
      test \"\$code\" != '404'"
else
  echo "WARN: MACHINE_JWT unset â€” merge-pairs machine route check skipped" >&2
fi

echo
echo "=== Manual / device checks (record in ops ticket) ==="
cat <<EOF
[ ] Planogram publish (admin or device) returns 2xx â€” no slot_layout_not_found
[ ] Device log: no LAYOUT_SNAPSHOT_SKIP reason=missing_active_layout after bootstrap
[ ] Storefront checkout quote succeeds for configured product+slot
[ ] Payment dialog shows QR rails after order is created (MoMo/ZaloPay/VietQR)
[ ] Web layout history shows snapshot within periodic capture window (5â€“12 min)
EOF

echo
echo "Summary: pass=${pass} fail=${fail}"
if [[ "$fail" -gt 0 ]]; then
  exit 1
fi
