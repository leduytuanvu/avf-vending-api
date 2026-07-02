#!/usr/bin/env bash
# Install per-machine EMQX ACL from repo templates and restart the broker container.
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE="${NODE_ROOT}/.env.data-node"
COMPOSE_FILE="${NODE_ROOT}/docker-compose.data-node.yml"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
EMQX_DIR="$(cd "${NODE_ROOT}/../emqx" && pwd)"
ACL_FILE="${EMQX_DIR}/acl.conf"

require_file "${ENV_FILE}"
require_file "${COMPOSE_FILE}"
require_file "${EMQX_DIR}/base.hocon"
require_file "${EMQX_DIR}/acl.conf.example"

if [[ ! -f "${ACL_FILE}" ]]; then
	note "render acl.conf from acl.conf.example"
	sed 's/TOPIC_PREFIX/avf\/devices/g' "${EMQX_DIR}/acl.conf.example" >"${ACL_FILE}"
fi

note "validate data-node compose with ACL mounts"
"${COMPOSE[@]}" config >/dev/null

note "recreate EMQX to load ACL + authorization config"
"${COMPOSE[@]}" up -d --force-recreate emqx

note "wait for EMQX management API"
for i in $(seq 1 60); do
	if curl -sf "http://127.0.0.1:18083/api/v5/status" | grep -Fq "emqx is running"; then
		echo "install_emqx_acl: PASS"
		exit 0
	fi
	sleep 2
done

fail "EMQX did not become ready after ACL install"
