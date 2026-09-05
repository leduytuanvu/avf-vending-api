#!/usr/bin/env bash
# Read-only: verify platform_auth_accounts exists on the API container DATABASE_URL.
set -Eeuo pipefail

POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

fail() { echo "verify-production-auth-schema: error: $*" >&2; exit 1; }
note() { echo "verify-production-auth-schema: $*"; }

find_api_container() {
	docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

container_env() {
	local container="$1"
	local key="$2"
	docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
		| grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r'
}

sanitize_psql_url() {
	python3 - "$1" <<'PY'
import sys
from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse

u = urlparse(sys.argv[1])
drop = {"default_query_exec_mode", "pgbouncer"}
q = [(k, v) for k, v in parse_qsl(u.query, keep_blank_values=True) if k not in drop]
print(urlunparse((u.scheme, u.netloc, u.path, u.params, urlencode(q), u.fragment)))
PY
}

mask_url() {
	python3 - "$1" <<'PY'
import sys
from urllib.parse import urlsplit, urlunsplit, unquote

raw = sys.argv[1].strip()
u = urlsplit(raw)
user = unquote(u.username or "")
host = u.hostname or ""
port = f":{u.port}" if u.port else ""
path = u.path or ""
netloc = host + port
if user:
    netloc = f"{user}:***@{netloc}"
print(urlunsplit((u.scheme, netloc, path, u.query, u.fragment)))
PY
}

table_exists() {
	local url="$1"
	local psql_url
	psql_url="$(sanitize_psql_url "${url}")"
	docker run --rm \
		-e "DATABASE_URL=${psql_url}" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${psql_url}" -v ON_ERROR_STOP=1 -Atqc \
		"SELECT to_regclass('public.platform_auth_accounts') IS NOT NULL;"
}

account_count() {
	local url="$1"
	local psql_url
	psql_url="$(sanitize_psql_url "${url}")"
	docker run --rm \
		-e "DATABASE_URL=${psql_url}" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${psql_url}" -v ON_ERROR_STOP=1 -Atqc \
		"SELECT count(*)::bigint FROM platform_auth_accounts;"
}

API_CONTAINER="$(find_api_container)"
[[ -n "${API_CONTAINER}" ]] || fail "api container not found"
DATABASE_URL="$(container_env "${API_CONTAINER}" DATABASE_URL)"
[[ -n "${DATABASE_URL}" ]] || fail "DATABASE_URL missing in ${API_CONTAINER}"

note "api container=${API_CONTAINER}"
note "database=$(mask_url "${DATABASE_URL}")"

exists="$(table_exists "${DATABASE_URL}")"
[[ "${exists}" == "t" ]] || fail "platform_auth_accounts missing on API DATABASE_URL"

count="$(account_count "${DATABASE_URL}")"
note "platform_auth_accounts count=${count}"
[[ "${count}" -gt 0 ]] || fail "platform_auth_accounts is empty; seed admin before technician login"

note "PASS auth schema present with ${count} account(s)"
