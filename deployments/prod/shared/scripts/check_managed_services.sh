#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${1:-${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../../app-node/.env.app-node}}"

require_cmd bash
require_cmd curl
require_cmd python3
load_env_file "${ENV_FILE_PATH}"

failures=0

pass() {
	echo "PASS: $1"
}

fail_check() {
	echo "FAIL: $1" >&2
	failures=$((failures + 1))
}

python_value() {
	local mode="$1"
	local value="$2"
	python3 - "$mode" "$value" <<'PY'
import sys
from urllib.parse import urlsplit

mode = sys.argv[1]
value = sys.argv[2]

if mode == "url_host_port":
    parsed = urlsplit(value)
    host = parsed.hostname or ""
    if not host:
        raise SystemExit(1)
    port = parsed.port
    if port is None:
        if parsed.scheme in ("postgres", "postgresql"):
            port = 5432
        elif parsed.scheme in ("redis", "rediss"):
            port = 6379
        elif parsed.scheme == "https":
            port = 443
        else:
            port = 80
    print(f"{host}:{port}")
elif mode == "host_port":
    if ":" in value:
        host, port = value.rsplit(":", 1)
    else:
        host, port = value, "6379"
    if not host or not port:
        raise SystemExit(1)
    print(f"{host}:{port}")
else:
    raise SystemExit(2)
PY
}

tcp_check() {
	local host="$1"
	local port="$2"
	python3 - "$host" "$port" <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
with socket.create_connection((host, port), timeout=5):
    pass
PY
}

check_managed_postgres() {
	local host_port host port
	[[ -n "${DATABASE_URL:-}" ]] || {
		fail_check "DATABASE_URL is required for production app-node health checks"
		return 0
	}
	host_port="$(python_value "url_host_port" "${DATABASE_URL}")" || {
		fail_check "DATABASE_URL is not parseable"
		return 0
	}
	host="${host_port%:*}"
	port="${host_port##*:}"

	if command -v pg_isready >/dev/null 2>&1; then
		if pg_isready -d "${DATABASE_URL}" -t 5 >/dev/null 2>&1; then
			pass "managed PostgreSQL accepts pg_isready"
		else
			fail_check "managed PostgreSQL did not pass pg_isready"
		fi
	elif tcp_check "${host}" "${port}" >/dev/null 2>&1; then
		pass "managed PostgreSQL TCP reachability ${host}:${port}"
	else
		fail_check "managed PostgreSQL TCP reachability failed for ${host}:${port}"
	fi
}

check_managed_redis() {
	local host_port host port redis_target
	if [[ -n "${REDIS_URL:-}" ]]; then
		redis_target="$(python_value "url_host_port" "${REDIS_URL}")" || {
			fail_check "REDIS_URL is not parseable"
			return 0
		}
	elif [[ -n "${REDIS_ADDR:-}" ]]; then
		redis_target="$(python_value "host_port" "${REDIS_ADDR}")" || {
			fail_check "REDIS_ADDR is not parseable"
			return 0
		}
	else
		echo "SKIP: managed Redis not configured"
		return 0
	fi

	host="${redis_target%:*}"
	port="${redis_target##*:}"

	if command -v redis-cli >/dev/null 2>&1 && [[ -n "${REDIS_URL:-}" ]]; then
		if redis-cli --no-auth-warning -u "${REDIS_URL}" ping >/dev/null 2>&1; then
			pass "managed Redis accepts PING"
		else
			fail_check "managed Redis did not pass redis-cli PING"
		fi
	elif tcp_check "${host}" "${port}" >/dev/null 2>&1; then
		pass "managed Redis TCP reachability ${host}:${port}"
	else
		fail_check "managed Redis TCP reachability failed for ${host}:${port}"
	fi
}

check_object_storage() {
	local endpoint host_port host port
	if [[ "$(normalize_bool "${API_ARTIFACTS_ENABLED:-0}")" != "1" ]]; then
		echo "SKIP: S3-compatible object storage checks disabled because API_ARTIFACTS_ENABLED is false"
		return 0
	fi

	endpoint="${OBJECT_STORAGE_ENDPOINT:-${S3_ENDPOINT:-}}"
	[[ -n "${endpoint}" ]] || {
		fail_check "API_ARTIFACTS_ENABLED=true requires OBJECT_STORAGE_ENDPOINT or S3_ENDPOINT"
		return 0
	}
	[[ -n "${OBJECT_STORAGE_BUCKET:-${S3_BUCKET:-}}" ]] || {
		fail_check "API_ARTIFACTS_ENABLED=true requires OBJECT_STORAGE_BUCKET or S3_BUCKET"
		return 0
	}

	host_port="$(python_value "url_host_port" "${endpoint}")" || {
		fail_check "object storage endpoint is not parseable"
		return 0
	}
	host="${host_port%:*}"
	port="${host_port##*:}"

	if curl --connect-timeout 5 --max-time 10 -k -sS -o /dev/null "${endpoint%/}/"; then
		pass "S3-compatible object storage HTTP(S) reachability"
	elif tcp_check "${host}" "${port}" >/dev/null 2>&1; then
		pass "S3-compatible object storage TCP reachability ${host}:${port}"
	else
		fail_check "S3-compatible object storage reachability failed for ${host}:${port}"
	fi
}

note "managed production dependency checks"
check_managed_postgres
check_managed_redis
check_object_storage

if (( failures > 0 )); then
	echo "check_managed_services: FAIL (${failures} checks failed)" >&2
	exit 1
fi

echo "check_managed_services: PASS"
