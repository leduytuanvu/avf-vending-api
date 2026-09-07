#!/usr/bin/env bash
# Check DB pool pressure and MoMo/PSP create failures on the app node.
#
# Usage (SSH tunnel or on host):
#   OPS_METRICS_URL=http://127.0.0.1:8081/metrics ./scripts/production/verify-pool-and-momo.sh
#   SSH_HOST=root@72.62.244.94 APP_NODE_DIR=/opt/avf/app-node ./scripts/production/verify-pool-and-momo.sh
set -euo pipefail

OUTPUT_DIR="${OUTPUT_DIR:-.production-latency-runs}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${OUTPUT_DIR}/pool-momo-${TS}"
mkdir -p "${RUN_DIR}"

OPS_METRICS_URL="${OPS_METRICS_URL:-http://127.0.0.1:8081/metrics}"
SSH_HOST="${SSH_HOST:-}"
APP_NODE_DIR="${APP_NODE_DIR:-}"
LOG_TAIL="${LOG_TAIL:-300}"

fetch_metrics() {
  curl -fsS "${OPS_METRICS_URL}" -o "${RUN_DIR}/ops-metrics.txt" 2>"${RUN_DIR}/metrics-curl.err" || true
}

if [[ -n "${SSH_HOST}" ]]; then
  ssh "${SSH_HOST}" "curl -fsS http://127.0.0.1:8081/metrics" >"${RUN_DIR}/ops-metrics.txt" 2>"${RUN_DIR}/metrics-curl.err" || true
  ssh "${SSH_HOST}" "APP_NODE_DIR='${APP_NODE_DIR}' LOG_TAIL='${LOG_TAIL}' bash -s" <<'REMOTE' >"${RUN_DIR}/psp-errors.txt" 2>&1 || true
set -euo pipefail
if [[ -n "${APP_NODE_DIR}" && -d "${APP_NODE_DIR}" ]]; then
  cd "${APP_NODE_DIR}"
  docker compose logs --tail="${LOG_TAIL}" api | grep -E 'CREATE_PAYMENT_SESSION_PSP_ERROR|provider_timeout|Unavailable' || true
fi
REMOTE
else
  fetch_metrics
  if [[ -n "${APP_NODE_DIR}" && -d "${APP_NODE_DIR}" ]]; then
    (
      cd "${APP_NODE_DIR}"
      docker compose logs --tail="${LOG_TAIL}" api \
        | grep -E 'CREATE_PAYMENT_SESSION_PSP_ERROR|provider_timeout|Unavailable' \
        >"${RUN_DIR}/psp-errors.txt" || true
    )
  fi
fi

POOL_LINES="$(grep -E '^avf_db_pool_' "${RUN_DIR}/ops-metrics.txt" 2>/dev/null || true)"
GRPC_CPS="$(grep -E 'grpc_request_duration_seconds.*CreatePaymentSession|grpc_requests_total.*CreatePaymentSession' \
  "${RUN_DIR}/ops-metrics.txt" 2>/dev/null || true)"

{
  echo "# Pool + MoMo verification"
  echo "# generated=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo ""
  echo "## avf_db_pool_*"
  echo "${POOL_LINES:-<no metrics — set OPS_METRICS_URL or SSH_HOST>}"
  echo ""
  if [[ -n "${POOL_LINES}" ]]; then
    acquired="$(echo "${POOL_LINES}" | awk '/^avf_db_pool_acquired_conns /{print $2; exit}')"
    max="$(echo "${POOL_LINES}" | awk '/^avf_db_pool_max_conns /{print $2; exit}')"
    if [[ -n "${acquired}" && -n "${max}" && "${max}" != "0" ]]; then
      pct="$(awk -v a="${acquired}" -v m="${max}" 'BEGIN { printf "%.0f", (a/m)*100 }')"
      echo "pool_utilization_pct=${pct} (target <70%)"
    fi
  fi
  echo ""
  echo "## grpc CreatePaymentSession (raw series)"
  echo "${GRPC_CPS:-<none>}"
  echo ""
  echo "## Recent PSP errors (api logs)"
  if [[ -f "${RUN_DIR}/psp-errors.txt" ]]; then
    cat "${RUN_DIR}/psp-errors.txt"
  else
    echo "<no local APP_NODE_DIR logs>"
  fi
} | tee "${RUN_DIR}/summary.txt"

echo "Artifacts: ${RUN_DIR}"
