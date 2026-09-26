#!/usr/bin/env bash
# Correlate QR/MoMo NOT_FOUND failures between client order ids and API logs + DB.
#
# Usage (on app-node with docker):
#   ORDER_ID=47b4baf6-ee80-4074-aafe-4723a00b5a7d \
#   PAYMENT_ID=01a0ded7-16b8-7422-8df8-d5d3544d1310 \
#   ./scripts/production/correlate-qr-not-found.sh
#
# Remote via SSH:
#   SSH_HOST=root@72.62.244.94 APP_NODE_DIR=/opt/avf/app-node \
#     ORDER_ID=... PAYMENT_ID=... ./scripts/production/correlate-qr-not-found.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ORDER_ID="${ORDER_ID:-}"
PAYMENT_ID="${PAYMENT_ID:-}"
SSH_HOST="${SSH_HOST:-}"
APP_NODE_DIR="${APP_NODE_DIR:-/opt/avf/app-node}"
OUTPUT_DIR="${OUTPUT_DIR:-.production-latency-runs}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${OUTPUT_DIR}/qr-not-found-${TS}"
mkdir -p "${RUN_DIR}"

if [[ -z "${ORDER_ID}" ]]; then
  echo "ORDER_ID is required" >&2
  exit 1
fi

run_remote() {
  if [[ -n "${SSH_HOST}" ]]; then
    ssh "${SSH_HOST}" "$1"
  else
    bash -lc "$1"
  fi
}

API_CONTAINER="$(run_remote "docker ps --format '{{.Names}}' | grep -E 'api|avf-vending-api' | head -1" || true)"
LOG_CMD="docker logs --since 24h \"${API_CONTAINER}\" 2>&1 | grep -E '${ORDER_ID}|${PAYMENT_ID}|CREATE_PAYMENT_SESSION|GET_ORDER_STATUS_NOT_FOUND|GET_PAYMENT_STATUS' || true"

{
  echo "# QR NOT_FOUND correlation"
  echo "# timestamp=${TS}"
  echo "# order_id=${ORDER_ID}"
  echo "# payment_id=${PAYMENT_ID}"
  echo "# ssh_host=${SSH_HOST:-local}"
  echo "# api_container=${API_CONTAINER:-unknown}"
  echo ""
  echo "=== API log grep (24h) ==="
} | tee "${RUN_DIR}/correlate.log"

if [[ -n "${API_CONTAINER}" ]]; then
  run_remote "cd '${APP_NODE_DIR}' && ${LOG_CMD}" | tee -a "${RUN_DIR}/api-grep.txt"
else
  echo "WARN: api container not found" | tee -a "${RUN_DIR}/correlate.log"
fi

DB_QUERY="SELECT id, machine_id, status, created_at FROM orders WHERE id = '${ORDER_ID}'::uuid;"
if [[ -n "${PAYMENT_ID}" ]]; then
  DB_QUERY="${DB_QUERY}
SELECT id, order_id, provider, state, created_at FROM payments WHERE id = '${PAYMENT_ID}'::uuid;"
fi

{
  echo ""
  echo "=== DB lookup ==="
  echo "${DB_QUERY}"
} | tee -a "${RUN_DIR}/correlate.log"

run_remote "cd '${APP_NODE_DIR}' && docker compose exec -T db psql -U avf -d avf_vending_prod -c \"${DB_QUERY}\"" \
  2>&1 | tee -a "${RUN_DIR}/db-lookup.txt" || true

{
  echo ""
  echo "Interpretation:"
  echo "  - CREATE_PAYMENT_SESSION without GET_ORDER_STATUS_SUCCESS -> client/server order id mismatch or stale poll id"
  echo "  - GET_ORDER_STATUS_NOT_FOUND with order row present -> access/machine_id mismatch"
  echo "  - No order row -> order never persisted or wrong environment"
} | tee -a "${RUN_DIR}/correlate.log"

echo "Correlation bundle: ${RUN_DIR}"
