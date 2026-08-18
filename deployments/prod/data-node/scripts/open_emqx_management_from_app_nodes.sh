#!/usr/bin/env bash
# Bind EMQX management :18083 off host loopback so app-node API containers can
# provision per-machine MQTT users. Do not open :18083 to the public internet:
# UFW allows only APP_NODE_A_HOST / APP_NODE_B_HOST.
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROD_ROOT="$(cd "${NODE_ROOT}/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

APP_NODE_A_HOST="${APP_NODE_A_HOST:-}"
APP_NODE_B_HOST="${APP_NODE_B_HOST:-}"
DATA_NODE_PUBLIC_HOST="${DATA_NODE_PUBLIC_HOST:-${EMQX_DATA_HOST:-}}"

[[ -n "${APP_NODE_A_HOST}" ]] || fail "APP_NODE_A_HOST is required (UFW allowlist)"

read_kv() {
	local file="$1"
	local key="$2"
	[[ -f "${file}" ]] || return 1
	local line
	line="$(grep -E "^${key}=" "${file}" | tail -n 1 || true)"
	[[ -n "${line}" ]] || return 1
	printf '%s' "${line#*=}"
}

set_kv() {
	local file="$1"
	local key="$2"
	local value="$3"
	[[ -f "${file}" ]] || return 1
	if grep -q "^${key}=" "${file}"; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${file}"
	else
		printf '%s=%s\n' "${key}" "${value}" >>"${file}"
	fi
}

is_loopback_bind() {
	case "${1:-}" in
	"" | "127.0.0.1" | "localhost" | "::1")
		return 0
		;;
	*)
		return 1
		;;
	esac
}

DATA_ENV="${NODE_ROOT}/.env.data-node"
PROD_ENV="${PROD_ROOT}/.env.production"
PRIVATE_BIND_IP="$(read_kv "${DATA_ENV}" PRIVATE_BIND_IP 2>/dev/null || read_kv "${PROD_ENV}" PRIVATE_BIND_IP 2>/dev/null || true)"
PRIVATE_BIND_IP="${PRIVATE_BIND_IP:-0.0.0.0}"
desired_bind="${PRIVATE_BIND_IP}"
if is_loopback_bind "${desired_bind}"; then
	desired_bind="0.0.0.0"
fi

CURRENT_BIND="$(read_kv "${DATA_ENV}" EMQX_DASHBOARD_BIND_IP 2>/dev/null || read_kv "${PROD_ENV}" EMQX_DASHBOARD_BIND_IP 2>/dev/null || true)"
CURRENT_BIND="${CURRENT_BIND:-127.0.0.1}"
note "EMQX dashboard bind current=${CURRENT_BIND} desired=${desired_bind}"

if [[ -f "${DATA_ENV}" ]]; then
	set_kv "${DATA_ENV}" "EMQX_DASHBOARD_BIND_IP" "${desired_bind}"
fi
if [[ -f "${PROD_ENV}" ]]; then
	set_kv "${PROD_ENV}" "EMQX_DASHBOARD_BIND_IP" "${desired_bind}"
fi

recreate_emqx() {
	local env_file="$1"
	local compose_file="$2"
	require_file "${env_file}"
	require_file "${compose_file}"
	local compose=(docker compose --env-file "${env_file}" -f "${compose_file}")
	if [[ "${CURRENT_BIND}" == "${desired_bind}" ]]; then
		note "emqx bind already ${desired_bind}; start/ensure without force-recreate (${compose_file})"
		"${compose[@]}" up -d --no-deps emqx
		return 0
	fi
	note "recreate emqx from ${compose_file} (port publish change; MQTT sessions on this broker will reconnect)"
	"${compose[@]}" up -d --no-deps --force-recreate emqx
}

recreated=0
if docker ps --format '{{.Names}}' | grep -qx 'avf-prod-emqx'; then
	legacy_env="${PROD_ROOT}/.env.production"
	legacy_compose="${PROD_ROOT}/docker-compose.prod.yml"
	if [[ -f "${legacy_env}" && -f "${legacy_compose}" ]]; then
		recreate_emqx "${legacy_env}" "${legacy_compose}"
		recreated=1
	fi
fi
if [[ -f "${DATA_ENV}" && -f "${NODE_ROOT}/docker-compose.data-node.yml" ]]; then
	recreate_emqx "${DATA_ENV}" "${NODE_ROOT}/docker-compose.data-node.yml"
	recreated=1
fi
[[ "${recreated}" -eq 1 ]] || fail "no EMQX compose stack found to recreate"

allow_ufw_from() {
	local peer="$1"
	[[ -n "${peer}" ]] || return 0
	if [[ -n "${DATA_NODE_PUBLIC_HOST}" && "${peer}" == "${DATA_NODE_PUBLIC_HOST}" ]]; then
		note "skip UFW allow for peer=${peer} (same host as data-node)"
		return 0
	fi
	if ! command -v ufw >/dev/null 2>&1; then
		note "ufw not installed; ensure host firewall allows ${peer} -> :18083/tcp"
		return 0
	fi
	if ufw status 2>/dev/null | grep -E "18083/tcp" | grep -Fq "${peer}"; then
		note "ufw already allows ${peer} to 18083/tcp"
		return 0
	fi
	note "ufw allow from ${peer} to 18083/tcp"
	ufw allow from "${peer}" to any port 18083 proto tcp comment 'avf-emqx-mgmt-app'
}

allow_ufw_from "${APP_NODE_A_HOST}"
if [[ -n "${APP_NODE_B_HOST}" ]]; then
	allow_ufw_from "${APP_NODE_B_HOST}"
fi

note "wait for local EMQX management API"
ok=0
for _ in $(seq 1 30); do
	if curl -sf --max-time 3 "http://127.0.0.1:18083/api/v5/status" | grep -Fq "emqx is running"; then
		ok=1
		break
	fi
	sleep 2
done
[[ "${ok}" -eq 1 ]] || fail "EMQX management API did not become ready on 127.0.0.1:18083"

if [[ "${desired_bind}" == "0.0.0.0" ]]; then
	[[ -n "${DATA_NODE_PUBLIC_HOST}" ]] || fail "DATA_NODE_PUBLIC_HOST is required when dashboard bind is 0.0.0.0"
	effective="http://${DATA_NODE_PUBLIC_HOST}:18083"
else
	effective="http://${desired_bind}:18083"
fi
echo "EMQX_MANAGEMENT_URL_EFFECTIVE=${effective}"
echo "EMQX_DASHBOARD_BIND_IP=${desired_bind}"
note "open_emqx_management_from_app_nodes: ok"
