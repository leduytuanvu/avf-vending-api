#!/usr/bin/env bash
# Copy shared runtime env from app-node A .env.app-node to app-node B (keeps B-local MQTT client ids).
# Primary fix: B on Supabase session pooler (:5432) while A uses transaction pooler (:6543).
set -Eeuo pipefail

fail() { echo "sync_app_node_b_database_env: error: $*" >&2; exit 1; }
note() { echo "sync_app_node_b_database_env: $*"; }

PRODUCTION_DEPLOY_ROOT="${PRODUCTION_DEPLOY_ROOT:-/opt/avf-vending-api}"
SSH_USER="${SSH_USER:-root}"
APP_NODE_B_HOST="${APP_NODE_B_HOST:-}"
SSH_PORT="${SSH_PORT:-22}"

[[ -n "${APP_NODE_B_HOST}" ]] || fail "APP_NODE_B_HOST is not set"

PRIMARY_ENV="${PRODUCTION_DEPLOY_ROOT}/deployments/prod/app-node/.env.app-node"
REMOTE_ENV="${PRODUCTION_DEPLOY_ROOT}/deployments/prod/app-node/.env.app-node"
target="${SSH_USER}@${APP_NODE_B_HOST}"
read -r -a ssh_opts <<< "${SSH_OPTS:--o BatchMode=yes}"

[[ -f "${PRIMARY_ENV}" ]] || fail "missing primary env file ${PRIMARY_ENV}"

read_env_value() {
	local key="$1" file="$2"
	grep -E "^${key}=" "${file}" 2>/dev/null | tail -n1 | cut -d= -f2- || true
}

read_env_from_running_container() {
	local key="$1"
	local container name val
	while read -r name; do
		[[ -n "${name}" ]] || continue
		case "${name}" in
		*api* | *worker* | *reconciler* | *mqtt-ingest*)
			container="${name}"
			val="$(docker exec "${container}" printenv "${key}" 2>/dev/null | tr -d '\r' || true)"
			if [[ -n "${val}" ]]; then
				printf '%s' "${val}"
				return 0
			fi
			val="$(docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
				| grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r' || true)"
			if [[ -n "${val}" ]]; then
				printf '%s' "${val}"
				return 0
			fi
			;;
		esac
	done < <(docker ps --format '{{.Names}}' 2>/dev/null || true)
	return 0
}

strip_env_quotes() {
	local v="$1"
	v="${v%\"}"
	v="${v#\"}"
	v="${v%\'}"
	v="${v#\'}"
	printf '%s' "${v}"
}

primary_env_candidate_files() {
	printf '%s\n' "${PRIMARY_ENV}"
	local f
	for f in "${PRIMARY_ENV}".bak* "${PRODUCTION_DEPLOY_ROOT}/deployments/prod/.env.production" \
		"${PRODUCTION_DEPLOY_ROOT}/.env.production"; do
		[[ -f "${f}" ]] && printf '%s\n' "${f}"
	done
}

append_redis_from_primary_env_files() {
	local f line key val
	while IFS= read -r f; do
		[[ -f "${f}" ]] || continue
		while IFS= read -r line; do
			[[ "${line}" =~ ^REDIS_[A-Za-z0-9_]+= ]] || continue
			key="${line%%=*}"
			val="$(strip_env_quotes "${line#*=}")"
			append_line_to_updates "${key}" "${val}"
		done < <(grep -E '^REDIS_[A-Za-z0-9_]+=' "${f}" 2>/dev/null || true)
	done < <(primary_env_candidate_files)
}

read_primary_env_value() {
	local key="$1"
	local val
	val="$(read_env_value "${key}" "${PRIMARY_ENV}")"
	if [[ -n "${val}" ]]; then
		printf '%s' "${val}"
		return 0
	fi
	read_env_from_running_container "${key}"
}

# Running api on A reflects real managed-service URLs; .env on disk may still list compose placeholders.
read_primary_runtime_value() {
	local key="$1"
	local val
	val="$(read_env_from_running_container "${key}")"
	if [[ -n "${val}" ]]; then
		printf '%s' "${val}"
		return 0
	fi
	read_env_value "${key}" "${PRIMARY_ENV}"
}

is_placeholder_value() {
	local value="${1-}"
	[[ -z "${value}" ]] && return 0
	case "${value}" in
	*"CHANGE_ME"* | *"REPLACE_ME"* | *"example.com"* | *"example.invalid"* | *"placeholder"*)
		return 0
		;;
	esac
	return 1
}

# Values that belong to docker-compose on a data node, not app-node B production.
is_compose_local_service_ref() {
	local key="$1" value="$2"
	case "${value}" in
	redis:* | redis:6379) return 0 ;;
	nats://nats* | nats://nats:*) return 0 ;;
	mqtt://mosquitto* | mqtt://mosquitto:*) return 0 ;;
	postgres://postgres* | postgres://postgres:*) return 0 ;;
	esac
	case "${key}" in
	REDIS_ADDR)
		[[ "${value}" =~ ^redis:[0-9]+$ ]] && return 0
		;;
	esac
	return 1
}

should_sync_key_value() {
	local key="$1" value="$2"
	is_placeholder_value "${value}" && return 1
	is_compose_local_service_ref "${key}" "${value}" && return 1
	return 0
}

append_line_to_updates() {
	local key="$1" value="$2"
	should_sync_key_value "${key}" "${value}" || return 0
	printf '%s=%s\n' "${key}" "${value}" >>"${updates_file}"
}

# Redis is required in production; copy every REDIS_* var from the healthy api container on A.
append_primary_api_redis_env() {
	local name line key val
	while read -r name; do
		[[ -n "${name}" ]] || continue
		[[ "${name}" == *api* ]] || continue
		while IFS= read -r line; do
			[[ "${line}" =~ ^REDIS_[A-Za-z0-9_]+= ]] || continue
			key="${line%%=*}"
			val="${line#*=}"
			append_line_to_updates "${key}" "${val}"
		done < <(
			{
				docker exec "${name}" env 2>/dev/null || true
				docker inspect "${name}" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null || true
			} | grep -E '^REDIS_' | sort -u || true
		)
		return 0
	done < <(docker ps --format '{{.Names}}' 2>/dev/null || true)
	return 0
}

SHARED_SYNC_KEYS=(
	DATABASE_URL
	BACKUP_DATABASE_URL
	API_DATABASE_MAX_CONNS
	WORKER_DATABASE_MAX_CONNS
	MQTT_INGEST_DATABASE_MAX_CONNS
	RECONCILER_DATABASE_MAX_CONNS
	NATS_URL
	NATS_JETSTREAM_URL
	REDIS_URL
	REDIS_USERNAME
	REDIS_PASSWORD
	REDIS_TLS_ENABLED
	REDIS_TLS_INSECURE_SKIP_VERIFY
	REDIS_CACHE_ENABLED
	REDIS_RATE_LIMIT_ENABLED
	REDIS_SESSION_CACHE_ENABLED
	MQTT_BROKER_URL
	MQTT_USERNAME
	MQTT_PASSWORD
	APP_ENV
)

url_port() {
	python3 - "$1" <<'PY'
import sys
from urllib.parse import urlparse

u = urlparse(sys.argv[1])
print(u.port or 0)
PY
}

NODE_LOCAL_KEYS=(
	COMPOSE_PROJECT_NAME
	APP_NODE_NAME
	APP_INSTANCE_ID
	MQTT_CLIENT_ID_API
	MQTT_CLIENT_ID_INGEST
)

primary_url="$(read_primary_runtime_value DATABASE_URL)"
[[ -n "${primary_url}" ]] || fail "DATABASE_URL missing on app-node A (.env and running api container)"

remote_url="$(
	ssh "${ssh_opts[@]}" -p "${SSH_PORT}" "${target}" \
		"grep -E '^DATABASE_URL=' '${REMOTE_ENV}' 2>/dev/null | tail -n1 | cut -d= -f2-" || true
)"
[[ -n "${remote_url}" ]] || fail "DATABASE_URL missing on app-node B (${REMOTE_ENV})"

primary_port="$(url_port "${primary_url}")"
remote_port="$(url_port "${remote_url}")"
if [[ "${primary_url}" == "${remote_url}" ]]; then
	note "DATABASE_URL already matches A (port ${primary_port})"
else
	note "align B DATABASE_URL (remote port ${remote_port}) to A (port ${primary_port})"
fi

updates_file="$(mktemp)"
trap 'rm -f "${updates_file}"' EXIT
for key in "${SHARED_SYNC_KEYS[@]}"; do
	val="$(read_primary_runtime_value "${key}")"
	[[ -n "${val}" ]] || continue
	append_line_to_updates "${key}" "${val}"
done
append_primary_api_redis_env
append_redis_from_primary_env_files

if [[ -n "${SYNC_REDIS_URL:-}" ]] && ! is_placeholder_value "${SYNC_REDIS_URL}"; then
	append_line_to_updates REDIS_URL "${SYNC_REDIS_URL}"
	note "redis sync: applied SYNC_REDIS_URL from workflow"
fi

if [[ ! -s "${updates_file}" ]]; then
	fail "no env keys collected from app-node A"
fi

redis_url="$(grep -E '^REDIS_URL=' "${updates_file}" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
redis_addr="$(grep -E '^REDIS_ADDR=' "${updates_file}" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
note "redis sync: REDIS_URL=$([[ -n "${redis_url}" ]] && echo set || echo missing) REDIS_ADDR=$([[ -n "${redis_addr}" ]] && echo set || echo missing)"
if [[ -z "${redis_url}" && -z "${redis_addr}" ]]; then
	note "warning: no managed Redis endpoint collected for B; rollout will disable REDIS_* feature flags on B"
	echo "DISABLE_REDIS_FEATURES_ON_B=1" >>"${updates_file}"
else
	printf 'PRODUCTION_ALLOW_MISSING_REDIS=false\n' >>"${updates_file}"
fi

remote_patch="$(mktemp)"
trap 'rm -f "${updates_file}" "${remote_patch}"' EXIT
scp "${ssh_opts[@]}" -P "${SSH_PORT}" "${updates_file}" "${target}:${remote_patch}" >/dev/null

ssh "${ssh_opts[@]}" -p "${SSH_PORT}" "${target}" bash -s -- "${REMOTE_ENV}" "${remote_patch}" <<'REMOTE'
set -Eeuo pipefail
env_path="$1"
patch_path="$2"
backup="${env_path}.bak.sync-db-$(date -u +%Y%m%dT%H%M%SZ)"
cp "${env_path}" "${backup}"
python3 - "${env_path}" "${patch_path}" <<'PY'
import pathlib
import re
import sys

path = pathlib.Path(sys.argv[1])
patch_path = pathlib.Path(sys.argv[2])
updates: dict[str, str] = {}
disable_redis = False
for raw in patch_path.read_text(encoding="utf-8", errors="replace").splitlines():
    raw = raw.rstrip("\n")
    if not raw or "=" not in raw:
        continue
    key, value = raw.split("=", 1)
    if key == "DISABLE_REDIS_FEATURES_ON_B":
        disable_redis = value == "1"
        continue
    updates[key] = value

lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
seen = set()
out: list[str] = []
for line in lines:
    if not line or line.lstrip().startswith("#") or "=" not in line:
        out.append(line)
        continue
    key = line.split("=", 1)[0]
    if key in updates:
        out.append(f"{key}={updates[key]}")
        seen.add(key)
    else:
        out.append(line)
for key, value in updates.items():
    if key not in seen:
        out.append(f"{key}={value}")
path.write_text("\n".join(out) + "\n", encoding="utf-8")

# REDIS_ADDR wins over REDIS_URL in app config; drop compose service names (api panic on B).
text = path.read_text(encoding="utf-8", errors="replace").splitlines()
filtered: list[str] = []
for line in text:
    if line.startswith("REDIS_ADDR="):
        _, addr = line.split("=", 1)
        if re.fullmatch(r"redis:\d+", addr):
            continue
    filtered.append(line)
path.write_text("\n".join(filtered) + "\n", encoding="utf-8")

if disable_redis:
    text = path.read_text(encoding="utf-8", errors="replace").splitlines()
    off = {
        "REDIS_ENABLED": "false",
        "REDIS_CACHE_ENABLED": "false",
        "REDIS_RATE_LIMIT_ENABLED": "false",
        "REDIS_SESSION_CACHE_ENABLED": "false",
        "REDIS_LOCKS_ENABLED": "false",
        "CACHE_ENABLED": "false",
        "PRODUCTION_ALLOW_MISSING_REDIS": "true",
    }
    drop_keys = {
        "REDIS_URL",
        "REDIS_ADDR",
        "REDIS_USERNAME",
        "REDIS_PASSWORD",
        "REDIS_DB",
        "REDIS_TLS_ENABLED",
        "REDIS_TLS_INSECURE_SKIP_VERIFY",
    }
    seen = set()
    out2: list[str] = []
    for line in text:
        if not line or line.lstrip().startswith("#") or "=" not in line:
            out2.append(line)
            continue
        key = line.split("=", 1)[0]
        if key in drop_keys:
            continue
        if key in off:
            out2.append(f"{key}={off[key]}")
            seen.add(key)
        else:
            out2.append(line)
    for key, value in off.items():
        if key not in seen:
            out2.append(f"{key}={value}")
    path.write_text("\n".join(out2) + "\n", encoding="utf-8")
PY
rm -f "${patch_path}"
REMOTE

node_name="$(ssh "${ssh_opts[@]}" -p "${SSH_PORT}" "${target}" \
	"grep -E '^APP_NODE_NAME=' '${REMOTE_ENV}' 2>/dev/null | tail -n1 | cut -d= -f2-" || true)"
if [[ -n "${node_name}" ]]; then
	ssh "${ssh_opts[@]}" -p "${SSH_PORT}" "${target}" bash -s -- "${REMOTE_ENV}" "${node_name}" <<'FIX'
set -Eeuo pipefail
env_path="$1"
node_name="$2"
instance_id="${node_name}-api"
if grep -q '^APP_INSTANCE_ID=' "${env_path}"; then
	sed -i "s/^APP_INSTANCE_ID=.*/APP_INSTANCE_ID=${instance_id}/" "${env_path}"
else
	printf 'APP_INSTANCE_ID=%s\n' "${instance_id}" >>"${env_path}"
fi
FIX
	note "set APP_INSTANCE_ID=${node_name}-api on B"
fi

note "aligned shared env on B with A (remote .env.app-node.bak.sync-db-* next to env file)"
