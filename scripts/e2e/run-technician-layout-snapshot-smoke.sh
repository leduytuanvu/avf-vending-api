#!/usr/bin/env bash
# Read-only smoke: list layout snapshot history for a machine (staging/production).
# Usage: MACHINE_ID=<uuid> BASE_URL=https://api.example.com ADMIN_TOKEN=... ./run-technician-layout-snapshot-smoke.sh
set -Eeuo pipefail

MACHINE_ID="${MACHINE_ID:-}"
BASE_URL="${BASE_URL:-}"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"

if [[ -z "$MACHINE_ID" || -z "$BASE_URL" || -z "$ADMIN_TOKEN" ]]; then
  echo "error: set MACHINE_ID, BASE_URL, and ADMIN_TOKEN" >&2
  exit 1
fi

url="${BASE_URL%/}/v1/admin/machines/${MACHINE_ID}/layout-history?limit=5"
code="$(curl -sS -o /tmp/avf-layout-history.json -w '%{http_code}' \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Accept: application/json" \
  "$url")"

echo "layout-history http=${code}"
if [[ "$code" != "200" ]]; then
  cat /tmp/avf-layout-history.json >&2 || true
  exit 1
fi

if command -v jq >/dev/null 2>&1; then
  jq -e '.items | type == "array"' /tmp/avf-layout-history.json >/dev/null
  tech="$(jq -r '[.items[]?.snapshotReason] | map(select(. == "TECHNICIAN_COMMIT")) | length' /tmp/avf-layout-history.json)"
  echo "technician_commit_count=${tech}"
fi

echo "PASS"
