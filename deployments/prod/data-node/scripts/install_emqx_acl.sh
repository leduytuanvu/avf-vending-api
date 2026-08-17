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

reload_emqx_acl() {
	local cid="$1"
	if [[ -z "${cid}" ]]; then
		fail "emqx container id is empty"
	fi
	note "reload EMQX authorization without recreating the broker (cid=${cid})"
	# Bind-mounted acl.conf is already visible. Clear authz cache so file rules apply
	# to new connections without dropping existing MQTT sessions.
	if docker exec "${cid}" emqx ctl authorization cache-clean all; then
		return 0
	fi
	note "emqx ctl authorization cache-clean failed; trying conf reload"
	if docker exec "${cid}" emqx ctl conf reload; then
		return 0
	fi
	# ctl can fail with "Node emqx@emqx not responding to pings" while MQTT and the
	# management API are healthy. Do not --force-recreate; new connections still see the file.
	if curl -sf "http://127.0.0.1:18083/api/v5/status" | grep -Fq "emqx is running"; then
		note "EMQX management API is up; continuing without ctl reload"
		return 0
	fi
	fail "EMQX ACL reload failed"
}

install_via_compose() {
	local env_file="$1"
	local compose_file="$2"
	local service_name="${3:-emqx}"
	require_file "${env_file}"
	require_file "${compose_file}"
	local compose=(docker compose --env-file "${env_file}" -f "${compose_file}")
	note "validate compose ${compose_file}"
	"${compose[@]}" config >/dev/null
	# Start the broker if missing; never --force-recreate (that drops every kiosk MQTT session).
	if ! "${compose[@]}" ps --status running --services 2>/dev/null | grep -qx "${service_name}"; then
		note "start ${service_name} (no force-recreate)"
		"${compose[@]}" up -d "${service_name}"
	fi
	local cid
	cid="$("${compose[@]}" ps -q "${service_name}")"
	reload_emqx_acl "${cid}"
}

installed=0
if docker ps --format '{{.Names}}' | grep -qx 'avf-prod-emqx'; then
	legacy_env="${PROD_ROOT}/.env.production"
	legacy_compose="${PROD_ROOT}/docker-compose.prod.yml"
	if [[ -f "${legacy_env}" && -f "${legacy_compose}" ]]; then
		install_via_compose "${legacy_env}" "${legacy_compose}" emqx
		installed=1
	fi
fi

if [[ -f "${NODE_ROOT}/.env.data-node" && -f "${NODE_ROOT}/docker-compose.data-node.yml" ]]; then
	install_via_compose "${NODE_ROOT}/.env.data-node" "${NODE_ROOT}/docker-compose.data-node.yml" emqx
	installed=1
fi

if [[ "${installed}" -eq 0 ]]; then
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
