#!/usr/bin/env bash
# First-time or full rollout for staging: pull prebuilt images, run DB migrations, bootstrap EMQX, bring stack up.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

fail() {
	echo "error: $*" >&2
	exit 1
}

if [[ ! -f .env.staging ]]; then
	fail "copy .env.staging.example -> .env.staging and fill secrets"
fi

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" .env.staging | tail -n1 || true)"
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

resolve_env() {
	local key="$1"
	local value="${!key:-}"
	if [[ -n "${value}" ]]; then
		printf '%s' "${value}"
		return 0
	fi
	read_env "${key}"
}

require_env() {
	local key="$1"
	local value
	if ! value="$(resolve_env "${key}")" || [[ -z "${value}" ]]; then
		fail "missing required ${key} in .env.staging"
	fi
	printf '%s' "${value}"
}

STATE="${ROOT}/.deploy"
mkdir -p "${STATE}"
COMPOSE=(docker compose --env-file .env.staging -f docker-compose.staging.yml)
LONG_LIVED_SERVICES=(postgres nats emqx api worker mqtt-ingest reconciler caddy)
ARTIFACT_SERVICES=(migrate api worker mqtt-ingest reconciler)

IMAGE_REGISTRY="$(require_env IMAGE_REGISTRY)"
APP_IMAGE_REPOSITORY="$(require_env APP_IMAGE_REPOSITORY)"
APP_IMAGE_TAG="$(require_env APP_IMAGE_TAG)"
GOOSE_IMAGE_REPOSITORY="$(require_env GOOSE_IMAGE_REPOSITORY)"
GOOSE_IMAGE_TAG="$(require_env GOOSE_IMAGE_TAG)"
LEGACY_IMAGE_TAG="$(resolve_env IMAGE_TAG || true)"

if [[ -n "${LEGACY_IMAGE_TAG}" ]] && [[ "${LEGACY_IMAGE_TAG}" != "${APP_IMAGE_TAG}" ]]; then
	fail "IMAGE_TAG=${LEGACY_IMAGE_TAG} must match APP_IMAGE_TAG=${APP_IMAGE_TAG} while legacy rollback tracking still depends on a single app tag"
fi

APP_IMAGE_REF="${IMAGE_REGISTRY}/${APP_IMAGE_REPOSITORY}:${APP_IMAGE_TAG}"
GOOSE_IMAGE_REF="${IMAGE_REGISTRY}/${GOOSE_IMAGE_REPOSITORY}:${GOOSE_IMAGE_TAG}"
PREVIOUS_TAG=""
if [[ -f "${STATE}/current_image_tag" ]]; then
	PREVIOUS_TAG="$(<"${STATE}/current_image_tag")"
fi

echo "==> compose config (syntax check)"
"${COMPOSE[@]}" config >/dev/null

echo "==> artifact refs"
echo "    app:   ${APP_IMAGE_REF}"
echo "    goose: ${GOOSE_IMAGE_REF}"

echo "==> pull prebuilt images (app + goose)"
"${COMPOSE[@]}" pull "${ARTIFACT_SERVICES[@]}"

echo "==> start data plane (postgres, nats, emqx)"
"${COMPOSE[@]}" up -d postgres nats emqx

echo "==> run database migrations (goose; aborts stack rollout on failure)"
if ! "${COMPOSE[@]}" run --rm migrate; then
	fail "migrations failed — fix DB state before retrying"
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

echo "==> optional smoke (set SKIP_SMOKE=1 to skip)"
if [[ "${SKIP_SMOKE:-0}" != "1" ]]; then
	bash "${ROOT}/scripts/healthcheck_staging.sh"
fi

if [[ -n "${PREVIOUS_TAG}" ]]; then
	echo "${PREVIOUS_TAG}" >"${STATE}/previous_image_tag"
fi
echo "${APP_IMAGE_TAG}" >"${STATE}/current_image_tag"
echo "${APP_IMAGE_TAG}" >"${STATE}/current_app_image_tag"
echo "${GOOSE_IMAGE_TAG}" >"${STATE}/current_goose_image_tag"
echo "deploy_staging: recorded APP_IMAGE_TAG=${APP_IMAGE_TAG} in ${STATE}/current_image_tag"
echo "deploy_staging: recorded app/goose tags in ${STATE}/current_app_image_tag and ${STATE}/current_goose_image_tag"

if [[ "${APP_IMAGE_TAG}" != "${GOOSE_IMAGE_TAG}" ]]; then
	echo "deploy_staging: warning: staging rollback still tracks a single legacy tag; app=${APP_IMAGE_TAG}, goose=${GOOSE_IMAGE_TAG}" >&2
fi

echo "deploy_staging: done"
