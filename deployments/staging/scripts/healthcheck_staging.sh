#!/usr/bin/env bash
# Post-deploy checks from the staging VPS host.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

note() {
	echo "==> $*"
}

fail_fast() {
	echo "error: $*" >&2
	exit 1
}

if [[ ! -f .env.staging ]]; then
	fail_fast "missing .env.staging"
fi

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" .env.staging | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		echo "error: ${key} not set in .env.staging" >&2
		exit 1
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

API_DOMAIN="$(read_env API_DOMAIN)"
COMPOSE=(docker compose --env-file .env.staging -f docker-compose.staging.yml)
failures=0
HEALTH_WAIT_SECS="${STAGING_HEALTH_WAIT_SECS:-120}"
HEALTH_POLL_SECS="${STAGING_HEALTH_POLL_SECS:-5}"

pass() {
	echo "PASS: $1"
}

fail() {
	echo "FAIL: $1" >&2
	failures=$((failures + 1))
}

show_container_diagnostics() {
	local container="$1"
	echo "  container inspect: ${container}" >&2
	docker inspect -f '    status={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}n/a{{end}} started={{.State.StartedAt}} finished={{.State.FinishedAt}} exit={{.State.ExitCode}}' "${container}" 2>/dev/null >&2 || true
	echo "  recent logs: ${container}" >&2
	docker logs --tail 20 "${container}" 2>&1 | sed 's/^/    /' >&2 || true
}

check_cmd() {
	local label="$1"
	local diagnostics="${2:-}"
	shift
	shift
	local output
	if output="$("$@" 2>&1)"; then
		pass "${label}"
	else
		fail "${label}"
		if [[ -n "${output}" ]]; then
			echo "${output}" | sed 's/^/  output: /' >&2
		fi
		if [[ -n "${diagnostics}" ]]; then
			echo "  hint: ${diagnostics}" >&2
		fi
	fi
}

wait_for_health() {
	local container="$1"
	local waited=0
	local status
	while (( waited <= HEALTH_WAIT_SECS )); do
		status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${container}" 2>/dev/null || echo missing)"
		if [[ "${status}" == "healthy" || "${status}" == "none" ]]; then
			return 0
		fi
		if [[ "${status}" == "unhealthy" || "${status}" == "missing" ]]; then
			return 1
		fi
		sleep "${HEALTH_POLL_SECS}"
		waited=$((waited + HEALTH_POLL_SECS))
	done
	return 1
}

note "compose ps"
"${COMPOSE[@]}" ps

need_running=(avf-staging-caddy avf-staging-api avf-staging-worker avf-staging-mqtt-ingest avf-staging-reconciler avf-staging-postgres avf-staging-nats avf-staging-emqx)
need_healthy=(avf-staging-api avf-staging-worker avf-staging-mqtt-ingest avf-staging-reconciler avf-staging-postgres avf-staging-nats avf-staging-emqx)
for c in "${need_running[@]}"; do
	state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
	if [[ "${state}" != "running" ]]; then
		fail "container ${c} running (state=${state})"
		show_container_diagnostics "${c}"
		continue
	fi
	pass "container ${c} running"
done

note "container health status"
for c in "${need_healthy[@]}"; do
	if wait_for_health "${c}"; then
		pass "container ${c} healthy"
	else
		health_state="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}n/a{{end}}' "${c}" 2>/dev/null || echo missing)"
		fail "container ${c} healthy (health=${health_state})"
		show_container_diagnostics "${c}"
	fi
done

note "internal service checks"
check_cmd "postgres pg_isready" "check staging postgres logs and DATABASE_URL / POSTGRES_* values" "${COMPOSE[@]}" exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d avf_vending >/dev/null'
check_cmd "nats health endpoint" "check avf-staging-nats logs and JetStream startup" "${COMPOSE[@]}" exec -T nats sh -c 'wget -qO- http://127.0.0.1:8222/healthz >/dev/null'
# `emqx_ctl` can fail while the HTTP management API is already up (parity with production healthcheck_prod).
check_cmd "emqx management HTTP /api/v5/status" "check avf-staging-emqx logs; confirm listener on 18083 and /api/v5/status" "${COMPOSE[@]}" exec -T emqx bash -lc 'exec 3<>/dev/tcp/127.0.0.1/18083; printf %b "GET /api/v5/status HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n" >&3; grep -Fq "emqx is running" <&3'
check_cmd "api live endpoint inside container" "check avf-staging-api logs and upstream dependency readiness" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/live | grep -qx ok'
check_cmd "api readiness inside container" "check avf-staging-api logs plus postgres/nats/emqx readiness" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/ready | grep -qx ok'

note "reverse proxy / API health (public HTTPS)"
check_cmd "public /health/live over HTTPS" "check staging DNS, firewall on 80/443, Caddy logs, and TLS issuance for ${API_DOMAIN}" bash -lc "curl -fsS 'https://${API_DOMAIN}/health/live' | grep -qx ok"
check_cmd "public /health/ready over HTTPS" "check staging DNS, firewall on 80/443, Caddy logs, and backend readiness for ${API_DOMAIN}" bash -lc "curl -fsS 'https://${API_DOMAIN}/health/ready' | grep -qx ok"

if (( failures > 0 )); then
	echo "==> failing container summary" >&2
	for c in "${need_running[@]}"; do
		state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
		if [[ "${state}" != "running" ]]; then
			show_container_diagnostics "${c}"
		fi
	done
	echo "healthcheck_staging: FAIL (${failures} checks failed)" >&2
	exit 1
fi

echo "healthcheck_staging: PASS"
