#!/usr/bin/env bash
# First-time or full rollout: build images, run DB migrations (fail-fast), bootstrap EMQX MQTT user, bring stack up.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ ! -f .env.production ]]; then
	echo "error: copy .env.production.example -> .env.production and fill secrets" >&2
	exit 1
fi

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" .env.production | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		return 1
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

STATE="${ROOT}/.deploy"
mkdir -p "${STATE}"
if [[ -f "${STATE}/current_image_tag" ]]; then
	cp -f "${STATE}/current_image_tag" "${STATE}/previous_image_tag"
fi

COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)
LONG_LIVED_SERVICES=(postgres nats emqx api worker mqtt-ingest reconciler caddy)

echo "==> compose config (syntax check)"
"${COMPOSE[@]}" config >/dev/null

echo "==> build images (api binaries + goose)"
"${COMPOSE[@]}" build migrate api

echo "==> start data plane (postgres, nats, emqx)"
"${COMPOSE[@]}" up -d postgres nats emqx

echo "==> run database migrations (goose; aborts stack rollout on failure)"
if ! "${COMPOSE[@]}" run --rm migrate; then
	echo "error: migrations failed — fix DB state before retrying; see README rollback/migration notes" >&2
	exit 1
fi

echo "==> EMQX MQTT user bootstrap (before api / mqtt-ingest connect)"
for _ in $(seq 1 90); do
	if "${COMPOSE[@]}" exec -T emqx emqx_ctl status >/dev/null 2>&1; then
		break
	fi
	sleep 2
done
bash "${ROOT}/scripts/emqx_bootstrap.sh"

echo "==> start application stack + Caddy"
"${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"

TAG="${IMAGE_TAG:-$(read_env IMAGE_TAG || true)}"
TAG="${TAG:-local}"
echo "${TAG}" >"${STATE}/current_image_tag"
echo "deploy_prod: recorded IMAGE_TAG=${TAG} in ${STATE}/current_image_tag"

echo "==> optional smoke (set SKIP_SMOKE=1 to skip)"
if [[ "${SKIP_SMOKE:-0}" != "1" ]]; then
	bash "${ROOT}/scripts/healthcheck_prod.sh"
fi

echo "deploy_prod: done"
