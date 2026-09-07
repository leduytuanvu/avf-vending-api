#!/usr/bin/env bash
# Correlate client QR latency evidence with server metrics (single run folder).
#
# Usage:
#   SSH_HOST=root@72.62.244.94 APP_NODE_DIR=/opt/avf/app-node \
#     ./scripts/production/correlate-qr-payment-latency.sh
# Optional client logcat file:
#   CLIENT_LOGCAT=/path/to/qr-timing.txt ./scripts/production/correlate-qr-payment-latency.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-.production-latency-runs}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${OUTPUT_DIR}/qr-correlate-${TS}"
mkdir -p "${RUN_DIR}"

export OUTPUT_DIR="${RUN_DIR}"
export SSH_HOST="${SSH_HOST:-}"
export APP_NODE_DIR="${APP_NODE_DIR:-}"
export PUBLIC_URL="${PUBLIC_URL:-https://api.ldtv.dev}"

echo "=== stack signals ===" | tee "${RUN_DIR}/correlate.log"
"${ROOT}/scripts/production/measure-production-stack.sh" 2>&1 | tee -a "${RUN_DIR}/correlate.log"

echo "=== create payment session logs ===" | tee -a "${RUN_DIR}/correlate.log"
"${ROOT}/scripts/production/measure-create-payment-session.sh" 2>&1 | tee -a "${RUN_DIR}/correlate.log"

echo "=== pool + momo ===" | tee -a "${RUN_DIR}/correlate.log"
"${ROOT}/scripts/production/verify-pool-and-momo.sh" 2>&1 | tee -a "${RUN_DIR}/correlate.log"

if [[ -n "${CLIENT_LOGCAT:-}" && -f "${CLIENT_LOGCAT}" ]]; then
  echo "=== client logcat parse ===" | tee -a "${RUN_DIR}/correlate.log"
  grep -E 'QR_PAYMENT_TIMING|PAYMENT_RAIL_DECISION|GRPC_CREATE_PAYMENT_SESSION_' "${CLIENT_LOGCAT}" \
    >"${RUN_DIR}/client-timing.txt" || true
  cat "${RUN_DIR}/client-timing.txt" | tee -a "${RUN_DIR}/correlate.log"
fi

{
  echo "# QR latency correlation run"
  echo "# timestamp=${TS}"
  echo ""
  echo "Compare:"
  echo "  - client GRPC_CREATE_PAYMENT_SESSION_RESULT durationMs"
  echo "  - server CREATE_PAYMENT_SESSION_PHASES (psp_create_ms, total_ms)"
  echo "  - grpc_request_duration_seconds method=CreatePaymentSession"
  echo "  - avf_db_pool_acquired_conns vs max during the same window"
  echo "  - docker-stats.txt CPU for api container"
} >"${RUN_DIR}/README.txt"

echo "Correlation bundle: ${RUN_DIR}"
