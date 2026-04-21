#!/usr/bin/env bash
# Post-deploy health and status checks from the VPS host (compose + HTTP/MQTT signals).
# Does not build images, compile source, run migrations, or invoke release.sh — read-only / curl / docker inspect only.
set -Eeuo pipefail

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

read_env_default() {
	local key="$1"
	local default="$2"
	local line
	line="$(grep -E "^${key}=" .env.production | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		printf '%s' "${default}"
		return
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
resolve_database_name() {
	local db_name db_url
	db_name="$(read_env_default POSTGRES_DB "")"
	if [[ -n "${db_name}" ]]; then
		printf '%s' "${db_name}"
		return 0
	fi

	db_url="$(read_env_default DATABASE_URL "")"
	[[ -n "${db_url}" ]] || fail_fast "POSTGRES_DB or DATABASE_URL must be set in .env.production"

	db_name="${db_url##*/}"
	db_name="${db_name%%\?*}"
	[[ -n "${db_name}" ]] || fail_fast "could not derive database name from DATABASE_URL"
	printf '%s' "${db_name}"
}

DB_NAME="$(resolve_database_name)"
COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)
failures=0
SMOKE_RETRY_COUNT="${SMOKE_RETRY_COUNT:-6}"
SMOKE_RETRY_SLEEP_SECS="${SMOKE_RETRY_SLEEP_SECS:-5}"

# Same eight containers as release.sh rollout gate (compose service name, container_name).
STACK_GATE_SVCS=(postgres nats emqx api worker mqtt-ingest reconciler caddy)
STACK_GATE_CTRS=(avf-prod-postgres avf-prod-nats avf-prod-emqx avf-prod-api avf-prod-worker avf-prod-mqtt-ingest avf-prod-reconciler avf-prod-caddy)

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

stack_container_ready() {
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

stack_container_fail_reason() {
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

print_stack_gate_status() {
	local i c svc state has_h h
	for i in "${!STACK_GATE_CTRS[@]}"; do
		c="${STACK_GATE_CTRS[$i]}"
		svc="${STACK_GATE_SVCS[$i]}"
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

print_stack_docker_health_timeout_diagnostics() {
	note "stack docker health gate exceeded — docker compose ps"
	"${COMPOSE[@]}" ps 2>&1 || true
	local i c svc
	for i in "${!STACK_GATE_CTRS[@]}"; do
		c="${STACK_GATE_CTRS[$i]}"
		svc="${STACK_GATE_SVCS[$i]}"
		if stack_container_ready "${c}"; then
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

wait_stack_docker_health() {
	local wait_secs poll start_ts now_ts elapsed bad_list c
	wait_secs="${STACK_DOCKER_HEALTH_WAIT_SECS:-180}"
	poll="${STACK_DOCKER_HEALTH_POLL_SECS:-5}"
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
	note "poll stack docker health (postgres, nats, emqx, api, worker, mqtt-ingest, reconciler, caddy) — up to ${wait_secs}s, poll every ${poll}s"
	while true; do
		bad_list=()
		for c in "${STACK_GATE_CTRS[@]}"; do
			if ! stack_container_ready "${c}"; then
				bad_list+=("${c}")
			fi
		done
		if [[ "${#bad_list[@]}" -eq 0 ]]; then
			note "stack docker health gate: all monitored containers ready"
			return 0
		fi
		now_ts="$(date +%s)"
		elapsed=$((now_ts - start_ts))
		if [[ "${elapsed}" -ge "${wait_secs}" ]]; then
			print_stack_docker_health_timeout_diagnostics
			return 1
		fi
		note "stack docker health (${elapsed}s / ${wait_secs}s) — waiting on: ${bad_list[*]}"
		for c in "${bad_list[@]}"; do
			printf '    %s: %s\n' "${c}" "$(stack_container_fail_reason "${c}")" >&2
		done
		print_stack_gate_status
		sleep "${poll}"
	done
}

show_container_diagnostics() {
	local container="$1"
	echo "  container inspect: ${container}" >&2
	docker inspect -f '    status={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}n/a{{end}} started={{.State.StartedAt}} finished={{.State.FinishedAt}} exit={{.State.ExitCode}}' "${container}" 2>/dev/null >&2 || true
	echo "  recent logs: ${container}" >&2
	docker logs --tail 20 "${container}" 2>&1 | sed 's/^/    /' >&2 || true
}

# Used only on final healthcheck failure: longer logs + health probe history when available.
show_container_diagnostics_deep() {
	local container="$1"
	echo "  === deep diagnostics: ${container} ===" >&2
	docker inspect -f '    status={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}} failing_streak={{.State.Health.FailingStreak}}{{else}}n/a{{end}} started={{.State.StartedAt}} exit={{.State.ExitCode}}' "${container}" 2>/dev/null >&2 || true
	if command -v jq >/dev/null 2>&1; then
		docker inspect "${container}" 2>/dev/null | jq -r '.[0].State.Health.Log // [] | .[-3:][] | "    health_log: \(.Output)"' 2>/dev/null >&2 || true
	fi
	echo "    logs (tail 100): ${container}" >&2
	docker logs --tail 100 "${container}" 2>&1 | sed 's/^/    /' >&2 || true
}

container_warrants_deep_diag() {
	local c="$1" state has_h h
	state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
	if [[ "${state}" != "running" ]]; then
		return 0
	fi
	has_h="$(docker inspect -f '{{if .State.Health}}yes{{else}}no{{end}}' "${c}" 2>/dev/null || echo no)"
	if [[ "${has_h}" != "yes" ]]; then
		return 1
	fi
	h="$(docker inspect -f '{{.State.Health.Status}}' "${c}" 2>/dev/null || true)"
	if [[ "${h}" != "healthy" ]]; then
		return 0
	fi
	return 1
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

retry_check_cmd() {
	local label="$1"
	local diagnostics="$2"
	shift 2
	local attempt output
	for attempt in $(seq 1 "${SMOKE_RETRY_COUNT}"); do
		if output="$("$@" 2>&1)"; then
			pass "${label}"
			return 0
		fi
		if [[ "${attempt}" -lt "${SMOKE_RETRY_COUNT}" ]]; then
			sleep "${SMOKE_RETRY_SLEEP_SECS}"
		fi
	done
	fail "${label}"
	if [[ -n "${output:-}" ]]; then
		echo "${output}" | sed 's/^/  output: /' >&2
	fi
	if [[ -n "${diagnostics}" ]]; then
		echo "  hint: ${diagnostics}" >&2
	fi
}

note "compose ps"
"${COMPOSE[@]}" ps

need_running=(avf-prod-caddy avf-prod-api avf-prod-worker avf-prod-mqtt-ingest avf-prod-reconciler avf-prod-postgres avf-prod-nats avf-prod-emqx)
for c in "${need_running[@]}"; do
	state="$(docker inspect -f '{{.State.Status}}' "${c}" 2>/dev/null || echo missing)"
	if [[ "${state}" != "running" ]]; then
		fail "container ${c} not running (state=${state})"
		show_container_diagnostics "${c}"
		continue
	fi
	pass "container ${c} running"
done

note "docker health gate (poll)"
if wait_stack_docker_health; then
	note "docker healthchecks (verify)"
	for c in "${STACK_GATE_CTRS[@]}"; do
		check_compose_health "${c}"
	done
else
	fail "stack docker health gate timed out (see diagnostics above)"
fi

note "internal service checks"
check_cmd "postgres pg_isready" "check postgres container logs and DATABASE_URL / POSTGRES_* values" "${COMPOSE[@]}" exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d "$1" >/dev/null' sh "${DB_NAME}"
check_cmd "nats health endpoint" "check avf-prod-nats logs and JetStream startup" "${COMPOSE[@]}" exec -T nats sh -c 'wget -qO- http://127.0.0.1:8222/healthz >/dev/null'
# `emqx_ctl` talks Erlang distribution and can fail with "not responding to pings" while
# `/api/v5/status` is already OK (same signal as docker-compose EMQX healthcheck + release.sh).
retry_check_cmd "emqx management HTTP /api/v5/status" "check avf-prod-emqx logs; confirm listener on 18083 and /api/v5/status" "${COMPOSE[@]}" exec -T emqx bash -lc 'exec 3<>/dev/tcp/127.0.0.1/18083; printf %b "GET /api/v5/status HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n" >&3; grep -Fq "emqx is running" <&3'
retry_check_cmd "api live endpoint inside container" "check avf-prod-api logs and upstream dependency readiness" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/live | grep -qx ok'
retry_check_cmd "api readiness inside container" "check avf-prod-api logs plus postgres/nats/emqx readiness" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/ready | grep -qx ok'

note "minimum production smoke"
if [[ "${SKIP_PUBLIC_HTTPS:-0}" == "1" ]]; then
	pass "public API live smoke over HTTPS skipped (SKIP_PUBLIC_HTTPS=1)"
else
	retry_check_cmd "public API live smoke over HTTPS" "check DNS, firewall, Caddy logs, and TLS issuance for ${API_DOMAIN}" bash -lc "curl -fsS 'https://${API_DOMAIN}/health/live' | grep -qx ok"
fi

note "telemetry metrics + worker readiness (when METRICS_ENABLED=true in .env.production)"
METRICS_FLAG="$(read_env_default METRICS_ENABLED false | tr '[:upper:]' '[:lower:]')"
if [[ "${METRICS_FLAG}" == "true" ]]; then
	check_cmd "worker /metrics exposes avf_telemetry_* series" "confirm METRICS_ENABLED=true and WORKER_METRICS_LISTEN; see ops/METRICS.md" "${COMPOSE[@]}" exec -T worker sh -c 'addr="${WORKER_METRICS_LISTEN:-127.0.0.1:9091}"; case "$addr" in :*) addr="127.0.0.1${addr}";; esac; curl -fsS "http://${addr}/metrics" | grep -Eq "avf_telemetry_(consumer|projection|duplicate)"'
	check_cmd "mqtt-ingest /metrics exposes avf_mqtt_ingest_dispatch_total" "confirm METRICS_ENABLED=true and MQTT_INGEST_METRICS_LISTEN" "${COMPOSE[@]}" exec -T mqtt-ingest sh -c 'addr="${MQTT_INGEST_METRICS_LISTEN:-127.0.0.1:9093}"; case "$addr" in :*) addr="127.0.0.1${addr}";; esac; curl -fsS "http://${addr}/metrics" | grep -q "avf_mqtt_ingest_dispatch_total"'
	if [[ "${SKIP_WORKER_READY_STRICT:-0}" == "1" ]]; then
		echo "SKIP_WORKER_READY_STRICT=1 set; skipping worker /health/ready on metrics port (503 may indicate JetStream backlog)"
	else
		check_cmd "worker /health/ready on metrics port" "if 503, see TELEMETRY_READINESS_* and docs/runbooks/telemetry-jetstream-resilience.md" "${COMPOSE[@]}" exec -T worker sh -c 'addr="${WORKER_METRICS_LISTEN:-127.0.0.1:9091}"; case "$addr" in :*) addr="127.0.0.1${addr}";; esac; curl -fsS "http://${addr}/health/ready" | grep -qx ok'
	fi
else
	pass "METRICS_ENABLED not true; skipping worker/mqtt-ingest telemetry metrics scrape (set true for fleet observability)"
fi

note "reverse proxy / API health (public HTTPS)"
if [[ "${SKIP_PUBLIC_HTTPS:-0}" == "1" ]]; then
	echo "SKIP_PUBLIC_HTTPS=1 set; skipping public HTTPS checks (use when DNS/TLS is still propagating)"
else
	echo "note: if the internal checks above pass but the public HTTPS checks fail, focus on DNS, external routing/firewall, or ACME/TLS issuance before debugging the API process itself"
	retry_check_cmd "public /health/live over HTTPS" "internal checks passing + public failure usually means DNS, upstream firewall on 80/443, or ACME/TLS issuance — inspect Caddy logs: docker logs avf-prod-caddy" bash -lc "curl -fsS 'https://${API_DOMAIN}/health/live' | grep -qx ok"
	retry_check_cmd "public /health/ready over HTTPS" "internal checks passing + public failure usually means DNS, upstream firewall on 80/443, or ACME/TLS issuance — inspect Caddy logs: docker logs avf-prod-caddy" bash -lc "curl -fsS 'https://${API_DOMAIN}/health/ready' | grep -qx ok"
fi

if (( failures > 0 )); then
	note "healthcheck failed — docker compose ps (refresh)"
	"${COMPOSE[@]}" ps 2>&1 || true

	# Same eight as release.sh / STACK_GATE rollout gate.
	rollout_critical=(avf-prod-postgres avf-prod-nats avf-prod-emqx avf-prod-api avf-prod-worker avf-prod-mqtt-ingest avf-prod-reconciler avf-prod-caddy)
	echo "==> deep container diagnostics (unhealthy or not running)" >&2
	for c in "${rollout_critical[@]}"; do
		if container_warrants_deep_diag "${c}"; then
			show_container_diagnostics_deep "${c}"
		fi
	done
	echo "hint: for logical check failures (e.g. public HTTPS) with healthy containers, see FAIL lines above." >&2
	echo "healthcheck_prod: FAIL (${failures} checks failed)" >&2
	exit 1
fi

echo "healthcheck_prod: PASS"
