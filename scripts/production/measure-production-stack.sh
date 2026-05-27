#!/usr/bin/env bash
# Collect optional VPS/docker stack signals for production latency triage.
# SSH_* variables are optional; when unset, only local curl timings run.
#
# Usage (on app node with compose):
#   APP_NODE_DIR=/opt/avf/app-node PUBLIC_URL=https://api.ldtv.dev ./scripts/production/measure-production-stack.sh
# Usage (remote via SSH):
#   SSH_HOST=user@app1 PUBLIC_URL=https://api.ldtv.dev ./scripts/production/measure-production-stack.sh
set -euo pipefail

OUTPUT_DIR="${OUTPUT_DIR:-.production-latency-runs}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${OUTPUT_DIR}/stack-${TS}"
mkdir -p "${RUN_DIR}"

PUBLIC_URL="${PUBLIC_URL:-https://api.ldtv.dev}"
PUBLIC_URL="${PUBLIC_URL%/}"
LOCAL_URL="${LOCAL_URL:-http://127.0.0.1:8080}"
APP_NODE_DIR="${APP_NODE_DIR:-}"
SSH_HOST="${SSH_HOST:-}"

run_local() {
  echo "# local health timings" >"${RUN_DIR}/curl-local.txt"
  for path in /health/live /health/ready /version; do
    curl -sS -o /dev/null -w "${path} total=%{time_total} http_code=%{http_code}\n" "${LOCAL_URL}${path}" >>"${RUN_DIR}/curl-local.txt" || true
  done
  echo "# public health timings" >"${RUN_DIR}/curl-public.txt"
  for path in /health/live /health/ready /version; do
    curl -sS -o /dev/null -w "${path} total=%{time_total} http_code=%{http_code}\n" "${PUBLIC_URL}${path}" >>"${RUN_DIR}/curl-public.txt" || true
  done
}

collect_compose() {
  local dir="$1"
  (
    cd "${dir}"
    docker compose ps >"${RUN_DIR}/docker-ps.txt" 2>&1 || true
    docker stats --no-stream >"${RUN_DIR}/docker-stats.txt" 2>&1 || true
    docker compose logs --tail=200 api >"${RUN_DIR}/api-logs-tail.txt" 2>&1 || true
    docker compose logs --tail=100 caddy >"${RUN_DIR}/caddy-logs-tail.txt" 2>&1 || true
  )
}

if [[ -n "${SSH_HOST}" ]]; then
  ssh "${SSH_HOST}" "APP_NODE_DIR='${APP_NODE_DIR}' bash -s" <<'REMOTE' >"${RUN_DIR}/remote-collect.txt" 2>&1 || true
set -euo pipefail
if [[ -n "${APP_NODE_DIR}" && -d "${APP_NODE_DIR}" ]]; then
  cd "${APP_NODE_DIR}"
  echo "=== docker compose ps ==="
  docker compose ps || true
  echo "=== docker stats ==="
  docker stats --no-stream || true
  echo "=== api logs tail ==="
  docker compose logs --tail=200 api || true
  echo "=== caddy logs tail ==="
  docker compose logs --tail=100 caddy || true
fi
echo "=== localhost health ==="
curl -sS -o /dev/null -w "live total=%{time_total} code=%{http_code}\n" http://127.0.0.1:8080/health/live || true
curl -sS -o /dev/null -w "ready total=%{time_total} code=%{http_code}\n" http://127.0.0.1:8080/health/ready || true
REMOTE
fi

run_local

if [[ -n "${APP_NODE_DIR}" && -d "${APP_NODE_DIR}" ]]; then
  collect_compose "${APP_NODE_DIR}"
fi

echo "Stack evidence: ${RUN_DIR}"
