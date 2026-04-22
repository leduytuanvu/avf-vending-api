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

EXIT_CODE_DEPLOY_FAILURE=51
EXIT_CODE_READINESS_FAILURE=52

if [[ -n "${SSH_PORT:-}" ]]; then
	SSH_OPTS="-p ${SSH_PORT} ${SSH_OPTS:-}"
fi

target="$(ssh_target "${DATA_NODE_HOST}")"

note "release data-node ${DATA_NODE_HOST}"
append_release_evidence "data-node" "${DATA_NODE_HOST}" "deploy" "running" "starting data-node deploy"
if ! run_remote_script "${target}" "${REMOTE_DIR}" "scripts/release_data_node.sh"; then
	append_release_evidence "data-node" "${DATA_NODE_HOST}" "deploy" "fail" "data-node deploy failed"
	exit "${EXIT_CODE_DEPLOY_FAILURE}"
fi
append_release_evidence "data-node" "${DATA_NODE_HOST}" "deploy" "pass" "data-node deploy completed"
note "verify data-node ${DATA_NODE_HOST}"
append_release_evidence "data-node" "${DATA_NODE_HOST}" "readiness" "running" "starting data-node readiness verification"
if ! run_remote_script "${target}" "${REMOTE_DIR}" "scripts/healthcheck_data_node.sh"; then
	append_release_evidence "data-node" "${DATA_NODE_HOST}" "readiness" "fail" "data-node readiness verification failed"
	exit "${EXIT_CODE_READINESS_FAILURE}"
fi
append_release_evidence "data-node" "${DATA_NODE_HOST}" "readiness" "pass" "data-node readiness verification passed"

echo "release_data_node: PASS"
