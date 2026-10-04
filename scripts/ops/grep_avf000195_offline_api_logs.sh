#!/usr/bin/env bash
# Read-only: recent API container logs for AVF000195 offline push / insert errors.
set -Eeuo pipefail

MACHINE_ID="${MACHINE_ID:-01a0a7e5-3c68-7895-b526-bcb6504bccfb}"
ORDER_ID="${ORDER_ID:-54a327c2-3ecd-4014-b90c-121fed479cd6}"
SINCE="${SINCE:-6h}"
TAIL_LINES="${TAIL_LINES:-200}"

find_api_container() {
  docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

container="$(find_api_container)"
if [[ -z "${container}" ]]; then
  echo "grep-avf000195-offline: error: no api container" >&2
  exit 1
fi

echo "grep-avf000195-offline: container=${container} since=${SINCE}"
pattern="(${MACHINE_ID}|${ORDER_ID}|offline event insert|MACHINE_OFFLINE_INSERT|PushOfflineEvents|offline-sale:${ORDER_ID})"
docker logs "${container}" --since "${SINCE}" 2>&1 \
  | grep -E "${pattern}" \
  | tail -n "${TAIL_LINES}" \
  || echo "grep-avf000195-offline: no matching log lines in window"
