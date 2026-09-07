#!/usr/bin/env bash
# Extract CreatePaymentSession phase timings from API container logs.
#
# Usage (on app node):
#   APP_NODE_DIR=/opt/avf/app-node ./scripts/production/measure-create-payment-session.sh
# Usage (remote):
#   SSH_HOST=root@72.62.244.94 APP_NODE_DIR=/opt/avf/app-node ./scripts/production/measure-create-payment-session.sh
# Optional:
#   LOG_TAIL=500 ORDER_ID=<uuid> SINCE=30m
set -euo pipefail

OUTPUT_DIR="${OUTPUT_DIR:-.production-latency-runs}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${OUTPUT_DIR}/create-payment-session-${TS}"
mkdir -p "${RUN_DIR}"

APP_NODE_DIR="${APP_NODE_DIR:-}"
SSH_HOST="${SSH_HOST:-}"
LOG_TAIL="${LOG_TAIL:-400}"
SINCE="${SINCE:-}"
ORDER_ID="${ORDER_ID:-}"

collect_logs() {
  local dir="$1"
  (
    cd "${dir}"
    if [[ -n "${SINCE}" ]]; then
      docker compose logs --since="${SINCE}" api >"${RUN_DIR}/api-logs.txt" 2>&1
    else
      docker compose logs --tail="${LOG_TAIL}" api >"${RUN_DIR}/api-logs.txt" 2>&1
    fi
  )
}

if [[ -n "${SSH_HOST}" ]]; then
  ssh "${SSH_HOST}" "APP_NODE_DIR='${APP_NODE_DIR}' LOG_TAIL='${LOG_TAIL}' SINCE='${SINCE}' bash -s" <<'REMOTE' >"${RUN_DIR}/api-logs.txt" 2>&1 || true
set -euo pipefail
if [[ -n "${APP_NODE_DIR}" && -d "${APP_NODE_DIR}" ]]; then
  cd "${APP_NODE_DIR}"
  if [[ -n "${SINCE}" ]]; then
    docker compose logs --since="${SINCE}" api
  else
    docker compose logs --tail="${LOG_TAIL}" api
  fi
fi
REMOTE
elif [[ -n "${APP_NODE_DIR}" && -d "${APP_NODE_DIR}" ]]; then
  collect_logs "${APP_NODE_DIR}"
else
  echo "Set APP_NODE_DIR or SSH_HOST to collect api logs." >&2
  exit 1
fi

grep -E 'CREATE_PAYMENT_SESSION_(PHASES|SUCCESS|PSP_ERROR|START|DB_ERROR)' "${RUN_DIR}/api-logs.txt" \
  >"${RUN_DIR}/create-payment-session-events.txt" || true

if [[ -n "${ORDER_ID}" ]]; then
  grep "${ORDER_ID}" "${RUN_DIR}/create-payment-session-events.txt" \
    >"${RUN_DIR}/create-payment-session-order.txt" || true
fi

{
  echo "# CreatePaymentSession phase summary"
  echo "# run_dir=${RUN_DIR}"
  echo "# generated=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo ""
  grep 'CREATE_PAYMENT_SESSION_PHASES' "${RUN_DIR}/create-payment-session-events.txt" | tail -20 || true
  echo ""
  echo "# SUCCESS (duration_ms)"
  grep 'CREATE_PAYMENT_SESSION_SUCCESS' "${RUN_DIR}/create-payment-session-events.txt" | tail -20 || true
  echo ""
  echo "# PSP errors"
  grep 'CREATE_PAYMENT_SESSION_PSP_ERROR' "${RUN_DIR}/create-payment-session-events.txt" | tail -20 || true
} >"${RUN_DIR}/summary.txt"

cat "${RUN_DIR}/summary.txt"
echo "Artifacts: ${RUN_DIR}"
