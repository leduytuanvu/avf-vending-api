#!/usr/bin/env bash
# Production release helper: deploy, rollback, status, logs (image-only compose; no source builds).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

ENV_FILE="${ROOT}/.env.production"
COMPOSE_FILE="${ROOT}/docker-compose.prod.yml"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
STATE="${ROOT}/.deploy"
LONG_LIVED_SERVICES=(postgres nats emqx api worker mqtt-ingest reconciler caddy)
ARTIFACT_SERVICES=(migrate api worker mqtt-ingest reconciler)

fail() {
	echo "release.sh: error: $*" >&2
	exit 1
}

note() {
	echo "==> $*"
}

usage() {
	cat <<'USAGE' >&2
usage:
  release.sh deploy <tag>
  release.sh rollback [tag]
  release.sh status
  release.sh logs [service] [tail]

  <tag>     Image tag for APP_IMAGE_TAG and GOOSE_IMAGE_TAG (e.g. sha-abc123...).
  [tag]     For rollback: optional; defaults to last recorded previous tag.
  [service] For logs: optional docker compose service name; omit to tail all services.
  [tail]    For logs: line count (default 200).

Environment (deploy / rollback):
  Required in .env.production (or exported before deploy for registry/repo overrides):
    IMAGE_REGISTRY, APP_IMAGE_REPOSITORY, GOOSE_IMAGE_REPOSITORY, DATABASE_URL, POSTGRES_*,
    EMQX_*, MQTT_CLIENT_ID_API, CADDY_ACME_EMAIL, ...

  Optional registry pull (VPS):
    GHCR_PULL_USERNAME, GHCR_PULL_TOKEN — if both set, runs docker login ghcr.io before pull.

  Optional after deploy:
    SKIP_SMOKE=1 — skip scripts/healthcheck_prod.sh
USAGE
	exit 1
}

read_env_file() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" "${ENV_FILE}" 2>/dev/null | tail -n1 || true)"
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
	read_env_file "${key}" || return 1
}

require_env_resolved() {
	local key="$1"
	local value
	if ! value="$(resolve_env "${key}")" || [[ -z "${value}" ]]; then
		fail "missing required ${key} (set in ${ENV_FILE} or export in environment)"
	fi
	printf '%s' "${value}"
}

set_env_value() {
	local key="$1"
	local value="$2"
	local file="$3"

	if grep -q "^${key}=" "${file}"; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${file}"
	else
		printf '%s=%s\n' "${key}" "${value}" >>"${file}"
	fi
}

compose_supports_wait() {
	"${COMPOSE[@]}" up --help 2>/dev/null | grep -qE '(^|[[:space:]])--wait([[:space:]]|$)'
}

registry_login_optional() {
	if [[ -n "${GHCR_PULL_USERNAME:-}" ]] && [[ -n "${GHCR_PULL_TOKEN:-}" ]]; then
		note "docker login ghcr.io (GHCR_PULL_USERNAME / GHCR_PULL_TOKEN)"
		printf '%s' "${GHCR_PULL_TOKEN}" | docker login ghcr.io -u "${GHCR_PULL_USERNAME}" --password-stdin
	elif [[ -n "${GHCR_PULL_USERNAME:-}" ]] || [[ -n "${GHCR_PULL_TOKEN:-}" ]]; then
		fail "set both GHCR_PULL_USERNAME and GHCR_PULL_TOKEN, or neither, for registry login"
	fi
}

record_deploy_state() {
	local new_tag="$1"
	mkdir -p "${STATE}"
	if [[ -f "${STATE}/current_image_tag" ]]; then
		local prev
		prev="$(tr -d '\r\n' <"${STATE}/current_image_tag" || true)"
		if [[ -n "${prev}" ]] && [[ "${prev}" != "${new_tag}" ]]; then
			printf '%s\n' "${prev}" >"${STATE}/previous_image_tag"
		fi
	fi
	printf '%s\n' "${new_tag}" >"${STATE}/current_image_tag"
	printf '%s\n' "${new_tag}" >"${STATE}/current_app_image_tag"
	printf '%s\n' "${new_tag}" >"${STATE}/current_goose_image_tag"
	printf '%s\t%s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "deploy ${new_tag}" >>"${STATE}/history.log"
}

record_rollback_state() {
	local tag="$1"
	mkdir -p "${STATE}"
	printf '%s\n' "${tag}" >"${STATE}/current_image_tag"
	printf '%s\n' "${tag}" >"${STATE}/current_app_image_tag"
	printf '%s\n' "${tag}" >"${STATE}/current_goose_image_tag"
	printf '%s\trollback %s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "${tag}" >>"${STATE}/history.log"
}

verify_long_lived_running() {
	local c state
	for c in avf-prod-postgres avf-prod-nats avf-prod-emqx avf-prod-api avf-prod-worker avf-prod-mqtt-ingest avf-prod-reconciler avf-prod-caddy; do
		state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
		if [[ "${state}" != "running" ]]; then
			fail "container ${c} is not running (state=${state}) — inspect: docker logs ${c}"
		fi
	done
}

cmd_deploy() {
	local tag="${1:-}"
	[[ -n "${tag}" ]] || fail "deploy requires <tag>"

	[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"

	note "validate required image / stack env"
	require_env_resolved IMAGE_REGISTRY >/dev/null
	require_env_resolved APP_IMAGE_REPOSITORY >/dev/null
	require_env_resolved GOOSE_IMAGE_REPOSITORY >/dev/null
	require_env_resolved DATABASE_URL >/dev/null

	note "write image tags and registry metadata to ${ENV_FILE}"
	# Prefer exported CI/VPS env over existing file values when set.
	local reg app goose
	reg="$(require_env_resolved IMAGE_REGISTRY)"
	app="$(require_env_resolved APP_IMAGE_REPOSITORY)"
	goose="$(require_env_resolved GOOSE_IMAGE_REPOSITORY)"
	set_env_value "IMAGE_REGISTRY" "${reg}" "${ENV_FILE}"
	set_env_value "APP_IMAGE_REPOSITORY" "${app}" "${ENV_FILE}"
	set_env_value "GOOSE_IMAGE_REPOSITORY" "${goose}" "${ENV_FILE}"
	set_env_value "APP_IMAGE_TAG" "${tag}" "${ENV_FILE}"
	set_env_value "GOOSE_IMAGE_TAG" "${tag}" "${ENV_FILE}"
	set_env_value "IMAGE_TAG" "${tag}" "${ENV_FILE}"

	registry_login_optional

	note "docker compose config"
	if ! "${COMPOSE[@]}" config >/dev/null; then
		fail "docker compose config failed — fix ${COMPOSE_FILE} or ${ENV_FILE}"
	fi

	note "docker compose pull (app + goose images)"
	if ! "${COMPOSE[@]}" pull "${ARTIFACT_SERVICES[@]}"; then
		fail "docker compose pull failed — check registry auth and tags"
	fi

	note "start data plane (postgres, nats, emqx) before migrations"
	if ! "${COMPOSE[@]}" up -d postgres nats emqx; then
		fail "failed to start postgres/nats/emqx"
	fi

	note "docker compose up migrate (foreground; one-shot migration)"
	# Ensures migrations run before application containers are switched.
	if ! "${COMPOSE[@]}" up migrate; then
		fail "migrate failed — fix database before retrying"
	fi

	note "EMQX MQTT user bootstrap"
	for _ in $(seq 1 90); do
		if "${COMPOSE[@]}" exec -T emqx emqx_ctl status >/dev/null 2>&1; then
			break
		fi
		sleep 2
	done
	if ! bash "${ROOT}/scripts/emqx_bootstrap.sh"; then
		fail "emqx_bootstrap.sh failed"
	fi

	note "docker compose up -d (${LONG_LIVED_SERVICES[*]})"
	if compose_supports_wait; then
		if ! "${COMPOSE[@]}" up -d --wait --wait-timeout 300 --remove-orphans "${LONG_LIVED_SERVICES[@]}"; then
			fail "compose up --wait failed"
		fi
	else
		if ! "${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"; then
			fail "compose up failed"
		fi
	fi

	verify_long_lived_running

	note "post-deploy health checks"
	if [[ "${SKIP_SMOKE:-0}" != "1" ]]; then
		if ! bash "${ROOT}/scripts/healthcheck_prod.sh"; then
			fail "healthcheck_prod.sh failed"
		fi
	fi

	record_deploy_state "${tag}"
	note "deploy complete (tag=${tag})"
}

cmd_rollback() {
	local tag="${1:-}"

	[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"

	if [[ -z "${tag}" ]]; then
		mkdir -p "${STATE}"
		local prev_file="${STATE}/previous_image_tag"
		[[ -f "${prev_file}" ]] || fail "no previous tag recorded at ${prev_file}; pass an explicit tag: rollback <tag>"
		tag="$(tr -d '\r\n' <"${prev_file}")"
		[[ -n "${tag}" ]] || fail "previous_image_tag is empty"
	fi

	note "rollback to tag=${tag}"
	require_env_resolved IMAGE_REGISTRY >/dev/null
	require_env_resolved APP_IMAGE_REPOSITORY >/dev/null
	require_env_resolved GOOSE_IMAGE_REPOSITORY >/dev/null

	set_env_value "APP_IMAGE_TAG" "${tag}" "${ENV_FILE}"
	set_env_value "GOOSE_IMAGE_TAG" "${tag}" "${ENV_FILE}"
	set_env_value "IMAGE_TAG" "${tag}" "${ENV_FILE}"

	registry_login_optional

	note "docker compose config"
	"${COMPOSE[@]}" config >/dev/null

	note "docker compose pull"
	"${COMPOSE[@]}" pull "${ARTIFACT_SERVICES[@]}"

	note "docker compose up -d (${LONG_LIVED_SERVICES[*]})"
	if compose_supports_wait; then
		"${COMPOSE[@]}" up -d --wait --wait-timeout 300 --remove-orphans "${LONG_LIVED_SERVICES[@]}"
	else
		"${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"
	fi

	verify_long_lived_running

	if [[ "${SKIP_SMOKE:-0}" != "1" ]]; then
		note "post-rollback health checks"
		bash "${ROOT}/scripts/healthcheck_prod.sh"
	fi

	record_rollback_state "${tag}"
	note "rollback complete (tag=${tag})"
}

cmd_status() {
	note "release state (${STATE})"
	if [[ -d "${STATE}" ]]; then
		for f in current_image_tag previous_image_tag current_app_image_tag current_goose_image_tag; do
			if [[ -f "${STATE}/${f}" ]]; then
				printf '  %s: %s\n' "${f}" "$(tr -d '\r\n' <"${STATE}/${f}")"
			fi
		done
		if [[ -f "${STATE}/history.log" ]]; then
			echo "  history (last 10 lines):"
			tail -n 10 "${STATE}/history.log" | sed 's/^/    /'
		fi
	else
		echo "  (no .deploy state yet)"
	fi
	echo ""
	note "docker compose ps"
	"${COMPOSE[@]}" ps
}

cmd_logs() {
	local service="${1:-}"
	local tail_n="${2:-200}"
	[[ "${tail_n}" =~ ^[0-9]+$ ]] || fail "tail must be a positive integer, got: ${tail_n}"

	if [[ -z "${service}" ]]; then
		note "docker compose logs --tail=${tail_n} (all services)"
		"${COMPOSE[@]}" logs --tail="${tail_n}" "${LONG_LIVED_SERVICES[@]}"
	else
		note "docker compose logs --tail=${tail_n} ${service}"
		"${COMPOSE[@]}" logs --tail="${tail_n}" "${service}"
	fi
}

main() {
	local sub="${1:-}"
	[[ -n "${sub}" ]] || usage
	shift

	case "${sub}" in
		deploy)
			cmd_deploy "${1:-}"
			;;
		rollback)
			cmd_rollback "${1:-}"
			;;
		status)
			cmd_status
			;;
		logs)
			cmd_logs "${1:-}" "${2:-}"
			;;
		-h | --help | help)
			usage
			;;
		*)
			fail "unknown subcommand: ${sub}"
			;;
	esac
}

main "$@"
