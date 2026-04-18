#!/usr/bin/env bash
# Post-deploy checks from the VPS host (Caddy + API + core containers).
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

if [[ ! -f .env.production ]]; then
	fail_fast "missing .env.production"
fi

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" .env.production | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		echo "error: ${key} not set in .env.production" >&2
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
COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)
failures=0

pass() {
	echo "PASS: $1"
}

fail() {
	echo "FAIL: $1" >&2
	failures=$((failures + 1))
}

check_compose_health() {
	local name="$1"
	local has_health status
	has_health="$(docker inspect -f '{{if .State.Health}}yes{{else}}no{{end}}' "${name}" 2>/dev/null || echo no)"
	if [[ "${has_health}" != "yes" ]]; then
		pass "${name} has no docker healthcheck (skipped)"
		return
	fi
	status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "${name}" 2>/dev/null || true)"
	if [[ "${status}" != "healthy" ]]; then
		fail "${name} docker health=${status} (expected healthy)"
		show_container_diagnostics "${name}"
		return
	fi
	pass "${name} docker health=healthy"
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

note "compose ps"
"${COMPOSE[@]}" ps

need_running=(avf-prod-caddy avf-prod-api avf-prod-worker avf-prod-mqtt-ingest avf-prod-reconciler avf-prod-postgres avf-prod-nats avf-prod-emqx)
for c in "${need_running[@]}"; do
	state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
	if [[ "${state}" != "running" ]]; then
		fail "container ${c} running (state=${state})"
		show_container_diagnostics "${c}"
		continue
	fi
	pass "container ${c} running"
done

note "docker healthchecks (where defined)"
for c in avf-prod-postgres avf-prod-nats avf-prod-emqx avf-prod-api avf-prod-worker avf-prod-mqtt-ingest avf-prod-reconciler avf-prod-caddy; do
	check_compose_health "${c}"
done

note "internal service checks"
check_cmd "postgres pg_isready" "check postgres container logs and DATABASE_URL / POSTGRES_* values" "${COMPOSE[@]}" exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d avf_vending >/dev/null'
check_cmd "nats health endpoint" "check avf-prod-nats logs and JetStream startup" "${COMPOSE[@]}" exec -T nats sh -c 'wget -qO- http://127.0.0.1:8222/healthz >/dev/null'
check_cmd "emqx broker status" "check avf-prod-emqx logs and dashboard/bootstrap credentials" "${COMPOSE[@]}" exec -T emqx sh -c 'emqx_ctl status >/dev/null'
check_cmd "api live endpoint inside container" "check avf-prod-api logs and upstream dependency readiness" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/live | grep -qx ok'
check_cmd "api readiness inside container" "check avf-prod-api logs plus postgres/nats/emqx readiness" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/ready | grep -qx ok'

note "reverse proxy / API health (public HTTPS)"
if [[ "${SKIP_PUBLIC_HTTPS:-0}" == "1" ]]; then
	echo "SKIP_PUBLIC_HTTPS=1 set; skipping public HTTPS checks (use when DNS/TLS is still propagating)"
else
	check_cmd "public /health/live over HTTPS" "if internal checks above passed, this failure is usually DNS, upstream firewall on 80/443, or ACME/TLS issuance — inspect Caddy logs: docker logs avf-prod-caddy" bash -lc "curl -fsS 'https://${API_DOMAIN}/health/live' | grep -qx ok"
	check_cmd "public /health/ready over HTTPS" "if internal checks above passed, this failure is usually DNS, upstream firewall on 80/443, or ACME/TLS issuance — inspect Caddy logs: docker logs avf-prod-caddy" bash -lc "curl -fsS 'https://${API_DOMAIN}/health/ready' | grep -qx ok"
fi

if (( failures > 0 )); then
	echo "==> failing container summary" >&2
	for c in "${need_running[@]}"; do
		state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
		if [[ "${state}" != "running" ]]; then
			show_container_diagnostics "${c}"
		fi
	done
	echo "healthcheck_prod: FAIL (${failures} checks failed)" >&2
	exit 1
fi

echo "healthcheck_prod: PASS"
