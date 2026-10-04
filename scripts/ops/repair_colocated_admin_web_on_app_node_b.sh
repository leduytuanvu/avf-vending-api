#!/usr/bin/env bash
# Restore https://admin.ldtv.dev when app-node B shares 80/443 with the operator web UI (VPS B).
set -Eeuo pipefail

WEB_DEPLOY_ROOT="${WEB_DEPLOY_ROOT:-/opt/avf-vending-web}"
APP_NODE_DIR="${APP_NODE_DIR:-/opt/avf-vending-api/deployments/prod/app-node}"
ADMIN_DOMAIN="${ADMIN_DOMAIN:-admin.ldtv.dev}"
WEB_REPO="${WEB_REPO:-leduytuanvu/avf-vending-web}"
GHCR_PULL_TOKEN="${GHCR_PULL_TOKEN:-}"

note() { printf '[repair-admin-web] %s\n' "$*"; }

require_cmd() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "error: missing command: $1" >&2
		exit 1
	}
}

require_cmd docker
require_cmd grep

if [[ ! -f "${APP_NODE_DIR}/.env.app-node" ]]; then
	echo "error: missing ${APP_NODE_DIR}/.env.app-node" >&2
	exit 1
fi

project="$(grep -E '^COMPOSE_PROJECT_NAME=' "${APP_NODE_DIR}/.env.app-node" 2>/dev/null | tail -n1 | cut -d= -f2-)"
project="${project:-avf-vending-prod-app-b}"
edge_network="${project}_edge"

note "enable admin vhost on app-node Caddy (project=${project}, edge=${edge_network})"
set_env_kv() {
	local file="$1" key="$2" value="$3"
	if grep -qE "^${key}=" "${file}" 2>/dev/null; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${file}"
	else
		printf '%s=%s\n' "${key}" "${value}" >> "${file}"
	fi
}

set_env_kv "${APP_NODE_DIR}/.env.app-node" "ENABLE_ADMIN_VHOST" "1"
set_env_kv "${APP_NODE_DIR}/.env.app-node" "CADDYFILE_REL_PATH" "../shared/Caddyfile.with-admin"
set_env_kv "${APP_NODE_DIR}/.env.app-node" "ADMIN_DOMAIN" "${ADMIN_DOMAIN}"

mkdir -p "${WEB_DEPLOY_ROOT}"
if [[ ! -f "${WEB_DEPLOY_ROOT}/.env.production" ]]; then
	if [[ -f "${WEB_DEPLOY_ROOT}/.env.production.example" ]]; then
		cp "${WEB_DEPLOY_ROOT}/.env.production.example" "${WEB_DEPLOY_ROOT}/.env.production"
	else
		echo "error: ${WEB_DEPLOY_ROOT}/.env.production missing" >&2
		exit 1
	fi
fi
set_env_kv "${WEB_DEPLOY_ROOT}/.env.production" "WEB_EDGE_NETWORK" "${edge_network}"
set_env_kv "${WEB_DEPLOY_ROOT}/.env.production" "COMPOSE_FILE" "docker-compose.web.yml"

if docker ps -a --format '{{.Names}}' | grep -qx avf-vending-web-caddy; then
	note "stop legacy standalone web Caddy (ports conflict with app-node Caddy)"
	docker stop avf-vending-web-caddy >/dev/null 2>&1 || true
	docker rm avf-vending-web-caddy >/dev/null 2>&1 || true
fi

if [[ -n "${GHCR_PULL_TOKEN}" ]]; then
	note "build web image from ${WEB_REPO}"
	rm -rf /tmp/avf-vending-web-build
	git clone --depth 1 "https://x-access-token:${GHCR_PULL_TOKEN}@github.com/${WEB_REPO}.git" /tmp/avf-vending-web-build
	cd /tmp/avf-vending-web-build
	docker build \
		--build-arg NEXT_PUBLIC_APP_URL="https://${ADMIN_DOMAIN}" \
		--build-arg APP_ENV=production \
		--build-arg AVF_API_BASE_URL=https://api.ldtv.dev \
		--build-arg AVF_OPENAPI_PATH=src/lib/api/generated/schema.ts \
		--build-arg NEXT_PUBLIC_ENABLE_MOCKS=false \
		--build-arg ENABLE_DEV_AUTH_MOCK=false \
		--build-arg SESSION_COOKIE_SECURE=true \
		-t avf-vending-web:deploy .
	set_env_kv "${WEB_DEPLOY_ROOT}/.env.production" "WEB_IMAGE_REF" "avf-vending-web:deploy"
fi

if [[ -f "${WEB_DEPLOY_ROOT}/deploy-web.sh" ]]; then
	note "start integrated web container on ${edge_network}"
	cd "${WEB_DEPLOY_ROOT}"
	COMPOSE_FILE=docker-compose.web.yml ./deploy-web.sh
else
	note "deploy-web.sh missing; starting web via compose only"
	cd "${WEB_DEPLOY_ROOT}"
	docker compose -f docker-compose.web.yml --env-file .env.production up -d
fi

note "recreate app-node Caddy with admin vhost"
cd "${APP_NODE_DIR}"
docker compose --env-file .env.app-node -f docker-compose.app-node.yml up -d --no-deps --force-recreate caddy

note "smoke: curl admin HTTPS via Caddy container"
for i in $(seq 1 18); do
	if docker compose --env-file .env.app-node -f docker-compose.app-node.yml exec -T caddy \
		wget -qO- "https://${ADMIN_DOMAIN}/api/health" 2>/dev/null | grep -q '"status":"ok"'; then
		note "PASS: admin health via TLS"
		exit 0
	fi
	sleep 5
done

echo "error: admin health check failed after recreate" >&2
docker compose --env-file .env.app-node -f docker-compose.app-node.yml logs --tail=60 caddy || true
docker logs --tail=40 avf-vending-web 2>&1 || true
exit 1
