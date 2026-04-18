#!/usr/bin/env bash
# Roll back to the previously recorded IMAGE_TAG (written by deploy_prod.sh).
# This does not reverse database migrations — plan DB rollback separately if schema changed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ ! -f .env.production ]]; then
	echo "error: missing .env.production" >&2
	exit 1
fi

set_env_value() {
	local key="$1"
	local value="$2"
	local file="$3"

	if grep -q "^${key}=" "${file}"; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${file}"
	else
		printf '%s=%s\n' "${key}" "${value}" >> "${file}"
	fi
}

PREV="${ROOT}/.deploy/previous_image_tag"
if [[ ! -f "${PREV}" ]]; then
	echo "error: missing ${PREV} — nothing to roll back to (run deploy_prod.sh at least twice)." >&2
	exit 1
fi

TAG="$(tr -d '\r\n' <"${PREV}")"
if [[ -z "${TAG}" ]]; then
	echo "error: previous_image_tag is empty" >&2
	exit 1
fi

COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)
LONG_LIVED_SERVICES=(postgres nats emqx api worker mqtt-ingest reconciler caddy)

compose_supports_wait() {
	"${COMPOSE[@]}" up --help 2>/dev/null | grep -qE '(^|[[:space:]])--wait([[:space:]]|$)'
}

set_env_value "APP_IMAGE_TAG" "${TAG}" "${ROOT}/.env.production"
set_env_value "GOOSE_IMAGE_TAG" "${TAG}" "${ROOT}/.env.production"
set_env_value "IMAGE_TAG" "${TAG}" "${ROOT}/.env.production"

echo "rollback_prod: APP_IMAGE_TAG=${TAG}"
if compose_supports_wait; then
	"${COMPOSE[@]}" up -d --wait --wait-timeout 300 --remove-orphans "${LONG_LIVED_SERVICES[@]}"
else
	"${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"
fi
echo "${TAG}" >"${ROOT}/.deploy/current_image_tag"
echo "${TAG}" >"${ROOT}/.deploy/current_app_image_tag"
echo "${TAG}" >"${ROOT}/.deploy/current_goose_image_tag"

echo "rollback_prod: done (DB schema unchanged — verify migrations if needed)"
