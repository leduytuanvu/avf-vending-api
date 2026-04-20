#!/usr/bin/env bash
# Production release helper: deploy, rollback, status, logs (image-only compose; no source builds).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

ENV_FILE="${ROOT}/.env.production"
COMPOSE_FILE="${ROOT}/docker-compose.prod.yml"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
STATE="${ROOT}/.deploy"
DEFAULT_API_KEY_FILE="${ROOT}/emqx/default_api_key.conf"
LONG_LIVED_SERVICES=(postgres nats emqx api worker mqtt-ingest reconciler caddy)
ARTIFACT_SERVICES=(migrate api worker mqtt-ingest reconciler)
# Monitored after `compose up`: running required; Docker health=healthy only when a healthcheck exists.
ROLLUP_GATE_SVCS=(postgres nats emqx api worker mqtt-ingest reconciler caddy)
ROLLUP_GATE_CTRS=(avf-prod-postgres avf-prod-nats avf-prod-emqx avf-prod-api avf-prod-worker avf-prod-mqtt-ingest avf-prod-reconciler avf-prod-caddy)
EMQX_API_KEY_RESOLVED=""
EMQX_API_SECRET_RESOLVED=""

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
  release.sh deploy [app_tag [goose_tag]]
  release.sh rollback [app_tag [goose_tag]]
  release.sh status
  release.sh logs [service] [tail]

  deploy: With args, sets APP_IMAGE_TAG and GOOSE_IMAGE_TAG (goose defaults to app). With no args,
          reads APP_IMAGE_TAG or falls back to IMAGE_TAG; GOOSE_IMAGE_TAG or IMAGE_TAG or same as app.
  rollback: With no args, uses .deploy/previous_app_image_tag and previous_goose_image_tag
          (or legacy previous_image_tag for both when goose file is absent).
  [service] For logs: optional docker compose service name; omit to tail all services.
  [tail]    For logs: line count (default 200).

Environment (deploy / rollback):
  Required in .env.production (or exported before deploy for registry/repo overrides):
    IMAGE_REGISTRY, APP_IMAGE_REPOSITORY, GOOSE_IMAGE_REPOSITORY, DATABASE_URL, POSTGRES_*,
    EMQX_DASHBOARD_*, EMQX_API_*, MQTT_CLIENT_ID_API, CADDY_ACME_EMAIL, ...

  Optional registry pull (VPS):
    GHCR_PULL_USERNAME, GHCR_PULL_TOKEN — if both set, runs docker login ghcr.io before pull.

  Optional after deploy:
    SKIP_SMOKE=1 — skip scripts/healthcheck_prod.sh

  Optional rollout wait (deploy / rollback; polls container state / Docker health):
    ROLLUP_HEALTH_WAIT_SECS (default 180; clamped 30–3600 so 0 cannot instant-fail),
    ROLLUP_HEALTH_POLL_SECS (default 5; minimum 1)
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

try_read_env_file() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" "${ENV_FILE}" 2>/dev/null | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		printf ''
		return 0
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

# Resolve APP_IMAGE_TAG or GOOSE_IMAGE_TAG; falls back to legacy IMAGE_TAG when primary is unset.
resolve_image_tag_from_env() {
	local primary_key="$1"
	local v
	v="$(try_read_env_file "${primary_key}")"
	if [[ -n "${v}" ]]; then
		printf '%s' "${v}"
		return 0
	fi
	v="$(try_read_env_file IMAGE_TAG)"
	if [[ -n "${v}" ]]; then
		printf '%s' "${v}"
		return 0
	fi
	return 1
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

registry_login_optional() {
	if [[ -n "${GHCR_PULL_USERNAME:-}" ]] && [[ -n "${GHCR_PULL_TOKEN:-}" ]]; then
		note "docker login ghcr.io (GHCR_PULL_USERNAME / GHCR_PULL_TOKEN)"
		printf '%s' "${GHCR_PULL_TOKEN}" | docker login ghcr.io -u "${GHCR_PULL_USERNAME}" --password-stdin
	elif [[ -n "${GHCR_PULL_USERNAME:-}" ]] || [[ -n "${GHCR_PULL_TOKEN:-}" ]]; then
		fail "set both GHCR_PULL_USERNAME and GHCR_PULL_TOKEN, or neither, for registry login"
	fi
}

resolve_emqx_api_credentials() {
	local file_key file_secret

	if [[ -n "${EMQX_API_KEY:-}" ]] || [[ -n "${EMQX_API_SECRET:-}" ]]; then
		if [[ -z "${EMQX_API_KEY:-}" ]] || [[ -z "${EMQX_API_SECRET:-}" ]]; then
			fail "EMQX_API_KEY and EMQX_API_SECRET must both be set together in environment or ${ENV_FILE}"
		fi
		EMQX_API_KEY_RESOLVED="${EMQX_API_KEY}"
		EMQX_API_SECRET_RESOLVED="${EMQX_API_SECRET}"
	else
		file_key="$(try_read_env_file EMQX_API_KEY)"
		file_secret="$(try_read_env_file EMQX_API_SECRET)"
		if [[ -z "${file_key}" ]] || [[ -z "${file_secret}" ]]; then
			fail "missing required EMQX_API_KEY / EMQX_API_SECRET (set in environment or ${ENV_FILE})"
		fi
		EMQX_API_KEY_RESOLVED="${file_key}"
		EMQX_API_SECRET_RESOLVED="${file_secret}"
	fi
}

validate_release_assets_or_fail() {
	local validator="${ROOT}/scripts/validate_release_assets.sh"
	[[ -f "${validator}" ]] || fail "missing ${validator}"
	if ! bash "${validator}" "${ENV_FILE}"; then
		fail "validate_release_assets.sh failed"
	fi
}

render_emqx_api_key_file() {
	local dir tmp
	resolve_emqx_api_credentials

	dir="$(dirname "${DEFAULT_API_KEY_FILE}")"
	mkdir -p "${dir}"
	tmp="$(mktemp "${dir}/default_api_key.conf.tmp.XXXXXX")"
	if ! printf '%s:%s:administrator\n' "${EMQX_API_KEY_RESOLVED}" "${EMQX_API_SECRET_RESOLVED}" >"${tmp}"; then
		rm -f "${tmp}"
		fail "failed to write temporary EMQX API bootstrap file"
	fi
	chmod 600 "${tmp}"
	mv -f "${tmp}" "${DEFAULT_API_KEY_FILE}"
	[[ -s "${DEFAULT_API_KEY_FILE}" ]] || fail "rendered ${DEFAULT_API_KEY_FILE} is missing or empty"
	note "rendered EMQX API bootstrap file at ${DEFAULT_API_KEY_FILE}"
}

wait_for_emqx_control_plane() {
	local attempts="${1:-90}"
	local sleep_secs="${2:-2}"
	local i
	note "waiting for EMQX control plane readiness"
	for i in $(seq 1 "${attempts}"); do
		if "${COMPOSE[@]}" exec -T emqx emqx_ctl status >/dev/null 2>&1; then
			note "EMQX control plane is responding"
			return 0
		fi
		sleep "${sleep_secs}"
	done
	fail "EMQX control plane did not become ready — inspect avf-prod-emqx logs"
}

preflight_emqx_api_auth() {
	local timeout_secs="${EMQX_API_PREFLIGHT_WAIT_SECS:-90}"
	local poll_secs="${EMQX_API_PREFLIGHT_POLL_SECS:-3}"
	local start_ts now_ts elapsed code tmp saw_401="0"

	[[ "${timeout_secs}" =~ ^[0-9]+$ ]] || timeout_secs=90
	[[ "${poll_secs}" =~ ^[0-9]+$ ]] || poll_secs=3
	[[ "${poll_secs}" -ge 1 ]] || poll_secs=1
	tmp="$(mktemp)"
	start_ts="$(date +%s)"

	note "preflight EMQX management API auth via /api/v5/status"
	while true; do
		code="$(
			curl -sS -o "${tmp}" -w "%{http_code}" \
				-u "${EMQX_API_KEY_RESOLVED}:${EMQX_API_SECRET_RESOLVED}" \
				"http://127.0.0.1:18083/api/v5/status" || true
		)"
		if [[ "${code}" == "200" ]]; then
			rm -f "${tmp}"
			note "EMQX API auth preflight passed"
			return 0
		fi

		now_ts="$(date +%s)"
		elapsed=$((now_ts - start_ts))
		if [[ "${code}" == "401" && "${saw_401}" == "0" ]]; then
			saw_401="1"
			echo "release.sh: EMQX API preflight got HTTP 401 — verify EMQX_API_KEY / EMQX_API_SECRET, ${ROOT}/emqx/default_api_key.conf on the VPS, /opt/emqx/etc/default_api_key.conf inside avf-prod-emqx, and that EMQX was force-recreated after config changes" >&2
		else
			note "EMQX API preflight pending (HTTP ${code:-empty}, ${elapsed}s/${timeout_secs}s)"
		fi

		if [[ "${elapsed}" -ge "${timeout_secs}" ]]; then
			if [[ -s "${tmp}" ]]; then
				cat "${tmp}" >&2
			fi
			rm -f "${tmp}"
			fail "EMQX API preflight failed after ${timeout_secs}s — verify EMQX_API_KEY / EMQX_API_SECRET, ${ROOT}/emqx/default_api_key.conf on the VPS, /opt/emqx/etc/default_api_key.conf inside avf-prod-emqx, and that EMQX was force-recreated after config changes"
		fi

		sleep "${poll_secs}"
	done
}

persist_rollout_image_state() {
	local new_app="$1" new_goose="$2" old_app="${3:-}" old_goose="${4:-}" log_label="$5"
	mkdir -p "${STATE}"
	if [[ -n "${old_app}" ]]; then
		printf '%s\n' "${old_app}" >"${STATE}/previous_app_image_tag"
		printf '%s\n' "${old_app}" >"${STATE}/previous_image_tag"
	fi
	if [[ -n "${old_goose}" ]]; then
		printf '%s\n' "${old_goose}" >"${STATE}/previous_goose_image_tag"
	fi
	printf '%s\n' "${new_app}" >"${STATE}/current_app_image_tag"
	printf '%s\n' "${new_goose}" >"${STATE}/current_goose_image_tag"
	printf '%s\n' "${new_app}" >"${STATE}/current_image_tag"
	printf '%s\t%s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "${log_label} app=${new_app} goose=${new_goose}" >>"${STATE}/history.log"
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

rollout_container_ok() {
	local c="$1" state has_h h
	state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
	[[ "${state}" == "running" ]] || return 1
	has_h="$(docker inspect -f '{{if .State.Health}}yes{{else}}no{{end}}' "${c}" 2>/dev/null || echo no)"
	if [[ "${has_h}" == "yes" ]]; then
		h="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "${c}" 2>/dev/null || true)"
		[[ "${h}" == "healthy" ]] || return 1
	fi
	return 0
}

rollout_container_fail_reason() {
	local c="$1" state has_h h
	state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
	if [[ "${state}" == "missing" ]]; then
		printf '%s' "container_not_found"
		return
	fi
	if [[ "${state}" != "running" ]]; then
		printf '%s' "state=${state}"
		return
	fi
	has_h="$(docker inspect -f '{{if .State.Health}}yes{{else}}no{{end}}' "${c}" 2>/dev/null || echo no)"
	if [[ "${has_h}" != "yes" ]]; then
		printf '%s' "running_no_docker_health"
		return
	fi
	h="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "${c}" 2>/dev/null || true)"
	if [[ "${h}" == "healthy" ]]; then
		printf '%s' "ok"
		return
	fi
	if [[ -z "${h}" ]]; then
		printf '%s' "health=starting_or_empty"
		return
	fi
	printf '%s' "health=${h}"
}

print_rollout_gate_status() {
	local i c svc state has_h h
	for i in "${!ROLLUP_GATE_CTRS[@]}"; do
		c="${ROLLUP_GATE_CTRS[$i]}"
		svc="${ROLLUP_GATE_SVCS[$i]}"
		state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
		has_h="$(docker inspect -f '{{if .State.Health}}yes{{else}}no{{end}}' "${c}" 2>/dev/null || echo no)"
		if [[ "${has_h}" == "yes" ]]; then
			h="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "${c}" 2>/dev/null || echo "?")"
		else
			h="n/a"
		fi
		printf '  %-16s %-24s status=%s health=%s\n' "${svc}" "${c}" "${state}" "${h}"
	done
}

print_rollout_timeout_diagnostics() {
	note "rollout health gate exceeded — docker compose ps"
	"${COMPOSE[@]}" ps 2>&1 || true
	local i c svc
	for i in "${!ROLLUP_GATE_CTRS[@]}"; do
		c="${ROLLUP_GATE_CTRS[$i]}"
		svc="${ROLLUP_GATE_SVCS[$i]}"
		if rollout_container_ok "${c}"; then
			continue
		fi
		echo "" >&2
		echo "==> failing: ${svc} (${c})" >&2
		docker inspect -f 'status={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}} failing_streak={{.State.Health.FailingStreak}}{{else}}n/a{{end}} exit={{.State.ExitCode}}' "${c}" 2>/dev/null >&2 || echo "  docker inspect failed for ${c}" >&2
		if command -v jq >/dev/null 2>&1; then
			docker inspect "${c}" 2>/dev/null | jq -r '.[0].State.Health.Log // [] | .[-3:][] | "  health_log: \(.Output)"' 2>/dev/null >&2 || true
		fi
		echo "  docker logs --tail=100 ${c}:" >&2
		docker logs --tail 100 "${c}" 2>&1 | sed 's/^/    /' >&2 || true
	done
}

wait_for_rollout_health() {
	local wait_secs poll start_ts now_ts elapsed bad_list c
	wait_secs="${ROLLUP_HEALTH_WAIT_SECS:-180}"
	poll="${ROLLUP_HEALTH_POLL_SECS:-5}"
	[[ "${wait_secs}" =~ ^[0-9]+$ ]] || wait_secs=180
	[[ "${poll}" =~ ^[0-9]+$ ]] || poll=5
	[[ "${poll}" -ge 1 ]] || poll=1
	if [[ "${wait_secs}" -lt 30 ]]; then
		wait_secs=30
	fi
	if [[ "${wait_secs}" -gt 3600 ]]; then
		wait_secs=3600
	fi
	start_ts="$(date +%s)"
	note "waiting for rollout gate (postgres, nats, emqx, api, worker, mqtt-ingest, reconciler, caddy) — up to ${wait_secs}s, poll every ${poll}s"
	while true; do
		bad_list=()
		for c in "${ROLLUP_GATE_CTRS[@]}"; do
			if ! rollout_container_ok "${c}"; then
				bad_list+=("${c}")
			fi
		done
		if [[ "${#bad_list[@]}" -eq 0 ]]; then
			note "rollout health gate: all monitored containers ready (running; healthy where Docker defines a healthcheck)"
			return 0
		fi
		now_ts="$(date +%s)"
		elapsed=$((now_ts - start_ts))
		if [[ "${elapsed}" -ge "${wait_secs}" ]]; then
			print_rollout_timeout_diagnostics
			fail "rollout health gate: timed out after ${wait_secs}s — not ready: ${bad_list[*]}"
		fi
		note "rollout health gate (${elapsed}s / ${wait_secs}s) — waiting on: ${bad_list[*]}"
		for c in "${bad_list[@]}"; do
			printf '    %s: %s\n' "${c}" "$(rollout_container_fail_reason "${c}")" >&2
		done
		print_rollout_gate_status
		sleep "${poll}"
	done
}

cmd_deploy() {
	local arg1="${1:-}" arg2="${2:-}"
	local old_app old_goose new_app new_goose reg repo_app repo_goose

	[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"

	old_app="$(try_read_env_file APP_IMAGE_TAG)"
	old_goose="$(try_read_env_file GOOSE_IMAGE_TAG)"

	if [[ -n "${arg1}" ]]; then
		new_app="${arg1}"
		new_goose="${arg2:-${arg1}}"
	else
		if ! new_app="$(resolve_image_tag_from_env APP_IMAGE_TAG)" || [[ -z "${new_app}" ]]; then
			fail "deploy needs [app_tag [goose_tag]] or APP_IMAGE_TAG / IMAGE_TAG in ${ENV_FILE}"
		fi
		new_goose="$(try_read_env_file GOOSE_IMAGE_TAG)"
		if [[ -z "${new_goose}" ]]; then
			new_goose="$(try_read_env_file IMAGE_TAG)"
		fi
		if [[ -z "${new_goose}" ]]; then
			new_goose="${new_app}"
		fi
	fi

	note "validate required image / stack env"
	require_env_resolved IMAGE_REGISTRY >/dev/null
	require_env_resolved APP_IMAGE_REPOSITORY >/dev/null
	require_env_resolved GOOSE_IMAGE_REPOSITORY >/dev/null
	require_env_resolved DATABASE_URL >/dev/null
	resolve_emqx_api_credentials
	export EMQX_API_KEY="${EMQX_API_KEY_RESOLVED}"
	export EMQX_API_SECRET="${EMQX_API_SECRET_RESOLVED}"

	note "write image tags and registry metadata to ${ENV_FILE} (APP_IMAGE_TAG=${new_app} GOOSE_IMAGE_TAG=${new_goose})"
	# Prefer exported CI/VPS env over existing file values when set.
	reg="$(require_env_resolved IMAGE_REGISTRY)"
	repo_app="$(require_env_resolved APP_IMAGE_REPOSITORY)"
	repo_goose="$(require_env_resolved GOOSE_IMAGE_REPOSITORY)"
	set_env_value "IMAGE_REGISTRY" "${reg}" "${ENV_FILE}"
	set_env_value "APP_IMAGE_REPOSITORY" "${repo_app}" "${ENV_FILE}"
	set_env_value "GOOSE_IMAGE_REPOSITORY" "${repo_goose}" "${ENV_FILE}"
	set_env_value "APP_IMAGE_TAG" "${new_app}" "${ENV_FILE}"
	set_env_value "GOOSE_IMAGE_TAG" "${new_goose}" "${ENV_FILE}"
	set_env_value "IMAGE_TAG" "${new_app}" "${ENV_FILE}"

	registry_login_optional
	validate_release_assets_or_fail

	note "docker compose config"
	if ! "${COMPOSE[@]}" config >/dev/null; then
		fail "docker compose config failed — fix ${COMPOSE_FILE} or ${ENV_FILE}"
	fi

	note "docker compose pull (app + goose images)"
	if ! "${COMPOSE[@]}" pull "${ARTIFACT_SERVICES[@]}"; then
		fail "docker compose pull failed — check registry auth and tags"
	fi

	note "start data plane (postgres, nats) before migrations"
	if ! "${COMPOSE[@]}" up -d postgres nats; then
		fail "failed to start postgres/nats"
	fi

	note "docker compose up migrate (foreground; one-shot migration)"
	# Ensures migrations run before application containers are switched.
	if ! "${COMPOSE[@]}" up migrate; then
		fail "migrate failed — fix database before retrying"
	fi

	render_emqx_api_key_file
	note "force-recreate EMQX so updated bootstrap assets are reloaded"
	if ! "${COMPOSE[@]}" up -d --force-recreate emqx; then
		fail "failed to force-recreate emqx"
	fi
	wait_for_emqx_control_plane
	preflight_emqx_api_auth

	note "EMQX MQTT user bootstrap"
	if ! bash "${ROOT}/scripts/emqx_bootstrap.sh"; then
		fail "emqx_bootstrap.sh failed"
	fi

	note "docker compose up -d (${LONG_LIVED_SERVICES[*]})"
	if ! "${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"; then
		fail "compose up failed"
	fi

	wait_for_rollout_health
	verify_long_lived_running

	note "post-deploy health checks"
	if [[ "${SKIP_SMOKE:-0}" != "1" ]]; then
		if ! bash "${ROOT}/scripts/healthcheck_prod.sh"; then
			fail "healthcheck_prod.sh failed"
		fi
	fi

	persist_rollout_image_state "${new_app}" "${new_goose}" "${old_app}" "${old_goose}" "deploy"
	note "deploy complete (APP_IMAGE_TAG=${new_app} GOOSE_IMAGE_TAG=${new_goose})"
}

cmd_rollback() {
	local arg1="${1:-}" arg2="${2:-}"
	local old_app old_goose rb_app rb_goose

	[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"

	old_app="$(try_read_env_file APP_IMAGE_TAG)"
	old_goose="$(try_read_env_file GOOSE_IMAGE_TAG)"

	if [[ -n "${arg1}" ]]; then
		rb_app="${arg1}"
		rb_goose="${arg2:-${arg1}}"
	else
		mkdir -p "${STATE}"
		local prev_app="${STATE}/previous_app_image_tag"
		local prev_legacy="${STATE}/previous_image_tag"
		if [[ -f "${prev_app}" ]]; then
			rb_app="$(tr -d '\r\n' <"${prev_app}")"
		elif [[ -f "${prev_legacy}" ]]; then
			rb_app="$(tr -d '\r\n' <"${prev_legacy}")"
		else
			fail "no previous_app_image_tag or previous_image_tag in ${STATE}; pass: rollback <app_tag> [goose_tag]"
		fi
		[[ -n "${rb_app}" ]] || fail "previous app tag is empty"
		local prev_goose="${STATE}/previous_goose_image_tag"
		if [[ -f "${prev_goose}" ]]; then
			rb_goose="$(tr -d '\r\n' <"${prev_goose}")"
		else
			rb_goose="${rb_app}"
		fi
	fi

	note "rollback to APP_IMAGE_TAG=${rb_app} GOOSE_IMAGE_TAG=${rb_goose}"
	require_env_resolved IMAGE_REGISTRY >/dev/null
	require_env_resolved APP_IMAGE_REPOSITORY >/dev/null
	require_env_resolved GOOSE_IMAGE_REPOSITORY >/dev/null

	set_env_value "APP_IMAGE_TAG" "${rb_app}" "${ENV_FILE}"
	set_env_value "GOOSE_IMAGE_TAG" "${rb_goose}" "${ENV_FILE}"
	set_env_value "IMAGE_TAG" "${rb_app}" "${ENV_FILE}"

	registry_login_optional

	note "docker compose config"
	"${COMPOSE[@]}" config >/dev/null

	note "docker compose pull"
	"${COMPOSE[@]}" pull "${ARTIFACT_SERVICES[@]}"

	note "docker compose up -d (${LONG_LIVED_SERVICES[*]})"
	if ! "${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"; then
		fail "compose up failed"
	fi

	wait_for_rollout_health
	verify_long_lived_running

	if [[ "${SKIP_SMOKE:-0}" != "1" ]]; then
		note "post-rollback health checks"
		if ! bash "${ROOT}/scripts/healthcheck_prod.sh"; then
			fail "healthcheck_prod.sh failed"
		fi
	fi

	persist_rollout_image_state "${rb_app}" "${rb_goose}" "${old_app}" "${old_goose}" "rollback"
	note "rollback complete (APP_IMAGE_TAG=${rb_app} GOOSE_IMAGE_TAG=${rb_goose})"
}

cmd_status() {
	note "release state (${STATE})"
	if [[ -d "${STATE}" ]]; then
		for f in current_image_tag previous_image_tag current_app_image_tag current_goose_image_tag previous_app_image_tag previous_goose_image_tag; do
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
			cmd_deploy "${1:-}" "${2:-}"
			;;
		rollback)
			cmd_rollback "${1:-}" "${2:-}"
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
