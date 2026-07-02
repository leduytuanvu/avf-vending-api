#!/usr/bin/env bash
# Install per-machine EMQX ACL from repo templates and restart the broker container.
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROD_ROOT="$(cd "${NODE_ROOT}/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

EMQX_DIR="${PROD_ROOT}/emqx"
ACL_FILE="${EMQX_DIR}/acl.conf"

require_file "${EMQX_DIR}/base.hocon"
require_file "${EMQX_DIR}/acl.conf.example"

if [[ ! -f "${ACL_FILE}" ]]; then
	note "render acl.conf from acl.conf.example"
	sed 's/TOPIC_PREFIX/avf\/devices/g; s/%u/${username}/g' "${EMQX_DIR}/acl.conf.example" >"${ACL_FILE}"
fi

install_via_compose() {
	local env_file="$1"
	local compose_file="$2"
	local service_name="${3:-emqx}"
	require_file "${env_file}"
	require_file "${compose_file}"
	local compose=(docker compose --env-file "${env_file}" -f "${compose_file}")
	note "validate compose ${compose_file}"
	"${compose[@]}" config >/dev/null
	note "recreate ${service_name} from ${compose_file}"
	"${compose[@]}" up -d --force-recreate "${service_name}"
}

if docker ps --format '{{.Names}}' | grep -qx 'avf-prod-emqx'; then
	legacy_env="${PROD_ROOT}/.env.production"
	legacy_compose="${PROD_ROOT}/docker-compose.prod.yml"
	if [[ -f "${legacy_env}" && -f "${legacy_compose}" ]]; then
		install_via_compose "${legacy_env}" "${legacy_compose}" emqx
	else
		note "avf-prod-emqx running but legacy env/compose missing; restarting container"
		docker restart avf-prod-emqx
	fi
elif [[ -f "${NODE_ROOT}/.env.data-node" && -f "${NODE_ROOT}/docker-compose.data-node.yml" ]]; then
	install_via_compose "${NODE_ROOT}/.env.data-node" "${NODE_ROOT}/docker-compose.data-node.yml" emqx
else
	fail "no supported EMQX deployment found (expected avf-prod-emqx or data-node compose)"
fi

note "wait for EMQX management API"
for i in $(seq 1 60); do
	if curl -sf "http://127.0.0.1:18083/api/v5/status" | grep -Fq "emqx is running"; then
		echo "install_emqx_acl: PASS"
		exit 0
	fi
	sleep 2
done

fail "EMQX did not become ready after ACL install"
