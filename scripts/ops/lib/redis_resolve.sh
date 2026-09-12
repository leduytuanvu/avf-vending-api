#!/usr/bin/env bash
# Resolve Redis connection settings mirroring internal/config loadRedisConfig().
# Source from wipe/audit scripts. Never logs passwords.
set -Eeuo pipefail

REDIS_RESOLVE_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

_redis_resolve_py() {
	if command -v python3 >/dev/null 2>&1; then
		python3 "$@"
	elif command -v python >/dev/null 2>&1; then
		python "$@"
	elif command -v py >/dev/null 2>&1; then
		py -3 "$@"
	else
		echo "redis_resolve: python required" >&2
		return 1
	fi
}

# Populate REDIS_RESOLVED_* from current environment (call after sourcing env file).
redis_resolve_config() {
	local json
	json="$(_redis_resolve_py - <<'PY'
import json, os, urllib.parse

def truthy(name, default=False):
    v = os.environ.get(name, "")
    if v == "":
        return default
    return v.strip().lower() in ("1", "true", "yes", "on")

addr = (os.environ.get("REDIS_ADDR") or "").strip()
redis_url = (os.environ.get("REDIS_URL") or "").strip()
username = (os.environ.get("REDIS_USERNAME") or "").strip()
password = os.environ.get("REDIS_PASSWORD") or ""
db = int(os.environ.get("REDIS_DB") or "0")
tls = truthy("REDIS_TLS_ENABLED", False)
tls_insecure = truthy("REDIS_TLS_INSECURE_SKIP_VERIFY", False)
key_prefix = (os.environ.get("REDIS_KEY_PREFIX") or "avf").strip() or "avf"
enabled = truthy("REDIS_ENABLED", bool(addr or redis_url))

if not addr and redis_url:
    u = urllib.parse.urlparse(redis_url)
    if not u.hostname:
        raise SystemExit("invalid REDIS_URL: missing host")
    host = u.hostname
    port = u.port or (6380 if u.scheme == "rediss" else 6379)
    addr = f"{host}:{port}"
    if not username and u.username:
        username = urllib.parse.unquote(u.username)
    if not password and u.password:
        password = urllib.parse.unquote(u.password)
    path = (u.path or "").lstrip("/")
    if path and db == 0:
        db = int(path)
    if not tls:
        tls = u.scheme == "rediss"

configured = enabled and bool(addr)
print(json.dumps({
    "configured": configured,
    "addr": addr,
    "username": username,
    "password": password,
    "db": db,
    "tls": tls,
    "tls_insecure": tls_insecure,
    "key_prefix": key_prefix,
}))
PY
)" || return 1

	REDIS_RESOLVED_CONFIGURED="$(_redis_resolve_py -c "import json,sys; print('true' if json.load(sys.stdin)['configured'] else 'false')" <<<"${json}")"
	REDIS_RESOLVED_ADDR="$(_redis_resolve_py -c "import json,sys; print(json.load(sys.stdin)['addr'])" <<<"${json}")"
	REDIS_RESOLVED_USERNAME="$(_redis_resolve_py -c "import json,sys; print(json.load(sys.stdin)['username'])" <<<"${json}")"
	REDIS_RESOLVED_PASSWORD="$(_redis_resolve_py -c "import json,sys; print(json.load(sys.stdin)['password'])" <<<"${json}")"
	REDIS_RESOLVED_DB="$(_redis_resolve_py -c "import json,sys; print(json.load(sys.stdin)['db'])" <<<"${json}")"
	REDIS_RESOLVED_TLS="$(_redis_resolve_py -c "import json,sys; print('true' if json.load(sys.stdin)['tls'] else 'false')" <<<"${json}")"
	REDIS_RESOLVED_TLS_INSECURE="$(_redis_resolve_py -c "import json,sys; print('true' if json.load(sys.stdin)['tls_insecure'] else 'false')" <<<"${json}")"
	REDIS_RESOLVED_KEY_PREFIX="$(_redis_resolve_py -c "import json,sys; print(json.load(sys.stdin)['key_prefix'])" <<<"${json}")"
	export REDIS_RESOLVED_CONFIGURED REDIS_RESOLVED_ADDR REDIS_RESOLVED_USERNAME REDIS_RESOLVED_PASSWORD
	export REDIS_RESOLVED_DB REDIS_RESOLVED_TLS REDIS_RESOLVED_TLS_INSECURE REDIS_RESOLVED_KEY_PREFIX
}

redis_configured() {
	redis_resolve_config
	[[ "${REDIS_RESOLVED_CONFIGURED}" == "true" ]]
}

redis_find_container() {
	local name="${1:-}"
	if [[ -n "${name}" ]] && docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "${name}"; then
		echo "${name}"
		return 0
	fi
	docker ps --format '{{.Names}}' 2>/dev/null | grep -i redis | head -1 || true
}

# Build redis-cli URL (password not echoed to stdout by callers).
redis_cli_url() {
	redis_resolve_config
	local scheme="redis"
	[[ "${REDIS_RESOLVED_TLS}" == "true" ]] && scheme="rediss"
	local auth=""
	if [[ -n "${REDIS_RESOLVED_USERNAME}" && -n "${REDIS_RESOLVED_PASSWORD}" ]]; then
		auth="${REDIS_RESOLVED_USERNAME}:${REDIS_RESOLVED_PASSWORD}@"
	elif [[ -n "${REDIS_RESOLVED_PASSWORD}" ]]; then
		auth=":${REDIS_RESOLVED_PASSWORD}@"
	elif [[ -n "${REDIS_RESOLVED_USERNAME}" ]]; then
		auth="${REDIS_RESOLVED_USERNAME}@"
	fi
	echo "${scheme}://${auth}${REDIS_RESOLVED_ADDR}/${REDIS_RESOLVED_DB}"
}

# Run redis-cli with resolved config. Extra args passed through.
redis_cli_exec() {
	redis_resolve_config
	local container="${REDIS_CONTAINER:-}"
	local url
	url="$(redis_cli_url)"
	local -a tls_args=()
	if [[ "${REDIS_RESOLVED_TLS}" == "true" ]]; then
		tls_args+=(--tls)
		[[ "${REDIS_RESOLVED_TLS_INSECURE}" == "true" ]] && tls_args+=(--insecure)
	fi

	if [[ -n "${container}" ]]; then
		local -a exec_args=(-n "${REDIS_RESOLVED_DB}")
		if [[ -n "${REDIS_RESOLVED_USERNAME}" && -n "${REDIS_RESOLVED_PASSWORD}" ]]; then
			exec_args+=(--user "${REDIS_RESOLVED_USERNAME}" -a "${REDIS_RESOLVED_PASSWORD}")
		elif [[ -n "${REDIS_RESOLVED_PASSWORD}" ]]; then
			exec_args+=(-a "${REDIS_RESOLVED_PASSWORD}")
		fi
		docker exec "${container}" redis-cli "${exec_args[@]}" "$@"
		return $?
	fi

	if [[ "${REDIS_RESOLVED_ADDR}" == *:* ]]; then
		local host="${REDIS_RESOLVED_ADDR%%:*}"
		local port="${REDIS_RESOLVED_ADDR##*:}"
		if docker ps --format '{{.Names}}' 2>/dev/null | grep -qi redis; then
			container="$(redis_find_container "")"
			if [[ -n "${container}" ]]; then
				docker run --rm --network "container:${container}" redis:7-alpine \
					redis-cli -u "${url}" "${tls_args[@]}" "$@"
				return $?
			fi
		fi
	fi

	if command -v redis-cli >/dev/null 2>&1; then
		redis-cli -u "${url}" "${tls_args[@]}" "$@"
	else
		docker run --rm redis:7-alpine redis-cli -u "${url}" "${tls_args[@]}" "$@"
	fi
}

redis_dbsize() {
	redis_cli_exec DBSIZE 2>/dev/null | tail -1
}

redis_avf_key_count() {
	redis_resolve_config
	local prefix="${REDIS_RESOLVED_KEY_PREFIX}"
	redis_cli_exec --scan --pattern "${prefix}*" 2>/dev/null | wc -l | tr -d ' '
}

redis_audit_log() {
	redis_resolve_config
	echo "redis_audit: configured=${REDIS_RESOLVED_CONFIGURED} addr=${REDIS_RESOLVED_ADDR} db=${REDIS_RESOLVED_DB} key_prefix=${REDIS_RESOLVED_KEY_PREFIX}"
	if [[ "${REDIS_RESOLVED_CONFIGURED}" != "true" ]]; then
		echo "redis_audit: not configured"
		return 0
	fi
	local dbsize avf_keys
	dbsize="$(redis_dbsize || echo "ERROR")"
	avf_keys="$(redis_avf_key_count || echo "ERROR")"
	echo "redis_audit: DBSIZE=${dbsize} avf_prefixed_keys=${avf_keys}"
}
