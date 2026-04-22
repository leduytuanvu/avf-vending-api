#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${1:-${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../../app-node/.env.app-node}}"
REPORT_PATH="${CHECK_MANAGED_SERVICES_REPORT_PATH:-}"
STRICT_MODE="$(normalize_bool "${CHECK_MANAGED_SERVICES_STRICT:-1}")"

require_cmd bash
require_cmd curl
require_cmd python3
load_env_file "${ENV_FILE_PATH}"

failures=0
skips=0
REPORT_ROWS="$(mktemp)"

cleanup() {
	rm -f "${REPORT_ROWS}"
}
trap cleanup EXIT

pass() {
	echo "PASS: $1"
	printf '%s\t%s\t%s\n' "pass" "$1" "" >>"${REPORT_ROWS}"
}

fail_check() {
	echo "FAIL: $1" >&2
	failures=$((failures + 1))
	printf '%s\t%s\t%s\n' "fail" "$1" "" >>"${REPORT_ROWS}"
}

skip_check() {
	echo "SKIP: $1"
	skips=$((skips + 1))
	printf '%s\t%s\t%s\n' "skip" "$1" "" >>"${REPORT_ROWS}"
}

fail_or_skip() {
	local message="$1"
	if [[ "${STRICT_MODE}" == "1" ]]; then
		fail_check "${message}"
	else
		skip_check "${message}"
	fi
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
		fail_or_skip "DATABASE_URL is required for managed PostgreSQL checks"
		return 0
	}
	if is_placeholder_value "${DATABASE_URL}"; then
		fail_or_skip "DATABASE_URL is placeholder-only; managed PostgreSQL live validation not attempted"
		return 0
	fi
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
		if is_placeholder_value "${REDIS_URL}"; then
			fail_or_skip "REDIS_URL is placeholder-only; managed Redis live validation not attempted"
			return 0
		fi
		redis_target="$(python_value "url_host_port" "${REDIS_URL}")" || {
			fail_check "REDIS_URL is not parseable"
			return 0
		}
	elif [[ -n "${REDIS_ADDR:-}" ]]; then
		if is_placeholder_value "${REDIS_ADDR}"; then
			fail_or_skip "REDIS_ADDR is placeholder-only; managed Redis live validation not attempted"
			return 0
		fi
		redis_target="$(python_value "host_port" "${REDIS_ADDR}")" || {
			fail_check "REDIS_ADDR is not parseable"
			return 0
		}
	else
		skip_check "managed Redis not configured"
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
		skip_check "S3-compatible object storage checks disabled because API_ARTIFACTS_ENABLED is false"
		return 0
	fi

	endpoint="${OBJECT_STORAGE_ENDPOINT:-${S3_ENDPOINT:-}}"
	[[ -n "${endpoint}" ]] || {
		fail_or_skip "API_ARTIFACTS_ENABLED=true requires OBJECT_STORAGE_ENDPOINT or S3_ENDPOINT"
		return 0
	}
	[[ -n "${OBJECT_STORAGE_BUCKET:-${S3_BUCKET:-}}" ]] || {
		fail_or_skip "API_ARTIFACTS_ENABLED=true requires OBJECT_STORAGE_BUCKET or S3_BUCKET"
		return 0
	}
	if is_placeholder_value "${endpoint}"; then
		fail_or_skip "object storage endpoint is placeholder-only; live validation not attempted"
		return 0
	fi

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

if [[ -n "${REPORT_PATH}" ]]; then
	report_verdict="pass"
	if [[ "${failures}" -gt 0 ]]; then
		report_verdict="fail"
	elif [[ "${skips}" -gt 0 ]]; then
		report_verdict="not-configured"
	fi
	{
		printf '{\n'
		printf '  "control_scope": "managed-service-readiness-only",\n'
		printf '  "control_status": "readiness-only",\n'
		printf '  "restore_drill_executed": false,\n'
		printf '  "env_file_path": "%s",\n' "$(json_escape "${ENV_FILE_PATH}")"
		printf '  "strict_mode": %s,\n' "${STRICT_MODE}"
		printf '  "verdict": "%s",\n' "${report_verdict}"
		printf '  "failures": %s,\n' "${failures}"
		printf '  "skips": %s,\n' "${skips}"
		printf '  "summary": "%s",\n' "$(json_escape "$(
			if [[ "${report_verdict}" == "pass" ]]; then
				printf 'managed service readiness checks passed; this report does not prove restore execution'
			elif [[ "${report_verdict}" == "not-configured" ]]; then
				printf 'managed service readiness checks were only partially configured; this report does not prove restore execution'
			else
				printf 'managed service readiness checks failed; this report does not prove restore execution'
			fi
		)")"
		printf '  "checks": ['
		first="1"
		while IFS=$'\t' read -r status message detail; do
			[[ -n "${status}" ]] || continue
			if [[ "${first}" == "1" ]]; then
				first="0"
			else
				printf ','
			fi
			printf '\n    {"status":"%s","message":"%s","detail":"%s"}' \
				"$(json_escape "${status}")" \
				"$(json_escape "${message}")" \
				"$(json_escape "${detail}")"
		done <"${REPORT_ROWS}"
		if [[ "${first}" == "0" ]]; then
			printf '\n'
		fi
		printf '  ]\n'
		printf '}\n'
	} >"${REPORT_PATH}"
fi

if ((failures > 0)); then
	echo "check_managed_services: FAIL (${failures} checks failed)" >&2
	exit 1
fi

echo "check_managed_services: PASS"
