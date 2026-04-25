#!/usr/bin/env bash
# First-time or full rollout for staging: pull prebuilt images, run DB migrations, bootstrap EMQX, bring stack up.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DEPLOY_SCRIPT_ROOT="${ROOT}/scripts"

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

require_non_latest_ref() {
	local label="$1"
	local ref="$2"
	[[ -n "${ref}" ]] || fail "missing required ${label}"
	if [[ "${ref}" == *":latest" ]]; then
		fail "${label} must not use the latest tag"
	fi
}

STATE="${ROOT}/.deploy"
mkdir -p "${STATE}"
COMPOSE=(docker compose --env-file .env.staging -f docker-compose.staging.yml)
LONG_LIVED_SERVICES=(postgres nats emqx api worker mqtt-ingest reconciler caddy)
ARTIFACT_SERVICES=(migrate api worker mqtt-ingest reconciler)

IMAGE_REGISTRY="$(require_env IMAGE_REGISTRY)"
APP_IMAGE_REPOSITORY="$(require_env APP_IMAGE_REPOSITORY)"
GOOSE_IMAGE_REPOSITORY="$(require_env GOOSE_IMAGE_REPOSITORY)"
APP_IMAGE_TAG="$(resolve_env APP_IMAGE_TAG || true)"
GOOSE_IMAGE_TAG="$(resolve_env GOOSE_IMAGE_TAG || true)"
APP_IMAGE_REF="$(resolve_env APP_IMAGE_REF || true)"
GOOSE_IMAGE_REF="$(resolve_env GOOSE_IMAGE_REF || true)"

if [[ -z "${APP_IMAGE_REF}" ]]; then
	[[ -n "${APP_IMAGE_TAG}" ]] || fail "set APP_IMAGE_REF or APP_IMAGE_TAG in .env.staging"
	APP_IMAGE_REF="${IMAGE_REGISTRY}/${APP_IMAGE_REPOSITORY}:${APP_IMAGE_TAG}"
fi

if [[ -z "${GOOSE_IMAGE_REF}" ]]; then
	[[ -n "${GOOSE_IMAGE_TAG}" ]] || fail "set GOOSE_IMAGE_REF or GOOSE_IMAGE_TAG in .env.staging"
	GOOSE_IMAGE_REF="${IMAGE_REGISTRY}/${GOOSE_IMAGE_REPOSITORY}:${GOOSE_IMAGE_TAG}"
fi

require_non_latest_ref "APP_IMAGE_REF" "${APP_IMAGE_REF}"
require_non_latest_ref "GOOSE_IMAGE_REF" "${GOOSE_IMAGE_REF}"
export APP_IMAGE_REF GOOSE_IMAGE_REF

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

echo "==> verify database environment (guard; redacted identity only)"
REPO_ROOT="$(cd "${ROOT}/../.." && pwd)"
set -a
# shellcheck source=/dev/null
source .env.staging
set +a
export APP_ENV="${APP_ENV:-staging}"
bash "${REPO_ROOT}/scripts/verify_database_environment.sh"

echo "==> run database migrations (goose; aborts stack rollout on failure)"
if ! "${COMPOSE[@]}" run --rm migrate; then
	fail "migrations failed — fix DB state before retrying"
fi

echo "==> EMQX MQTT user bootstrap (before api / mqtt-ingest connect)"
for _ in $(seq 1 90); do
	if "${COMPOSE[@]}" exec -T emqx bash -lc 'exec 3<>/dev/tcp/127.0.0.1/18083; printf %b "GET /api/v5/status HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n" >&3; grep -Fq "emqx is running" <&3' >/dev/null 2>&1; then
		break
	fi
	sleep 2
done
bash "${ROOT}/scripts/emqx_bootstrap.sh"

echo "==> start application stack + Caddy"
"${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"

echo "==> optional post-deploy validation (set SKIP_SMOKE=1 to skip)"
if [[ "${SKIP_SMOKE:-0}" != "1" ]]; then
	bash "${DEPLOY_SCRIPT_ROOT}/healthcheck_staging.sh"
	bash "${DEPLOY_SCRIPT_ROOT}/smoke_staging.sh"
fi

if [[ -n "${PREVIOUS_TAG}" ]]; then
	echo "${PREVIOUS_TAG}" >"${STATE}/previous_image_tag"
fi
if [[ -n "${APP_IMAGE_TAG}" ]]; then
	echo "${APP_IMAGE_TAG}" >"${STATE}/current_image_tag"
	echo "${APP_IMAGE_TAG}" >"${STATE}/current_app_image_tag"
	echo "deploy_staging: recorded APP_IMAGE_TAG=${APP_IMAGE_TAG} in ${STATE}/current_image_tag"
fi
if [[ -n "${GOOSE_IMAGE_TAG}" ]]; then
	echo "${GOOSE_IMAGE_TAG}" >"${STATE}/current_goose_image_tag"
fi
echo "${APP_IMAGE_REF}" >"${STATE}/current_app_image_ref"
echo "${GOOSE_IMAGE_REF}" >"${STATE}/current_goose_image_ref"
echo "deploy_staging: recorded app/goose refs in ${STATE}/current_app_image_ref and ${STATE}/current_goose_image_ref"

if [[ -n "${APP_IMAGE_TAG}" ]] && [[ -n "${GOOSE_IMAGE_TAG}" ]] && [[ "${APP_IMAGE_TAG}" != "${GOOSE_IMAGE_TAG}" ]]; then
	echo "deploy_staging: warning: staging rollback still tracks a single legacy tag; app=${APP_IMAGE_TAG}, goose=${GOOSE_IMAGE_TAG}" >&2
fi

echo "deploy_staging: done"
