#!/usr/bin/env bash
# Probe EMQX_MANAGEMENT_URL from the running API container (or host curl fallback).
# Never prints EMQX_API_SECRET.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NODE_ROOT="$(cd "${SCRIPT_DIR}/../../app-node" && pwd)"
ENV_FILE="${APP_NODE_ENV_FILE:-${APP_NODE_ROOT}/.env.app-node}"
COMPOSE_FILE="${APP_NODE_COMPOSE_FILE:-${APP_NODE_ROOT}/docker-compose.app-node.yml}"
TIMEOUT_SECS="${EMQX_PROBE_TIMEOUT_SECS:-8}"
REQUIRE_OK="${EMQX_PROBE_REQUIRE_OK:-0}"

fail() {
	echo "error: $*" >&2
	exit 1
}

note() {
	echo "==> $*"
}

[[ -f "${ENV_FILE}" ]] || fail "env file not found: ${ENV_FILE}"

mgmt_url="$(grep -E '^EMQX_MANAGEMENT_URL=' "${ENV_FILE}" | tail -n 1 | cut -d= -f2- || true)"
mgmt_url="${mgmt_url%/}"
echo "EMQX_MANAGEMENT_URL_PRESENT=$([ -n "${mgmt_url}" ] && echo true || echo false)"
if [[ -n "${mgmt_url}" ]]; then
	echo "EMQX_MANAGEMENT_URL=${mgmt_url}"
fi

probe_status() {
	local target="$1"
	curl -sS --max-time "${TIMEOUT_SECS}" -o /tmp/avf-emqx-status.txt -w "%{http_code}" "${target}/api/v5/status" || true
}

cid=""
if [[ -f "${COMPOSE_FILE}" ]]; then
	cid="$(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" ps -q api 2>/dev/null || true)"
fi
if [[ -z "${cid}" ]]; then
	cid="$(docker ps --format '{{.ID}} {{.Names}}' | awk '/api/{print $1; exit}')"
fi

http_code=""
if [[ -n "${cid}" ]]; then
	note "probe from api container id=${cid}"
	docker exec "${cid}" sh -c 'printf "container_EMQX_MANAGEMENT_URL=%s\n" "${EMQX_MANAGEMENT_URL:-}"'
	container_url="$(docker exec "${cid}" sh -c 'printf %s "${EMQX_MANAGEMENT_URL:-}"')"
	container_url="${container_url%/}"
	if [[ -z "${container_url}" ]]; then
		container_url="${mgmt_url}"
	fi
	http_code="$(docker exec "${cid}" sh -c "curl -sS --max-time ${TIMEOUT_SECS} -o /tmp/avf-emqx-status.txt -w '%{http_code}' '${container_url}/api/v5/status'" || true)"
	docker exec "${cid}" sh -c 'head -c 200 /tmp/avf-emqx-status.txt 2>/dev/null || true' || true
	echo
else
	note "api container not found; probing from host"
	[[ -n "${mgmt_url}" ]] || fail "EMQX_MANAGEMENT_URL empty"
	http_code="$(probe_status "${mgmt_url}")"
	head -c 200 /tmp/avf-emqx-status.txt 2>/dev/null || true
	echo
fi

echo "EMQX_STATUS_HTTP_CODE=${http_code:-000}"
if [[ "${http_code}" == "200" ]]; then
	note "probe_emqx_management_from_api: ok"
	exit 0
fi
echo "warn: EMQX management probe failed (timeout, refused, or non-200)" >&2
if [[ "${REQUIRE_OK}" == "1" ]]; then
	fail "EMQX management API unreachable from API container"
fi
exit 0
