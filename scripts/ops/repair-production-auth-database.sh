#!/usr/bin/env bash
# Repair production auth login when API DATABASE_URL points at an empty/wrong Postgres database.
# Aligns .env.app-node DATABASE_URL to a database that has platform_auth_accounts, then recreates app workloads.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=../../deployments/prod/shared/scripts/lib_release.sh
source "${REPO_ROOT}/deployments/prod/shared/scripts/lib_release.sh"

ENV_FILE="${APP_NODE_ENV_FILE:-${REPO_ROOT}/deployments/prod/app-node/.env.app-node}"
COMPOSE_FILE="${COMPOSE_FILE:-${REPO_ROOT}/deployments/prod/app-node/docker-compose.app-node.yml}"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
CONFIRM_REPAIR="${CONFIRM_PRODUCTION_AUTH_REPAIR:-}"

fail() { echo "repair-production-auth-database: error: $*" >&2; exit 1; }
note() { echo "repair-production-auth-database: $*"; }

[[ "${CONFIRM_REPAIR}" == "REPAIR_PRODUCTION_AUTH" ]] || \
	fail "set CONFIRM_PRODUCTION_AUTH_REPAIR=REPAIR_PRODUCTION_AUTH to run"
[[ -f "${ENV_FILE}" ]] || fail "env file not found: ${ENV_FILE}"
[[ -f "${COMPOSE_FILE}" ]] || fail "compose file not found: ${COMPOSE_FILE}"

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
	local psql_url exists
	psql_url="$(sanitize_psql_url "${url}")"
	exists="$(docker run --rm \
		-e "DATABASE_URL=${psql_url}" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${psql_url}" -v ON_ERROR_STOP=1 -Atqc \
		"SELECT to_regclass('public.platform_auth_accounts') IS NOT NULL;" 2>/dev/null || echo "f")"
	[[ "${exists}" == "t" ]]
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

candidate_urls() {
	python3 - "$1" <<'PY'
import sys
from urllib.parse import urlparse, urlunparse

raw = sys.argv[1].strip()
u = urlparse(raw)
paths = []
if u.path and u.path != "/":
    paths.append(u.path.lstrip("/"))
seen = set()
for name in paths + ["postgres", "avf_vending_prod"]:
    if not name or name in seen:
        continue
    seen.add(name)
    print(urlunparse((u.scheme, u.netloc, f"/{name}", u.params, u.query, u.fragment)))
PY
}

find_api_container() {
	docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

container_env() {
	local container="$1"
	local key="$2"
	docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
		| grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r'
}

ENV_FILE="${APP_NODE_ENV_FILE:-${REPO_ROOT}/deployments/prod/app-node/.env.app-node}"
load_env_file "${ENV_FILE}"
ENV_DATABASE_URL="${DATABASE_URL:-}"
API_CONTAINER="$(find_api_container || true)"
CONTAINER_DATABASE_URL=""
if [[ -n "${API_CONTAINER}" ]]; then
	CONTAINER_DATABASE_URL="$(container_env "${API_CONTAINER}" DATABASE_URL || true)"
fi

note "env file=${ENV_FILE}"
[[ -n "${ENV_DATABASE_URL}" ]] && note "env DATABASE_URL=$(mask_url "${ENV_DATABASE_URL}")"
[[ -n "${CONTAINER_DATABASE_URL}" ]] && note "container DATABASE_URL=$(mask_url "${CONTAINER_DATABASE_URL}")"

working_url=""
try_candidate() {
	local candidate="$1"
	[[ -n "${candidate}" ]] || return 1
	if table_exists "${candidate}"; then
		working_url="${candidate}"
		return 0
	fi
	return 1
}

for seed in "${ENV_DATABASE_URL}" "${CONTAINER_DATABASE_URL}"; do
	try_candidate "${seed}" && break
done
if [[ -z "${working_url}" ]]; then
	while IFS= read -r candidate; do
		try_candidate "${candidate}" && break
	done < <(candidate_urls "${ENV_DATABASE_URL:-${CONTAINER_DATABASE_URL}}")
fi

[[ -n "${working_url}" ]] || fail "no candidate DATABASE_URL has platform_auth_accounts; restore DB backup or run goose on correct database"

count="$(account_count "${working_url}")"
note "selected database=$(mask_url "${working_url}") accounts=${count}"

if [[ "${ENV_DATABASE_URL}" != "${working_url}" ]]; then
	note "updating DATABASE_URL in ${ENV_FILE}"
	python3 - "${ENV_FILE}" "${working_url}" <<'PY'
import pathlib, sys

path = pathlib.Path(sys.argv[1])
new_url = sys.argv[2].strip()
lines = path.read_text(encoding="utf-8").splitlines()
out = []
replaced = False
for line in lines:
    if line.startswith("DATABASE_URL="):
        out.append(f"DATABASE_URL={new_url}")
        replaced = True
    else:
        out.append(line)
if not replaced:
    out.append(f"DATABASE_URL={new_url}")
path.write_text("\n".join(out) + "\n", encoding="utf-8")
PY
else
	note "env DATABASE_URL already aligned; skipping env rewrite"
fi

COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
	COMPOSE+=(--project-name "${COMPOSE_PROJECT_NAME}")
fi

note "recreating api workloads with aligned DATABASE_URL"
"${COMPOSE[@]}" up -d --remove-orphans --force-recreate api worker reconciler mqtt-ingest

bash "${SCRIPT_DIR}/verify-production-auth-schema.sh"

note "PASS repair complete"
