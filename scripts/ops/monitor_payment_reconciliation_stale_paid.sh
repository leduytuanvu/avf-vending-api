#!/usr/bin/env bash
# Poll admin payments reconciliation for stale paid/vending orders (post vend-outbox fix rollout).
#
# Usage:
#   export ADMIN_API_BASE="https://api.ldtv.dev"   # or your admin API host
#   export ADMIN_BEARER_TOKEN="<commerce:read or payment:read JWT>"
#   bash scripts/ops/monitor_payment_reconciliation_stale_paid.sh [machine_code] [stale_hours]
#
# Optional machine filter: pass machine code as $1; script filters JSON client-side with jq.

set -euo pipefail

BASE="${ADMIN_API_BASE:-https://api.ldtv.dev}"
TOKEN="${ADMIN_BEARER_TOKEN:-}"
MACHINE_FILTER="${1:-}"
STALE_HOURS="${2:-24}"
LIMIT="${RECONCILIATION_LIMIT:-100}"

if [[ -z "$TOKEN" ]]; then
  echo "Set ADMIN_BEARER_TOKEN (JWT with payment:read or commerce:read)" >&2
  exit 1
fi

URL="${BASE}/v1/admin/payments/reconciliation?stale_hours=${STALE_HOURS}&limit=${LIMIT}"
echo "GET ${URL}"

RESP="$(curl -fsS -H "Authorization: Bearer ${TOKEN}" -H "Accept: application/json" "$URL")"

if command -v jq >/dev/null 2>&1; then
  if [[ -n "$MACHINE_FILTER" ]]; then
    echo "$RESP" | jq --arg mc "$MACHINE_FILTER" '
      .stale_paid_orders? // .items? // . |
      (if type == "array" then . else .stale_paid_orders // [] end)
      | map(select(.machine_code == $mc or .machine?.code == $mc))
    '
  else
    echo "$RESP" | jq '.'
  fi
else
  echo "$RESP"
fi
