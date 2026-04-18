#!/usr/bin/env bash
# Post-deploy checks from the VPS host (Caddy + API + core containers).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ ! -f .env.production ]]; then
	echo "error: missing .env.production" >&2
	exit 1
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

check_cmd() {
	local label="$1"
	shift
	if "$@"; then
		pass "${label}"
	else
		fail "${label}"
	fi
}

echo "==> compose ps"
"${COMPOSE[@]}" ps

need_running=(avf-prod-caddy avf-prod-api avf-prod-worker avf-prod-mqtt-ingest avf-prod-reconciler avf-prod-postgres avf-prod-nats avf-prod-emqx)
for c in "${need_running[@]}"; do
	state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
	if [[ "${state}" != "running" ]]; then
		fail "container ${c} running (state=${state})"
		continue
	fi
	pass "container ${c} running"
done

echo "==> internal service checks"
check_cmd "postgres pg_isready" "${COMPOSE[@]}" exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d avf_vending >/dev/null'
check_cmd "nats health endpoint" "${COMPOSE[@]}" exec -T nats sh -c 'wget -qO- http://127.0.0.1:8222/healthz >/dev/null'
check_cmd "emqx broker status" "${COMPOSE[@]}" exec -T emqx sh -c 'emqx_ctl status >/dev/null'
check_cmd "api live endpoint inside container" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/live | grep -qx ok'
check_cmd "api readiness inside container" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/ready | grep -qx ok'

echo "==> reverse proxy / API health (public HTTPS)"
check_cmd "public /health/live over HTTPS" bash -lc "curl -fsS 'https://${API_DOMAIN}/health/live' | grep -qx ok"
check_cmd "public /health/ready over HTTPS" bash -lc "curl -fsS 'https://${API_DOMAIN}/health/ready' | grep -qx ok"

if (( failures > 0 )); then
	echo "healthcheck_prod: FAIL (${failures} checks failed)" >&2
	exit 1
fi

echo "healthcheck_prod: PASS"
