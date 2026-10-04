#!/usr/bin/env bash
# Tail api/worker logs on a remote app-node (via SSH from app-node A runner).
set -Eeuo pipefail

TARGET="${1:-}"
PRODUCTION_DEPLOY_ROOT="${PRODUCTION_DEPLOY_ROOT:-/opt/avf-vending-api}"
SSH_PORT="${SSH_PORT:-22}"
TAIL_LINES="${APP_NODE_LOG_TAIL_LINES:-120}"

[[ -n "${TARGET}" ]] || { echo "collect_app_node_remote_logs: error: target host required" >&2; exit 1; }

read -r -a ssh_opts <<< "${SSH_OPTS:--o BatchMode=yes}"
remote_dir="${PRODUCTION_DEPLOY_ROOT}/deployments/prod/app-node"

echo "collect_app_node_remote_logs: target=${TARGET} tail=${TAIL_LINES}"

ssh "${ssh_opts[@]}" -p "${SSH_PORT}" "${TARGET}" bash -s -- "${remote_dir}" "${TAIL_LINES}" <<'REMOTE'
set -Eeuo pipefail
remote_dir="$1"
tail_lines="$2"
cd "${remote_dir}"
compose=(docker compose --env-file .env.app-node -f docker-compose.app-node.yml)
for svc in api worker reconciler mqtt-ingest caddy; do
  echo "=== ${svc} ps ==="
  "${compose[@]}" ps "${svc}" 2>/dev/null || true
  echo "=== ${svc} logs (tail ${tail_lines}) ==="
  "${compose[@]}" logs --tail "${tail_lines}" "${svc}" 2>&1 || true
done
echo "=== api /health/ready probe ==="
"${compose[@]}" exec -T api sh -c 'curl -sS -o /tmp/h.txt -w "%{http_code}" http://127.0.0.1:8080/health/ready; echo; head -c 200 /tmp/h.txt' 2>&1 || true
REMOTE
