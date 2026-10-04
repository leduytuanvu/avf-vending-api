#!/usr/bin/env bash
# Report running API /version (git commit) on local app-node or via SSH.
set -Eeuo pipefail

LABEL="${1:-local}"
TARGET="${2:-}"

remote_dir="${PRODUCTION_DEPLOY_ROOT:-/opt/avf-vending-api}/deployments/prod/app-node"
version_cmd="cd '${remote_dir}' && docker compose --env-file .env.app-node -f docker-compose.app-node.yml exec -T api curl -fsS http://127.0.0.1:8080/version"

fetch_version() {
  if [[ -n "${TARGET}" ]]; then
    read -r -a ssh_opts <<< "${SSH_OPTS:--o BatchMode=yes}"
    ssh "${ssh_opts[@]}" -p "${SSH_PORT:-22}" "${TARGET}" "${version_cmd}"
  else
    bash -lc "${version_cmd}"
  fi
}

echo "inspect-api-git-sha: label=${LABEL}"
if ! body="$(fetch_version 2>&1)"; then
  echo "inspect-api-git-sha: error: ${body}" >&2
  exit 1
fi
echo "${body}"
if command -v jq >/dev/null 2>&1; then
  jq -r '{version,commit,build_time}' <<<"${body}" 2>/dev/null || true
fi
