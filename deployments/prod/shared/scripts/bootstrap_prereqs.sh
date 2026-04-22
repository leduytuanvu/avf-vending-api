#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

NODE_KIND="${1:-}"
[[ -n "${NODE_KIND}" ]] || fail "usage: bootstrap_prereqs.sh <app-node|data-node>"

case "${NODE_KIND}" in
app-node)
	NODE_ROOT="$(cd "${SHARED_ROOT}/../../app-node" && pwd)"
	ENV_FILE="${NODE_ROOT}/.env.app-node"
	COMPOSE_FILE="${NODE_ROOT}/docker-compose.app-node.yml"
	REQUIRED_DIRS=("${SHARED_ROOT}")
	REQUIRED_FILES=("${ENV_FILE}" "${COMPOSE_FILE}" "${SHARED_ROOT}/Caddyfile")
	REQUIRED_CMDS=(bash curl docker grep sed awk python3)
	;;
data-node)
	NODE_ROOT="$(cd "${SHARED_ROOT}/../../data-node" && pwd)"
	ENV_FILE="${NODE_ROOT}/.env.data-node"
	COMPOSE_FILE="${NODE_ROOT}/docker-compose.data-node.yml"
	REQUIRED_DIRS=("${SHARED_ROOT}/../emqx" "${SHARED_ROOT}/../emqx/certs")
	REQUIRED_FILES=("${ENV_FILE}" "${COMPOSE_FILE}" "${SHARED_ROOT}/../emqx/base.hocon")
	REQUIRED_CMDS=(bash curl docker grep sed awk jq)
	;;
*)
	fail "unknown node kind: ${NODE_KIND}"
	;;
esac

COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
init_state_dir

note "bootstrap prerequisite check (${NODE_KIND})"

for cmd in "${REQUIRED_CMDS[@]}"; do
	require_cmd "${cmd}"
done

docker info >/dev/null 2>&1 || fail "docker daemon is not reachable"
docker compose version >/dev/null 2>&1 || fail "docker compose plugin is not available"

for path in "${REQUIRED_FILES[@]}"; do
	require_file "${path}"
done
for path in "${REQUIRED_DIRS[@]}"; do
	require_dir "${path}"
done

if [[ "${NODE_KIND}" == "data-node" ]]; then
	load_env_file "${ENV_FILE}"
	if [[ "$(normalize_bool "${EMQX_SSL_ENABLED:-1}")" == "1" ]]; then
		require_file "${SHARED_ROOT}/../emqx/certs/ca.crt"
		require_file "${SHARED_ROOT}/../emqx/certs/server.crt"
		require_file "${SHARED_ROOT}/../emqx/certs/server.key"
	else
		warn "EMQX_SSL_ENABLED is disabled; plaintext MQTT must remain private-network-only and 1883 must not be exposed publicly"
	fi
fi

compose_config_or_fail

touch "${STATE_DIR}/.write-test"
rm -f "${STATE_DIR}/.write-test"

note "bootstrap prerequisite check passed (${NODE_KIND})"
