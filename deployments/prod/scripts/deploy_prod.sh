#!/usr/bin/env bash
# First-time or full rollout: pull prebuilt images, run DB migrations (fail-fast), bootstrap EMQX MQTT user, bring stack up.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

fail() {
	echo "error: $*" >&2
	exit 1
}

log_phase() {
	echo "==> $*"
}

compose_supports_wait() {
	"${COMPOSE[@]}" up --help 2>/dev/null | grep -qE '(^|[[:space:]])--wait([[:space:]]|$)'
}

verify_long_lived_running() {
	local c state
	for c in avf-prod-postgres avf-prod-nats avf-prod-emqx avf-prod-api avf-prod-worker avf-prod-mqtt-ingest avf-prod-reconciler avf-prod-caddy; do
		state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
		if [[ "${state}" != "running" ]]; then
			fail "phase application stack: container ${c} is not running (state=${state}) — inspect logs with: docker logs ${c}"
		fi
	done
}

if [[ ! -f .env.production ]]; then
	fail "copy .env.production.example -> .env.production and fill secrets"
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
		fail "missing required ${key} in .env.production"
	fi
	printf '%s' "${value}"
}

STATE="${ROOT}/.deploy"
mkdir -p "${STATE}"
COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)
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

log_phase "compose config (syntax check)"
if ! "${COMPOSE[@]}" config >/dev/null; then
	fail "phase compose config: docker compose config failed — fix docker-compose.prod.yml or .env.production"
fi

log_phase "artifact refs"
echo "    app:   ${APP_IMAGE_REF}"
echo "    goose: ${GOOSE_IMAGE_REF}"

log_phase "pull prebuilt images (app + goose)"
if ! "${COMPOSE[@]}" pull "${ARTIFACT_SERVICES[@]}"; then
	fail "phase image pull: docker compose pull failed — check registry auth (docker login) and image tags in .env.production"
fi

log_phase "start data plane (postgres, nats, emqx)"
if ! "${COMPOSE[@]}" up -d postgres nats emqx; then
	fail "phase data plane: failed to start postgres/nats/emqx — inspect: docker compose --env-file .env.production -f docker-compose.prod.yml ps"
fi

log_phase "run database migrations (goose; aborts stack rollout on failure)"
if ! "${COMPOSE[@]}" run --rm migrate; then
	fail "phase migrations: goose up failed — fix DB state before retrying; see README rollback/migration notes"
fi

log_phase "EMQX MQTT user bootstrap (before api / mqtt-ingest connect)"
for _ in $(seq 1 90); do
	if "${COMPOSE[@]}" exec -T emqx emqx_ctl status >/dev/null 2>&1; then
		break
	fi
	sleep 2
done
if ! bash "${ROOT}/scripts/emqx_bootstrap.sh"; then
	fail "phase emqx bootstrap: emqx_bootstrap.sh failed — MQTT clients cannot authenticate until this succeeds"
fi

log_phase "start application stack + Caddy (api, worker, mqtt-ingest, reconciler, caddy)"
if compose_supports_wait; then
	echo "deploy_prod: docker compose supports --wait; waiting for service healthchecks (up to 300s)"
	if ! "${COMPOSE[@]}" up -d --wait --wait-timeout 300 --remove-orphans "${LONG_LIVED_SERVICES[@]}"; then
		fail "phase application stack: compose up --wait failed — inspect: docker compose --env-file .env.production -f docker-compose.prod.yml ps"
	fi
else
	echo "deploy_prod: docker compose has no --wait; starting stack without blocking on healthchecks"
	if ! "${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"; then
		fail "phase application stack: compose up failed — inspect: docker compose --env-file .env.production -f docker-compose.prod.yml ps"
	fi
fi

verify_long_lived_running

log_phase "optional smoke (set SKIP_SMOKE=1 to skip)"
if [[ "${SKIP_SMOKE:-0}" != "1" ]]; then
	if ! bash "${ROOT}/scripts/healthcheck_prod.sh"; then
		fail "phase smoke tests: healthcheck_prod.sh failed — fix failing checks before treating this deploy as successful"
	fi
fi

if [[ -n "${PREVIOUS_TAG}" ]]; then
	echo "${PREVIOUS_TAG}" >"${STATE}/previous_image_tag"
fi
echo "${APP_IMAGE_TAG}" >"${STATE}/current_image_tag"
echo "${APP_IMAGE_TAG}" >"${STATE}/current_app_image_tag"
echo "${GOOSE_IMAGE_TAG}" >"${STATE}/current_goose_image_tag"
echo "deploy_prod: recorded APP_IMAGE_TAG=${APP_IMAGE_TAG} in ${STATE}/current_image_tag"
echo "deploy_prod: recorded app/goose tags in ${STATE}/current_app_image_tag and ${STATE}/current_goose_image_tag"

if [[ "${APP_IMAGE_TAG}" != "${GOOSE_IMAGE_TAG}" ]]; then
	echo "deploy_prod: warning: rollback_prod.sh still tracks a single legacy tag; app=${APP_IMAGE_TAG}, goose=${GOOSE_IMAGE_TAG}" >&2
fi

echo "deploy_prod: done"
