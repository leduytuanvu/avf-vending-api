#!/usr/bin/env bash
# Audit and purge EMQX machine identities, retained messages, and stale sessions.
# Usage:
#   bash scripts/ops/emqx-audit-and-purge.sh --audit-only
#   bash scripts/ops/emqx-audit-and-purge.sh --purge
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY_ROOT="${AVF_DEPLOY_ROOT:-${ROOT}}"
MODE="audit-only"
DRY_RUN=0

fail() {
	echo "emqx-audit-and-purge: error: $*" >&2
	exit 1
}

note() {
	echo "emqx-audit-and-purge: $*"
}

usage() {
	cat <<EOF
usage: bash scripts/ops/emqx-audit-and-purge.sh [--audit-only|--purge] [--dry-run]

Reads EMQX_* and MQTT_USERNAME from deployments/prod/.env.production or data-node env.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--audit-only) MODE="audit-only"; shift ;;
	--purge) MODE="purge"; shift ;;
	--dry-run) DRY_RUN=1; shift ;;
	-h | --help) usage; exit 0 ;;
	*) fail "unknown argument: $1" ;;
	esac
done

ENVF="${DEPLOY_ROOT}/deployments/prod/.env.production"
if [[ ! -f "${ENVF}" && -f "${DEPLOY_ROOT}/deployments/prod/data-node/.env.data-node" ]]; then
	ln -sf data-node/.env.data-node "${DEPLOY_ROOT}/deployments/prod/.env.production" 2>/dev/null || true
	ENVF="${DEPLOY_ROOT}/deployments/prod/data-node/.env.data-node"
fi
[[ -f "${ENVF}" ]] || fail "missing EMQX env file"

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" "${ENVF}" 2>/dev/null | tail -n1 || true)"
	[[ -n "${line}" ]] || fail "${key} not set in ${ENVF}"
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

MQTT_USERNAME="$(read_env MQTT_USERNAME)"
EMQX_API_KEY="${EMQX_API_KEY:-$(read_env EMQX_API_KEY)}"
EMQX_API_SECRET="${EMQX_API_SECRET:-$(read_env EMQX_API_SECRET)}"
EMQX_BASE="${EMQX_MANAGEMENT_URL:-http://127.0.0.1:18083}"
EMQX_BASE="${EMQX_BASE%/}"
TOPIC_PREFIX="${MQTT_TOPIC_PREFIX:-avf/devices}"

command -v jq >/dev/null 2>&1 || fail "jq is required"
command -v curl >/dev/null 2>&1 || fail "curl is required"

_emqx_urlencode() {
	_emqx_resolve_py -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=''))" "$1"
}

_emqx_resolve_py() {
	if command -v python3 >/dev/null 2>&1; then python3 "$@"; else python "$@"; fi
}

AUTH_PATH="authentication/password_based%3Abuilt_in_database/users"
API="${EMQX_BASE}/api/v5"

emqx_curl() {
	curl -fsS -u "${EMQX_API_KEY}:${EMQX_API_SECRET}" "$@"
}

note "mode=${MODE} management=${EMQX_BASE} service_user=${MQTT_USERNAME} topic_prefix=${TOPIC_PREFIX}"

if ! emqx_curl --max-time 5 "${API}/status" >/dev/null 2>&1; then
	fail "EMQX management API not reachable at ${EMQX_BASE}"
fi

# --- Users ---
users_json="$(emqx_curl "${API}/${AUTH_PATH}?limit=1000")"
machine_users=0
service_users=0
while IFS= read -r user_id; do
	[[ -n "${user_id}" ]] || continue
	if [[ "${user_id}" == "${MQTT_USERNAME}" || "${user_id}" == "avf_app" || "${user_id}" == "avf-mqtt-api" || "${user_id}" == "avf-mqtt-ingest" ]]; then
		service_users=$((service_users + 1))
		note "user keep service=${user_id}"
	else
		machine_users=$((machine_users + 1))
		note "user machine=${user_id}"
		if [[ "${MODE}" == "purge" && "${DRY_RUN}" -eq 0 ]]; then
			code="$(curl -sS -o /dev/null -w "%{http_code}" \
				-u "${EMQX_API_KEY}:${EMQX_API_SECRET}" \
				-X DELETE "${API}/${AUTH_PATH}/${user_id}" || true)"
			note "deleted user=${user_id} http=${code}"
		fi
	fi
done < <(jq -r '.data[].user_id // empty' <<<"${users_json}")

# --- Retained messages ---
	retained_count=0
	if retained_json="$(emqx_curl "${API}/mqtt/retainer/messages?limit=1000" 2>/dev/null)"; then
		retained_count="$(jq -r '[.data[] | select(.topic | startswith("$SYS/") | not)] | length' <<<"${retained_json}" 2>/dev/null || echo 0)"
	note "retained_messages=${retained_count}"
	if [[ "${MODE}" == "purge" && "${retained_count}" -gt 0 ]]; then
		while IFS= read -r topic; do
			[[ -n "${topic}" ]] || continue
			[[ "${topic}" == \$SYS/* ]] && continue
			if [[ "${DRY_RUN}" -eq 1 ]]; then
				note "[dry-run] would delete retained topic=${topic}"
			else
				encoded="$(_emqx_urlencode "${topic}")"
				curl -fsS -u "${EMQX_API_KEY}:${EMQX_API_SECRET}" \
					-X DELETE "${API}/mqtt/retainer/message/${encoded}" >/dev/null 2>&1 \
					|| note "warn: failed to delete retained ${topic}"
			fi
		done < <(jq -r '.data[].topic // empty' <<<"${retained_json}")
		if [[ "${MODE}" == "purge" && "${DRY_RUN}" -eq 0 ]]; then
			curl -fsS -u "${EMQX_API_KEY}:${EMQX_API_SECRET}" \
				-X DELETE "${API}/mqtt/retainer/messages" >/dev/null 2>&1 \
				|| note "warn: bulk retainer delete failed; trying emqx ctl"
			EMQX_CTN="$(docker ps --format '{{.Names}}' 2>/dev/null | grep -i emqx | head -1 || true)"
			if [[ -n "${EMQX_CTN}" ]]; then
				docker exec "${EMQX_CTN}" emqx ctl retainer clean 2>/dev/null \
					|| docker exec "${EMQX_CTN}" emqx_ctl retainer clean 2>/dev/null \
					|| true
			fi
		fi
	fi
else
	note "retained_messages=UNKNOWN (API endpoint unavailable)"
fi

# --- Connected clients (machine sessions) ---
client_count=0
if clients_json="$(emqx_curl "${API}/clients?limit=1000" 2>/dev/null)"; then
	client_count="$(jq -r '.data | length' <<<"${clients_json}" 2>/dev/null || echo 0)"
	note "connected_clients=${client_count}"
	if [[ "${MODE}" == "purge" && "${client_count}" -gt 0 ]]; then
		while IFS= read -r client_id; do
			[[ -n "${client_id}" ]] || continue
			if [[ "${client_id}" == *"${MQTT_USERNAME}"* ]]; then
				note "client keep service=${client_id}"
				continue
			fi
			if [[ "${DRY_RUN}" -eq 1 ]]; then
				note "[dry-run] would kick client=${client_id}"
			else
				curl -fsS -u "${EMQX_API_KEY}:${EMQX_API_SECRET}" \
					-X DELETE "${API}/clients/${client_id}" >/dev/null 2>&1 \
					|| note "warn: failed to kick ${client_id}"
			fi
		done < <(jq -r '.data[].clientid // empty' <<<"${clients_json}")
	fi
else
	note "connected_clients=UNKNOWN"
fi

note "summary machine_users=${machine_users} service_users=${service_users} retained=${retained_count} clients=${client_count}"

audit_fail=0
[[ "${machine_users}" -eq 0 ]] || audit_fail=1
# retained_count already excludes $SYS/* infrastructure topics
[[ "${retained_count}" -eq 0 ]] || audit_fail=1

if [[ "${MODE}" == "audit-only" && "${audit_fail}" -ne 0 ]]; then
	note "audit: FAIL"
	exit 1
fi

if [[ "${MODE}" == "purge" && "${DRY_RUN}" -eq 0 ]]; then
	users_json="$(emqx_curl "${API}/${AUTH_PATH}?limit=1000")"
	machine_users="$(jq -r --arg svc "${MQTT_USERNAME}" '[.data[].user_id | select(. != $svc and . != "avf_app" and . != "avf-mqtt-api" and . != "avf-mqtt-ingest")] | length' <<<"${users_json}")"
	[[ "${machine_users}" -eq 0 ]] || fail "post-purge machine users remain: ${machine_users}"
	retained_after=0
	if retained_json="$(emqx_curl "${API}/mqtt/retainer/messages?limit=1000" 2>/dev/null)"; then
		retained_after="$(jq -r '[.data[] | select(.topic | startswith("$SYS/") | not)] | length' <<<"${retained_json}" 2>/dev/null || echo 0)"
	fi
	[[ "${retained_after}" -eq 0 ]] || fail "post-purge AVF retained messages remain: ${retained_after}"
	note "purge: PASS"
fi
