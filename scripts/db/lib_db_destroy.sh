#!/usr/bin/env bash
# Shared helpers for AVF database discovery/destruction tooling.
# Never prints full DATABASE_URL or passwords.
set -Eeuo pipefail

db_destroy_py() {
	if command -v python3 >/dev/null 2>&1; then
		python3 "$@"
	elif command -v python >/dev/null 2>&1; then
		python "$@"
	elif command -v py >/dev/null 2>&1; then
		py -3 "$@"
	else
		db_destroy_fail "python3/python not found"
	fi
}

DB_DESTROY_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DB_DESTROY_REPO_ROOT="$(cd "${DB_DESTROY_LIB_DIR}/../.." && pwd)"

if ! command -v docker >/dev/null 2>&1; then
	for _docker_dir in \
		"/c/Program Files/Docker/Docker/resources/bin" \
		"/mnt/c/Program Files/Docker/Docker/resources/bin"; do
		if [[ -d "${_docker_dir}" ]]; then
			export PATH="${_docker_dir}:${PATH}"
			break
		fi
	done
fi
DB_DESTROY_REGISTRY="${DB_DESTROY_REGISTRY:-${DB_DESTROY_LIB_DIR}/target_registry.json}"
DB_DESTROY_EVIDENCE_DIR="${DB_DESTROY_EVIDENCE_DIR:-${DB_DESTROY_REPO_ROOT}/.db-destroy-evidence}"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

AVF_MARKER_TABLES=("platform_auth_accounts" "machines" "goose_db_version")

db_destroy_fail() {
	echo "db_destroy: error: $*" >&2
	exit 2
}

db_destroy_note() {
	echo "db_destroy: $*"
}

db_destroy_warn() {
	echo "db_destroy: warn: $*" >&2
}

db_destroy_utc() {
	date -u +"%Y-%m-%dT%H:%M:%SZ"
}

db_destroy_require_cmd() {
	local cmd="$1"
	command -v "${cmd}" >/dev/null 2>&1 || db_destroy_fail "required command not found: ${cmd}"
}

db_destroy_mask_url() {
	db_destroy_py - "$1" <<'PY'
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

db_destroy_sanitize_psql_url() {
	db_destroy_py - "$1" <<'PY'
import sys
from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse

u = urlparse(sys.argv[1])
drop = {"default_query_exec_mode", "pgbouncer"}
q = [(k, v) for k, v in parse_qsl(u.query, keep_blank_values=True) if k not in drop]
print(urlunparse((u.scheme, u.netloc, u.path, u.params, urlencode(q), u.fragment)))
PY
}

db_destroy_parse_url() {
	db_destroy_py - "$1" <<'PY'
import json, sys
from urllib.parse import urlsplit, unquote

raw = sys.argv[1].strip()
u = urlsplit(raw)
dbn = (u.path or "").lstrip("/").split("?", 1)[0]
sslmode = "default"
if u.query:
    for part in u.query.split("&"):
        if part.startswith("sslmode="):
            sslmode = part.split("=", 1)[1]
            break
out = {
    "scheme": u.scheme,
    "host": (u.hostname or "").lower(),
    "port": u.port or (5432 if u.scheme.startswith("postgres") else None),
    "database": dbn,
    "username": unquote(u.username or ""),
    "sslmode": sslmode,
}
print(json.dumps(out))
PY
}

db_destroy_fingerprint() {
	db_destroy_py - "$1" "$2" "$3" <<'PY'
import hashlib, sys

host, port, database = sys.argv[1:4]
key = f"{host.lower()}:{port}:{database.lower()}"
print(hashlib.sha256(key.encode()).hexdigest()[:16])
PY
}

db_destroy_maintenance_url() {
	db_destroy_py - "$1" <<'PY'
import sys
from urllib.parse import urlsplit, urlunsplit

u = urlsplit(sys.argv[1])
path = "/postgres"
print(urlunsplit((u.scheme, u.netloc, path, u.query, u.fragment)))
PY
}

db_destroy_replace_database() {
	db_destroy_py - "$1" "$2" <<'PY'
import sys
from urllib.parse import urlsplit, urlunsplit

raw, new_db = sys.argv[1], sys.argv[2]
u = urlsplit(raw)
print(urlunsplit((u.scheme, u.netloc, f"/{new_db}", u.query, u.fragment)))
PY
}

db_destroy_is_system_database() {
	local name="$1"
	case "${name}" in
	postgres | template0 | template1) return 0 ;;
	esac
	return 1
}

db_destroy_is_out_of_scope_database() {
	local name="$1"
	case "${name}" in
	temporal | temporal_visibility) return 0 ;;
	esac
	return 1
}

db_destroy_local_postgres_container() {
	if command -v docker >/dev/null 2>&1; then
		if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx 'avf-postgres'; then
			echo 'avf-postgres'
			return 0
		fi
	fi
	return 1
}

db_destroy_is_loopback_url() {
	local url="$1"
	local host
	host="$(db_destroy_py -c 'import json,sys; from urllib.parse import urlsplit; print((urlsplit(sys.argv[1]).hostname or "").lower())' "${url}")"
	case "${host}" in
	localhost | 127.0.0.1 | ::1) return 0 ;;
	esac
	return 1
}

db_destroy_run_psql_at() {
	local url="$1"
	local sql="$2"
	local psql_url db_name user container
	psql_url="$(db_destroy_sanitize_psql_url "${url}")"
	db_name="$(db_destroy_py -c 'import sys; from urllib.parse import urlsplit; print((urlsplit(sys.argv[1]).path or "").lstrip("/").split("?")[0])' "${psql_url}")"
	user="$(db_destroy_py -c 'import sys; from urllib.parse import urlsplit, unquote; print(unquote(urlsplit(sys.argv[1]).username or "postgres"))' "${psql_url}")"

	if db_destroy_is_loopback_url "${psql_url}" && container="$(db_destroy_local_postgres_container)"; then
		docker exec -i "${container}" psql -v ON_ERROR_STOP=1 -U "${user}" -d "${db_name}" -Atqc "${sql}"
		return 0
	fi

	if command -v psql >/dev/null 2>&1; then
		psql "${psql_url}" -v ON_ERROR_STOP=1 -Atqc "${sql}"
		return 0
	fi

	# Fallback: tools container; rewrite loopback for Docker Desktop host access.
	if db_destroy_is_loopback_url "${psql_url}"; then
		psql_url="$(db_destroy_py -c 'import sys; from urllib.parse import urlsplit, urlunsplit; u=urlsplit(sys.argv[1]); h=u.hostname or ""; nh="host.docker.internal" if h in ("localhost","127.0.0.1","::1") else h; print(urlunsplit((u.scheme, f"{u.username or \"postgres\"}@{nh}" + (f":{u.port}" if u.port else ""), u.path, u.params, u.query, u.fragment)))' "${psql_url}")"
	fi
	docker run --rm --add-host=host.docker.internal:host-gateway \
		-e "DATABASE_URL=${psql_url}" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${psql_url}" -v ON_ERROR_STOP=1 -Atqc "${sql}"
}

db_destroy_probe_live() {
	local url="$1"
	local result
	if ! result="$(db_destroy_run_psql_at "${url}" "SELECT 1" 2>/dev/null)"; then
		echo "unreachable"
		return 1
	fi
	echo "reachable"
	return 0
}

db_destroy_inventory_database() {
	local url="$1"
	local psql_url
	psql_url="$(db_destroy_sanitize_psql_url "${url}")"
	local sql="
SELECT json_build_object(
  'database', current_database(),
  'pg_version', (SELECT split_part(version(), ' ', 2)),
  'table_count', (SELECT count(*)::int FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'),
  'goose_version', COALESCE((SELECT max(version_id)::text FROM goose_db_version), (SELECT max(version)::text FROM goose_db_version), 'unknown'),
  'size_bytes', pg_database_size(current_database()),
  'has_platform_auth_accounts', to_regclass('public.platform_auth_accounts') IS NOT NULL,
  'has_machines', to_regclass('public.machines') IS NOT NULL
)::text;
"
	db_destroy_run_psql_at "${psql_url}" "${sql}" 2>/dev/null || echo "{}"
}

db_destroy_list_cluster_databases() {
	local maintenance_url="$1"
	db_destroy_run_psql_at "${maintenance_url}" \
		"SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY 1;" 2>/dev/null || true
}

db_destroy_resolve_url_for_target() {
	local target_id="$1"
	local url=""
	case "${target_id}" in
	TARGET-DB-001)
		url="${DATABASE_URL:-}"
		if [[ -z "${url}" ]]; then
			url="postgres://postgres:postgres@127.0.0.1:15432/avf_vending?sslmode=disable"
		fi
		;;
	TARGET-DB-002)
		url="${TEST_DATABASE_URL:-}"
		if [[ -z "${url}" ]]; then
			url="postgres://postgres:postgres@127.0.0.1:15432/avf_vending_test?sslmode=disable"
		fi
		;;
	TARGET-DB-003 | TARGET-DB-004)
		url="${STAGING_DATABASE_URL:-${DATABASE_URL:-}}"
		;;
	TARGET-DB-005 | TARGET-DB-007)
		url="${PRODUCTION_DATABASE_URL:-${DATABASE_URL:-}}"
		;;
	TARGET-DB-006)
		url="${BACKUP_DATABASE_URL:-}"
		;;
	TARGET-DB-008)
		url="${TEST_DATABASE_URL:-}"
		if [[ -n "${url}" ]]; then
			url="$(db_destroy_replace_database "${url}" "avf_vending_test_full_verify")"
		fi
		;;
	*)
		db_destroy_fail "unknown target id: ${target_id}"
		;;
	esac
	printf '%s' "${url}"
}

db_destroy_target_environment() {
	local target_id="$1"
	case "${target_id}" in
	TARGET-DB-001) echo "development" ;;
	TARGET-DB-002 | TARGET-DB-008) echo "test" ;;
	TARGET-DB-003 | TARGET-DB-004) echo "staging" ;;
	TARGET-DB-005 | TARGET-DB-006 | TARGET-DB-007) echo "production" ;;
	*) db_destroy_fail "unknown target id: ${target_id}" ;;
	esac
}

db_destroy_append_evidence() {
	local phase="$1"
	local target_id="$2"
	local status="$3"
	local detail="${4:-}"
	local file="${DB_DESTROY_EVIDENCE_LOG:-${DB_DESTROY_EVIDENCE_DIR}/destruction.jsonl}"
	mkdir -p "$(dirname "${file}")"
	db_destroy_py - "${phase}" "${target_id}" "${status}" "${detail}" "${file}" <<'PY'
import json, sys
from datetime import datetime, timezone

phase, target_id, status, detail, path = sys.argv[1:6]
row = {
    "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "phase": phase,
    "target_id": target_id,
    "status": status,
    "detail": detail,
}
with open(path, "a", encoding="utf-8") as f:
    f.write(json.dumps(row) + "\n")
PY
}

db_destroy_check_staging_production_alias() {
	local staging="${STAGING_DATABASE_URL:-}"
	local production="${PRODUCTION_DATABASE_URL:-}"
	if [[ -n "${staging}" && -n "${production}" && "${staging}" == "${production}" ]]; then
		db_destroy_fail "P0: STAGING_DATABASE_URL equals PRODUCTION_DATABASE_URL"
	fi
}

db_destroy_verify_environment_guard() {
	local target_id="$1"
	local url="$2"
	local app_env
	app_env="$(db_destroy_target_environment "${target_id}")"
	export APP_ENV="${app_env}"
	export DATABASE_URL="${url}"
	export PAYMENT_ENV="${PAYMENT_ENV:-}"
	case "${app_env}" in
	staging) export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}" ;;
	production) export PAYMENT_ENV="${PAYMENT_ENV:-live}" ;;
	development | test) export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}" ;;
	esac
	bash "${DB_DESTROY_REPO_ROOT}/scripts/verify_database_environment.sh" >/dev/null
}

db_destroy_validate_db_name() {
	local name="$1"
	[[ "${name}" =~ ^[a-zA-Z0-9_]+$ ]] || db_destroy_fail "invalid database name: ${name}"
}
