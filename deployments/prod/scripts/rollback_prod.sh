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

export IMAGE_TAG="${TAG}"
COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)
LONG_LIVED_SERVICES=(postgres nats emqx api worker mqtt-ingest reconciler caddy)

echo "rollback_prod: IMAGE_TAG=${IMAGE_TAG}"
"${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"
echo "${IMAGE_TAG}" >"${ROOT}/.deploy/current_image_tag"

echo "rollback_prod: done (DB schema unchanged — verify migrations if needed)"
