#!/usr/bin/env bash
# Production purge via docker inspect (no .env file or SSH required).
# For self-hosted runner on app-node A with docker group access.
set -Eeuo pipefail

OPS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
REDIS_TOOLS_IMAGE="${REDIS_TOOLS_IMAGE:-redis:7-alpine}"
PHASE="${1:-all}"

fail() { echo "docker-production-purge: error: $*" >&2; exit 1; }
note() { echo "docker-production-purge: $*"; }

require_confirm() {
	[[ "${CONFIRM_PRODUCTION_PURGE:-}" == "I_UNDERSTAND_THIS_WIPES_PRODUCTION" ]] \
		|| fail "set CONFIRM_PRODUCTION_PURGE=I_UNDERSTAND_THIS_WIPES_PRODUCTION"
}

find_container() {
	local pattern="$1"
	docker ps --format '{{.Names}}' | grep -E "${pattern}" | head -n1
}

container_env() {
	local container="$1"
	local key="$2"
	docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
		| grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r'
}

read_runtime_env() {
	API_CONTAINER="$(find_container 'api')"
	[[ -n "${API_CONTAINER}" ]] || fail "api container not found"
	DATABASE_URL="$(container_env "${API_CONTAINER}" DATABASE_URL)"
	REDIS_URL="$(container_env "${API_CONTAINER}" REDIS_URL)"
	API_DOMAIN="$(container_env "${API_CONTAINER}" API_DOMAIN)"
	[[ -n "${DATABASE_URL}" ]] || fail "DATABASE_URL missing in ${API_CONTAINER}"
	note "api container=${API_CONTAINER}"
}

psql_file() {
	local sql_file="$1"
	docker run --rm \
		-e "DATABASE_URL=${DATABASE_URL}" \
		-v "${OPS}:/ops:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "/ops/$(basename "${sql_file}")"
}

psql_query() {
	local query="$1"
	docker run --rm \
		-e "DATABASE_URL=${DATABASE_URL}" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -Atqc "${query}"
}

stop_workers() {
	local name
	while IFS= read -r name; do
		[[ -n "${name}" ]] || continue
		note "stopping ${name}"
		docker stop "${name}" >/dev/null || true
	done < <(docker ps --format '{{.Names}}' | grep -E 'worker|mqtt-ingest|reconciler' || true)
}

start_workers() {
	local name
	while IFS= read -r name; do
		[[ -n "${name}" ]] || continue
		note "starting ${name}"
		docker start "${name}" >/dev/null || true
	done < <(docker ps -a --format '{{.Names}}' | grep -E 'worker|mqtt-ingest|reconciler' || true)
}

phase_postgres() {
	read_runtime_env
	note "dry-run counts"
	psql_file "${OPS}/production-purge-dry-run.sql"
	note "executing purge"
	docker run --rm \
		-e "DATABASE_URL=${DATABASE_URL}" \
		-v "${OPS}:/ops:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 \
		-c "SET avf.confirm_production_purge='I_UNDERSTAND_THIS_WIPES_PRODUCTION';" \
		-f /ops/production-purge-keep-admin.sql
	local machines products accounts
	machines="$(psql_query 'SELECT count(*) FROM machines;')"
	products="$(psql_query 'SELECT count(*) FROM products;')"
	accounts="$(psql_query 'SELECT count(*) FROM platform_auth_accounts;')"
	[[ "${machines}" == "0" ]] || fail "expected 0 machines, got ${machines}"
	[[ "${products}" == "0" ]] || fail "expected 0 products, got ${products}"
	[[ "${accounts}" == "1" ]] || fail "expected 1 auth account, got ${accounts}"
	note "postgres purge verified"
}

phase_redis() {
	read_runtime_env
	[[ -n "${REDIS_URL:-}" ]] || fail "REDIS_URL missing"
	note "flushing redis"
	docker run --rm "${REDIS_TOOLS_IMAGE}" redis-cli -u "${REDIS_URL}" FLUSHDB
}

phase_media() {
	if [[ -n "${CLOUDINARY_CLOUD_NAME:-}" && -n "${CLOUDINARY_API_KEY:-}" && -n "${CLOUDINARY_API_SECRET:-}" ]]; then
		local folder="${CLOUDINARY_FOLDER:-avf-vending/products}"
		note "deleting cloudinary folder ${folder}"
		curl -fsS -X DELETE \
			"https://api.cloudinary.com/v1_1/${CLOUDINARY_CLOUD_NAME}/folders/${folder}" \
			-u "${CLOUDINARY_API_KEY}:${CLOUDINARY_API_SECRET}" \
			|| note "cloudinary folder delete returned non-zero"
	else
		note "cloudinary secrets not set — skip"
	fi
}

phase_emqx() {
	[[ -n "${EMQX_API_KEY:-}" && -n "${EMQX_API_SECRET:-}" ]] || {
		note "EMQX API credentials not set — skip"
		return 0
	}
	local base="${EMQX_MANAGEMENT_URL:-http://127.0.0.1:18083}"
	base="${base%/}/api/v5"
	local auth_path="authentication/password_based%3Abuilt_in_database/users"
	local mqtt_user="${MQTT_USERNAME:-}"
	[[ -n "${mqtt_user}" ]] || mqtt_user="$(container_env "$(find_container 'mqtt-ingest')" MQTT_USERNAME 2>/dev/null || true)"
	command -v jq >/dev/null 2>&1 || fail "jq required for emqx purge"
	local list_json deleted kept user_id code
	list_json="$(curl -fsS -u "${EMQX_API_KEY}:${EMQX_API_SECRET}" "${base}/${auth_path}?limit=1000")"
	deleted=0
	kept=0
	while IFS= read -r user_id; do
		[[ -n "${user_id}" ]] || continue
		if [[ -n "${mqtt_user}" && "${user_id}" == "${mqtt_user}" ]]; then
			note "keep emqx service user ${user_id}"
			kept=$((kept + 1))
			continue
		fi
		code="$(curl -sS -o /dev/null -w "%{http_code}" \
			-u "${EMQX_API_KEY}:${EMQX_API_SECRET}" \
			-X DELETE "${base}/${auth_path}/${user_id}" || true)"
		if [[ "${code}" == "204" || "${code}" == "200" ]]; then
			note "deleted emqx user ${user_id}"
			deleted=$((deleted + 1))
		fi
	done < <(jq -r '.data[].user_id // empty' <<<"${list_json}")
	note "emqx deleted=${deleted} kept=${kept}"
}

phase_verify() {
	read_runtime_env
	curl -fsS "https://${API_DOMAIN:-api.ldtv.dev}/health/live"
	psql_file "${OPS}/production-purge-dry-run.sql"
}

case "${PHASE}" in
preflight)
	require_confirm
	read_runtime_env
	stop_workers
	note "preflight complete (stop workers only; run backup manually if needed)"
	;;
postgres)
	require_confirm
	stop_workers
	phase_postgres
	;;
redis) require_confirm; phase_redis ;;
media) require_confirm; phase_media ;;
emqx) require_confirm; phase_emqx ;;
restart)
	require_confirm
	start_workers
	;;
verify) require_confirm; phase_verify ;;
all)
	require_confirm
	stop_workers
	phase_postgres
	phase_redis
	phase_media
	phase_emqx
	start_workers
	phase_verify
	;;
*)
	fail "unknown phase: ${PHASE}"
	;;
esac

note "phase ${PHASE} complete"
