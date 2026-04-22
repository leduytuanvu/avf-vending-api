#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

require_cmd ssh

DATA_NODE_HOST="${DATA_NODE_HOST:-}"
[[ -n "${DATA_NODE_HOST}" ]] || fail "set DATA_NODE_HOST"

REMOTE_ROOT="${PRODUCTION_DEPLOY_ROOT:-/opt/avf-vending-api}"
REMOTE_DIR="${DATA_NODE_REMOTE_DIR:-${REMOTE_ROOT}/deployments/prod/data-node}"

if [[ -n "${SSH_PORT:-}" ]]; then
	SSH_OPTS="-p ${SSH_PORT} ${SSH_OPTS:-}"
fi

target="$(ssh_target "${DATA_NODE_HOST}")"

note "release data-node ${DATA_NODE_HOST}"
run_remote_script "${target}" "${REMOTE_DIR}" "scripts/release_data_node.sh"
note "verify data-node ${DATA_NODE_HOST}"
run_remote_script "${target}" "${REMOTE_DIR}" "scripts/healthcheck_data_node.sh"

echo "release_data_node: PASS"
