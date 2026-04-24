#!/usr/bin/env bash
# Enterprise telemetry storm / offline-replay validation gate (staging default; production explicit).
# Publishes valid device envelopes to {MQTT_TOPIC_PREFIX}/{machine_id}/telemetry via mosquitto_pub.
#
# Sequential staging evidence (100×100, 500×200, 1000×500): run_staging_telemetry_storm_suite.sh
#
# Defaults: DRY_RUN=true, LOAD_TEST_ENV=staging, EXECUTE_LOAD_TEST=false — no credentials required.
#
# Certification (non-dry execute): requires scrapable Prometheus metrics from mqtt-ingest and worker
# (METRICS_ENABLED=true on both, or MQTT_INGEST_METRICS_URL / WORKER_METRICS_URL). Without usable
# metrics deltas the script FAILS — it will not report overall PASS.
#
# Required for EXECUTE_LOAD_TEST=true:
#   - mosquitto_clients (mosquitto_pub)
#   - MQTT_BROKER_URL or MQTT_HOST + MQTT_PORT
#   - MQTT_USERNAME, MQTT_PASSWORD, MQTT_TOPIC_PREFIX
#   - python3 (payload JSON, URL parse, accounting)
#
# shellcheck disable=SC1091
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
# Offline replay JSON contract fixtures (vend, payment, etc.): "${REPO_ROOT}/testdata/telemetry/"
ACCOUNTING_PY="${REPO_ROOT}/tools/telemetry-load/storm_accounting.py"

# --- tunables (env) ---
DRY_RUN="${DRY_RUN:-true}"
EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST:-false}"
LOAD_TEST_ENV="${LOAD_TEST_ENV:-staging}" # staging | production

MACHINE_COUNT="${MACHINE_COUNT:-10}"
EVENTS_PER_MACHINE="${EVENTS_PER_MACHINE:-50}"
EVENT_RATE_PER_MACHINE="${EVENT_RATE_PER_MACHINE:-2}"
CRITICAL_EVENT_RATIO="${CRITICAL_EVENT_RATIO:-0.1}"
# Integer 0..99: fraction of critical events that receive an immediate identical second publish (duplicate replay).
CRITICAL_DUPLICATE_REPLAY_PERCENT="${CRITICAL_DUPLICATE_REPLAY_PERCENT:-0}"

MQTT_BROKER_URL="${MQTT_BROKER_URL:-}"
MQTT_HOST="${MQTT_HOST:-}"
MQTT_PORT="${MQTT_PORT:-}"
MQTT_USERNAME="${MQTT_USERNAME:-}"
MQTT_PASSWORD="${MQTT_PASSWORD:-}"
MQTT_TOPIC_PREFIX="${MQTT_TOPIC_PREFIX:-}"
MQTT_TLS_CAFILE="${MQTT_TLS_CAFILE:-}"
MQTT_USE_TLS="${MQTT_USE_TLS:-}"

SCENARIO_PRESET="${SCENARIO_PRESET:-}" # 100x100 | 500x200 | 1000x500

CONFIRM_PROD_LOAD_TEST="${CONFIRM_PROD_LOAD_TEST:-false}"

RESULT_JSON_FILE="${RESULT_JSON_FILE:-telemetry-storm-result.json}"

# Compose / observability
COMPOSE_FILE="${COMPOSE_FILE:-${REPO_ROOT}/deployments/prod/app-node/docker-compose.app-node.yml}"
ENV_FILE="${ENV_FILE:-${REPO_ROOT}/deployments/prod/app-node/.env.app-node}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-}"
SKIP_DOCKER_GATES="${SKIP_DOCKER_GATES:-false}"
RUN_POST_GATES="${RUN_POST_GATES:-true}"

API_READY_URL="${API_READY_URL:-}"
WORKER_READY_URL="${WORKER_READY_URL:-}"
READINESS_MAX_FAIL_SECONDS="${READINESS_MAX_FAIL_SECONDS:-120}"
READINESS_POLL_INTERVAL="${READINESS_POLL_INTERVAL:-3}"

MQTT_INGEST_METRICS_URL="${MQTT_INGEST_METRICS_URL:-}"
WORKER_METRICS_URL="${WORKER_METRICS_URL:-}"

# Fail if max avf_telemetry_consumer_lag exceeds this after run (0 = skip lag gate).
MAX_LAG_FAIL_THRESHOLD="${MAX_LAG_FAIL_THRESHOLD:-0}"

WAVE_PARALLEL_LIMIT="${WAVE_PARALLEL_LIMIT:-256}"
STABILIZE_SECONDS="${STABILIZE_SECONDS:-20}"

MACHINE_IDS_FILE="${MACHINE_IDS_FILE:-}"

STORM_DETERMINISTIC_TIMESTAMPS="${STORM_DETERMINISTIC_TIMESTAMPS:-false}"
STORM_EPOCH_BASE="${STORM_EPOCH_BASE:-1700000000}"

# Mosquitto publish failures recorded in FAILURES_FILE (background jobs cannot update a shell counter).
FAILURES_FILE=""

fail() { echo "telemetry_storm_load_test: error: $*" >&2; exit 1; }
warn() { echo "telemetry_storm_load_test: warning: $*" >&2; }
note() { echo "==> $*"; }

# Append to STRICT_FAIL_REASON so gate + health + accounting failures all surface (no silent overwrite).
append_strict_fail_reason() {
	local msg="$1"
	[[ -n "${msg}" ]] || return
	if [[ -z "${STRICT_FAIL_REASON}" ]]; then
		STRICT_FAIL_REASON="${msg}"
	else
		STRICT_FAIL_REASON="${STRICT_FAIL_REASON}; ${msg}"
	fi
}

usage() {
	cat <<'EOF'
Usage: telemetry_storm_load_test.sh

Environment (main):
  DRY_RUN=true|false              default true — plan + accounting preview; no MQTT
  EXECUTE_LOAD_TEST=true|false    default false — publish traffic (requires broker + creds)
  LOAD_TEST_ENV=staging|production  default staging
    Any production run requires CONFIRM_PROD_LOAD_TEST=true (including dry-run).

Load shape:
  MACHINE_COUNT, EVENTS_PER_MACHINE, EVENT_RATE_PER_MACHINE
  CRITICAL_EVENT_RATIO            0..1 → mapped to integer percent for bucket threshold
  CRITICAL_DUPLICATE_REPLAY_PERCENT 0..99 — duplicate identical critical publishes (idempotency drill)
  SCENARIO_PRESET                 100x100 | 500x200 | 1000x500

Accounting / certification:
  RESULT_JSON_FILE                default telemetry-storm-result.json
  STRICT_ACCOUNTING=true|false    default true — FAIL if Prometheus deltas unavailable for execute runs
  SKIP_DOCKER_GATES=true          skips compose metrics (usually makes STRICT accounting fail)

MQTT: (same as before)
  MQTT_BROKER_URL, MQTT_TOPIC_PREFIX, credentials, TLS CA...

Gates:
  API_READY_URL, WORKER_READY_URL   optional concurrent /health/ready monitors
  MAX_LAG_FAIL_THRESHOLD           max avf_telemetry_consumer_lag after run (0=off)

Examples:
  DRY_RUN=true SCENARIO_PRESET=1000x500 bash deployments/prod/scripts/telemetry_storm_load_test.sh
  EXECUTE_LOAD_TEST=true LOAD_TEST_ENV=staging ... bash deployments/prod/scripts/telemetry_storm_load_test.sh
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
	usage
	exit 0
fi

STRICT_ACCOUNTING="${STRICT_ACCOUNTING:-true}"
export STRICT_ACCOUNTING

truthy() {
	case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')" in
	true | 1 | yes) return 0 ;;
	*) return 1 ;;
	esac
}

apply_scenario_preset() {
	case "${SCENARIO_PRESET}" in
	100x100)
		MACHINE_COUNT=100
		EVENTS_PER_MACHINE=100
		;;
	500x200)
		MACHINE_COUNT=500
		EVENTS_PER_MACHINE=200
		;;
	1000x500)
		MACHINE_COUNT=1000
		EVENTS_PER_MACHINE=500
		;;
	"") ;;
	*) fail "unknown SCENARIO_PRESET=${SCENARIO_PRESET} (use 100x100, 500x200, 1000x500)" ;;
	esac
}

apply_scenario_preset

if [[ "$(printf '%s' "${LOAD_TEST_ENV}" | tr '[:upper:]' '[:lower:]')" == "production" ]]; then
	if ! truthy "${CONFIRM_PROD_LOAD_TEST}"; then
		fail "LOAD_TEST_ENV=production requires CONFIRM_PROD_LOAD_TEST=true"
	fi
fi

if [[ -n "${MACHINE_IDS_FILE}" ]]; then
	[[ -f "${MACHINE_IDS_FILE}" ]] || fail "MACHINE_IDS_FILE not found: ${MACHINE_IDS_FILE}"
	_mids_nlines="$(grep -cve '^[[:space:]]*$' "${MACHINE_IDS_FILE}" || true)"
	if awk -v n="${_mids_nlines}" -v m="${MACHINE_COUNT}" 'BEGIN {exit !(n>=m)}'; then
		:
	else
		fail "MACHINE_IDS_FILE has ${_mids_nlines} non-empty lines; need >= MACHINE_COUNT (${MACHINE_COUNT})"
	fi
	unset _mids_nlines
fi

if truthy "${EXECUTE_LOAD_TEST}"; then
	DRY_RUN=false
fi

if awk -v c="${MACHINE_COUNT}" 'BEGIN {exit !(c>0 && c==int(c))}'; then
	:
else
	fail "MACHINE_COUNT must be a positive integer"
fi
if awk -v c="${EVENTS_PER_MACHINE}" 'BEGIN {exit !(c>0 && c==int(c))}'; then
	:
else
	fail "EVENTS_PER_MACHINE must be a positive integer"
fi
awk -v r="${EVENT_RATE_PER_MACHINE}" 'BEGIN {if (r<=0) exit 1; exit 0}' || fail "EVENT_RATE_PER_MACHINE must be > 0"
awk -v r="${CRITICAL_EVENT_RATIO}" 'BEGIN {if (r<0 || r>1) exit 1; exit 0}' || fail "CRITICAL_EVENT_RATIO must be 0..1"
awk -v r="${CRITICAL_DUPLICATE_REPLAY_PERCENT}" 'BEGIN {if (r<0 || r>99) exit 1; exit 0}' ||
	fail "CRITICAL_DUPLICATE_REPLAY_PERCENT must be 0..99"

TOTAL_EVENTS=$((MACHINE_COUNT * EVENTS_PER_MACHINE))
CRITICAL_PCT="$(awk -v r="${CRITICAL_EVENT_RATIO}" 'BEGIN {printf "%d", int(r*100+1e-9)}')"

[[ -f "${ACCOUNTING_PY}" ]] || fail "missing ${ACCOUNTING_PY}"

EXPECT_JSON="$(python3 "${ACCOUNTING_PY}" expect \
	--machine-count "${MACHINE_COUNT}" \
	--events-per-machine "${EVENTS_PER_MACHINE}" \
	--critical-pct "${CRITICAL_PCT}" \
	--duplicate-pct "${CRITICAL_DUPLICATE_REPLAY_PERCENT}")"
CRITICAL_EXPECTED_UNIQUE="$(printf '%s' "${EXPECT_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["critical_expected_unique"])')"
CRITICAL_DUP_PLANNED="$(printf '%s' "${EXPECT_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["critical_duplicate_replays_planned"])')"
CRITICAL_PUBLISH_PLANNED="$(printf '%s' "${EXPECT_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["critical_publishes_planned"])')"

# --- stable UUID-shaped ids ---
hex32() {
	local s="$1"
	if command -v sha256sum >/dev/null 2>&1; then
		printf '%s' "${s}" | sha256sum | awk '{print substr($1,1,32)}'
	else
		printf '%s' "${s}" | shasum -a 256 2>/dev/null | awk '{print substr($1,1,32)}'
	fi
}

uuid_from_hex32() {
	local h="$1"
	printf '%s-%s-%s-%s-%s' "${h:0:8}" "${h:8:4}" "${h:12:4}" "${h:16:4}" "${h:20:12}"
}

machine_uuid() {
	local idx="$1"
	if [[ -n "${MACHINE_IDS_FILE}" ]]; then
		local line
		line="$(sed -n "${idx}p" "${MACHINE_IDS_FILE}" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')"
		[[ -n "${line}" ]] || fail "MACHINE_IDS_FILE row ${idx} missing (need ${MACHINE_COUNT} lines)"
		[[ "${line}" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]] || fail "invalid machine UUID at line ${idx} of MACHINE_IDS_FILE"
		printf '%s' "${line}"
		return
	fi
	local h
	h="$(hex32 "avf-telemetry-storm|machine|${idx}")"
	uuid_from_hex32 "${h}"
}

boot_uuid() {
	local idx="$1"
	local h
	h="$(hex32 "avf-telemetry-storm|boot|${idx}")"
	uuid_from_hex32 "${h}"
}

is_critical_event() {
	local idx="$1" wave="$2"
	local bucket=$(( (idx * 10007 + wave * 7919) % 100 ))
	(( bucket < CRITICAL_PCT ))
}

should_duplicate_replay() {
	local idx="$1" wave="$2"
	(( CRITICAL_DUPLICATE_REPLAY_PERCENT > 0 )) || return 1
	is_critical_event "${idx}" "${wave}" || return 1
	local b=$(( (idx * 131 + wave * 171) % 100 ))
	(( b < CRITICAL_DUPLICATE_REPLAY_PERCENT ))
}

event_type_for_wave() {
	local idx="$1"
	local wave="$2"
	local bucket=$(( (idx * 10007 + wave * 7919) % 100 ))
	if (( bucket < CRITICAL_PCT )); then
		case $(( (idx + wave) % 3 )) in
		0) printf '%s' 'events.vend' ;;
		1) printf '%s' 'events.cash' ;;
		*) printf '%s' 'events.inventory' ;;
		esac
	else
		case $(( wave % 2 )) in
		0) printf '%s' 'heartbeat' ;;
		*) printf '%s' 'metrics.cpu' ;;
		esac
	fi
}

occurred_at_iso() {
	local idx="$1"
	local wave="$2"
	if truthy "${STORM_DETERMINISTIC_TIMESTAMPS}"; then
		local off=$((STORM_EPOCH_BASE + idx * 1000 + wave))
		if date -u -r "${off}" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null; then
			return
		fi
		date -u -d "@${off}" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null && return
		date -u '+%Y-%m-%dT%H:%M:%SZ'
	else
		date -u '+%Y-%m-%dT%H:%M:%SZ'
	fi
}

envelope_json() {
	local mid="$1"
	local boot="$2"
	local wave="$3"
	local idx="$4"
	local et="$5"
	local occurred="$6"
	STORM_MID="${mid}" STORM_BOOT="${boot}" STORM_WAVE="${wave}" STORM_IDX="${idx}" STORM_ET="${et}" STORM_AT="${occurred}" python3 <<'PY'
import json, os

mid = os.environ["STORM_MID"]
boot = os.environ["STORM_BOOT"]
wave = int(os.environ["STORM_WAVE"])
idx = int(os.environ["STORM_IDX"])
et = os.environ["STORM_ET"]
occurred = os.environ["STORM_AT"]
# Stable identities for idempotency (aligned with backend critical ingest).
event_id = f"storm-{mid}-{wave}-{idx}"
idem = f"storm:{mid}:{wave}:{idx}:{et}"
inner = {"storm": True, "idx": idx, "wave": wave, "event_type": et}
obj = {
    "schema_version": 1,
    "machine_id": mid,
    "event_id": event_id,
    "boot_id": boot,
    "seq_no": wave,
    "occurred_at": occurred,
    "emitted_at": occurred,
    "dedupe_key": idem,
    "idempotency_key": idem,
    "event_type": et,
    "payload": inner,
}
print(json.dumps(obj, separators=(",", ":")))
PY
}

publish_one() {
	local idx="$1"
	local wave="$2"
	local mid boot et occurred payload topic
	mid="$(machine_uuid "${idx}")"
	boot="$(boot_uuid "${idx}")"
	et="$(event_type_for_wave "${idx}" "${wave}")"
	occurred="$(occurred_at_iso "${idx}" "${wave}")"
	payload="$(envelope_json "${mid}" "${boot}" "${wave}" "${idx}" "${et}" "${occurred}")"
	topic="${MQTT_TOPIC_PREFIX}/${mid}/telemetry"

	if truthy "${DRY_RUN}"; then
		return 0
	fi

	local -a pub=(mosquitto_pub "${MOSQ_AUTH[@]}" "${MOSQ_TLS[@]}" -h "${MOSQ_HOST}" -p "${MOSQ_PORT}" -t "${topic}" -m "${payload}" -q 1)
	if ! "${pub[@]}"; then
		if [[ -n "${FAILURES_FILE}" ]]; then
			printf '1\n' >>"${FAILURES_FILE}"
		fi
		echo "telemetry_storm_load_test: mosquitto_pub failed idx=${idx} wave=${wave}" >&2
	fi
}

publish_wave_message() {
	local idx="$1"
	local wave="$2"
	publish_one "${idx}" "${wave}"
	if should_duplicate_replay "${idx}" "${wave}"; then
		publish_one "${idx}" "${wave}"
	fi
}

parse_mqtt_broker_url() {
	if [[ -n "${MQTT_HOST}" && -n "${MQTT_PORT}" ]]; then
		MOSQ_HOST="${MQTT_HOST}"
		MOSQ_PORT="${MQTT_PORT}"
		if truthy "${MQTT_USE_TLS}"; then
			MOSQ_TLS=(--tls-version tlsv1.2)
			[[ -n "${MQTT_TLS_CAFILE}" ]] && MOSQ_TLS+=(--cafile "${MQTT_TLS_CAFILE}")
		else
			MOSQ_TLS=()
		fi
		if [[ -n "${MQTT_USERNAME}" || -n "${MQTT_PASSWORD}" ]]; then
			MOSQ_AUTH=(-u "${MQTT_USERNAME}" -P "${MQTT_PASSWORD}")
		else
			MOSQ_AUTH=()
		fi
		return
	fi
	[[ -n "${MQTT_BROKER_URL}" ]] || fail "set MQTT_BROKER_URL or MQTT_HOST+MQTT_PORT for execute mode"

	local _parsed
	_parsed="$(
		python3 - "${MQTT_BROKER_URL}" "${MQTT_USERNAME}" "${MQTT_PASSWORD}" <<'PY'
import sys, urllib.parse
url = sys.argv[1]
user_override = sys.argv[2]
pass_override = sys.argv[3]
u = urllib.parse.urlsplit(url)
scheme = (u.scheme or "mqtt").lower()
if u.hostname is None:
    raise SystemExit("MQTT_BROKER_URL missing host")
host = u.hostname
port = u.port
if port is None:
    port = 8883 if scheme in ("mqtts", "ssl") else 1883
user = urllib.parse.unquote(u.username or "") or user_override
password = urllib.parse.unquote(u.password or "") or pass_override
tls = "1" if scheme in ("mqtts", "ssl") else "0"
print(f"{host}\n{port}\n{tls}\n{user}\n{password}")
PY
	)"
	MOSQ_HOST="$(printf '%s\n' "${_parsed}" | sed -n '1p')"
	MOSQ_PORT="$(printf '%s\n' "${_parsed}" | sed -n '2p')"
	MOSQ_TLS_FLAG="$(printf '%s\n' "${_parsed}" | sed -n '3p')"
	MOSQ_USER="$(printf '%s\n' "${_parsed}" | sed -n '4p')"
	MOSQ_PASS="$(printf '%s\n' "${_parsed}" | sed -n '5p')"

	if [[ -n "${MOSQ_USER}" || -n "${MOSQ_PASS}" ]]; then
		MOSQ_AUTH=(-u "${MOSQ_USER}" -P "${MOSQ_PASS}")
	else
		MOSQ_AUTH=()
	fi
	if [[ "${MOSQ_TLS_FLAG}" == "1" ]]; then
		MOSQ_TLS=(--tls-version tlsv1.2)
		[[ -n "${MQTT_TLS_CAFILE}" ]] && MOSQ_TLS+=(--cafile "${MQTT_TLS_CAFILE}")
		if [[ -z "${MQTT_TLS_CAFILE}" ]]; then
			warn "mqtts without MQTT_TLS_CAFILE: add --cafile if mosquitto_pub fails TLS verify"
		fi
	else
		MOSQ_TLS=()
	fi
}

print_scrape_commands() {
	note "Scrape /metrics (certification requires METRICS_ENABLED=true or direct *_METRICS_URL)"
	local cf ef proj
	cf="${COMPOSE_FILE}"
	ef="${ENV_FILE}"
	proj="${COMPOSE_PROJECT}"

	echo "--- mqtt-ingest /metrics ---"
	if [[ -n "${MQTT_INGEST_METRICS_URL}" ]]; then
		printf 'curl -fsS %q | grep -E "avf_telemetry_ingest_|avf_mqtt_ingest_"\n' "${MQTT_INGEST_METRICS_URL}"
	else
		if [[ -n "${proj}" ]]; then
			echo "  docker compose -p \"${proj}\" -f \"${cf}\" --env-file \"${ef}\" exec -T mqtt-ingest sh -c \\"
		else
			echo "  docker compose -f \"${cf}\" --env-file \"${ef}\" exec -T mqtt-ingest sh -c \\"
		fi
		echo "    'addr=\"\${MQTT_INGEST_METRICS_LISTEN:-127.0.0.1:9093}\"; case \"\$addr\" in :*) addr=\"127.0.0.1\${addr}\";; esac; curl -fsS \"http://\${addr}/metrics\"' \\"
		echo "    | grep -E 'avf_telemetry_ingest_|avf_mqtt_ingest_'"
	fi

	echo "--- worker /metrics ---"
	if [[ -n "${WORKER_METRICS_URL}" ]]; then
		printf 'curl -fsS %q | grep -E "avf_telemetry_consumer_lag|avf_telemetry_projection_|avf_telemetry_duplicate|avf_telemetry_idempotency"\n' "${WORKER_METRICS_URL}"
	else
		if [[ -n "${proj}" ]]; then
			echo "  docker compose -p \"${proj}\" -f \"${cf}\" --env-file \"${ef}\" exec -T worker sh -c \\"
		else
			echo "  docker compose -f \"${cf}\" --env-file \"${ef}\" exec -T worker sh -c \\"
		fi
		echo "    'addr=\"\${WORKER_METRICS_LISTEN:-127.0.0.1:9091}\"; case \"\$addr\" in :*) addr=\"127.0.0.1\${addr}\";; esac; curl -fsS \"http://\${addr}/metrics\"' \\"
		echo "    | grep -E 'avf_telemetry_consumer_lag|avf_telemetry_projection_|avf_telemetry_duplicate|avf_telemetry_idempotency'"
	fi
}

fetch_metrics_to_optional() {
	local dest="$1"
	local kind="$2"
	if [[ "${kind}" == "mqtt" ]]; then
		if [[ -n "${MQTT_INGEST_METRICS_URL}" ]]; then
			if curl -fsS "${MQTT_INGEST_METRICS_URL}" -o "${dest}" 2>/dev/null; then
				return 0
			fi
			return 1
		fi
	fi
	if [[ "${kind}" == "worker" ]]; then
		if [[ -n "${WORKER_METRICS_URL}" ]]; then
			if curl -fsS "${WORKER_METRICS_URL}" -o "${dest}" 2>/dev/null; then
				return 0
			fi
			return 1
		fi
	fi
	if truthy "${SKIP_DOCKER_GATES}"; then
		return 1
	fi
	local dc=(docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")
	if [[ -n "${COMPOSE_PROJECT}" ]]; then
		dc=(docker compose -p "${COMPOSE_PROJECT}" -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")
	fi
	case "${kind}" in
	mqtt)
		if "${dc[@]}" exec -T mqtt-ingest sh -c \
			'addr="${MQTT_INGEST_METRICS_LISTEN:-127.0.0.1:9093}"; case "$addr" in :*) addr="127.0.0.1${addr}";; esac; curl -fsS "http://${addr}/metrics"' \
			>"${dest}" 2>/dev/null; then
			return 0
		fi
		;;
	worker)
		if "${dc[@]}" exec -T worker sh -c \
			'addr="${WORKER_METRICS_LISTEN:-127.0.0.1:9091}"; case "$addr" in :*) addr="127.0.0.1${addr}";; esac; curl -fsS "http://${addr}/metrics"' \
			>"${dest}" 2>/dev/null; then
			return 0
		fi
		;;
	*) return 1 ;;
	esac
	return 1
}

compose_restart_count() {
	local svc="$1"
	local dc=(docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")
	if [[ -n "${COMPOSE_PROJECT}" ]]; then
		dc=(docker compose -p "${COMPOSE_PROJECT}" -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")
	fi
	local cid
	cid="$("${dc[@]}" ps -q "${svc}" 2>/dev/null | head -n1 || true)"
	[[ -n "${cid}" ]] || { echo 0; return; }
	docker inspect "${cid}" --format '{{.RestartCount}}' 2>/dev/null || echo 0
}

container_oom_killed() {
	local svc="$1"
	local dc=(docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")
	if [[ -n "${COMPOSE_PROJECT}" ]]; then
		dc=(docker compose -p "${COMPOSE_PROJECT}" -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")
	fi
	local cid
	cid="$("${dc[@]}" ps -q "${svc}" 2>/dev/null | head -n1 || true)"
	[[ -n "${cid}" ]] || { echo "false"; return; }
	docker inspect "${cid}" --format '{{.State.OOMKilled}}' 2>/dev/null || echo "false"
}

readiness_monitor_loop() {
	local url="$1"
	local max_fail_seconds="$2"
	local interval="$3"
	local fail_file="$4"
	local name="$5"
	local bad_since=""
	while true; do
		if curl -fsS --max-time "${interval}" "${url}" 2>/dev/null | grep -qx ok; then
			bad_since=""
		else
			if [[ -z "${bad_since}" ]]; then
				bad_since="${SECONDS}"
			elif ((SECONDS - bad_since >= max_fail_seconds)); then
				echo "telemetry_storm_load_test: ${name} ${url} not ok for >= ${max_fail_seconds}s" >&2
				printf '1' >"${fail_file}"
				return 1
			fi
		fi
		sleep "${interval}"
	done
}

post_run_gates() {
	local before_m="$1"
	local before_w="$2"
	local after_m="$3"
	local after_w="$4"
	local br_mqtt="$5"
	local br_worker="$6"
	local ar_mqtt="$7"
	local ar_worker="$8"

	if [[ ! -s "${before_m}" || ! -s "${after_m}" ]]; then
		if truthy "${STRICT_ACCOUNTING}"; then
			warn "post_run_gates: mqtt-ingest metrics snapshot empty — cannot certify (STRICT_ACCOUNTING=true)"
			GATE_FAIL_REASON="mqtt_ingest_metrics_snapshot_empty"
			DB_POOL_RESULT="unknown"
			return 1
		fi
		warn "post_run_gates: mqtt-ingest metrics snapshot empty — skipping drop/reject log checks"
		DB_POOL_RESULT="unknown"
		return 0
	fi

	local drop_non_droppable_before drop_non_droppable_after
	drop_non_droppable_before="$(awk '$1 ~ /^avf_telemetry_ingest_dropped_total/ && $0 !~ /reason=\"droppable_queue_full\"/ {sum+=$(NF)} END{print sum+0}' "${before_m}")"
	drop_non_droppable_after="$(awk '$1 ~ /^avf_telemetry_ingest_dropped_total/ && $0 !~ /reason=\"droppable_queue_full\"/ {sum+=$(NF)} END{print sum+0}' "${after_m}")"
	local drop_nd_delta=$((drop_non_droppable_after - drop_non_droppable_before))
	if ((drop_nd_delta > 0)); then
		echo "telemetry_storm_load_test: ingest dropped non-droppable events (delta=${drop_nd_delta})" >&2
		GATE_FAIL_REASON="non_droppable_drop_delta=${drop_nd_delta}"
		DB_POOL_RESULT="unknown"
		return 1
	fi

	local crit_rej_before crit_rej_after
	crit_rej_before="$(awk '$1 ~ /^avf_telemetry_ingest_rejected_total/ && $0 ~ /reason=\"(critical_queue_full|critical_queue_full_timeout)\"/ {sum+=$(NF)} END{print sum+0}' "${before_m}")"
	crit_rej_after="$(awk '$1 ~ /^avf_telemetry_ingest_rejected_total/ && $0 ~ /reason=\"(critical_queue_full|critical_queue_full_timeout)\"/ {sum+=$(NF)} END{print sum+0}' "${after_m}")"
	local crit_delta=$((crit_rej_after - crit_rej_before))
	if ((crit_delta > 0)); then
		echo "telemetry_storm_load_test: critical ingress backpressure (rejected delta=${crit_delta})" >&2
		GATE_FAIL_REASON="critical_ingress_reject_delta=${crit_delta}"
		DB_POOL_RESULT="unknown"
		return 1
	fi

	if ((ar_mqtt > br_mqtt || ar_worker > br_worker)); then
		echo "telemetry_storm_load_test: container restart (mqtt-ingest ${br_mqtt}->${ar_mqtt}, worker ${br_worker}->${ar_worker})" >&2
		GATE_FAIL_REASON="container_restart"
		DB_POOL_RESULT="unknown"
		return 1
	fi

	if ! truthy "${SKIP_DOCKER_GATES}"; then
		local dc=(docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")
		if [[ -n "${COMPOSE_PROJECT}" ]]; then
			dc=(docker compose -p "${COMPOSE_PROJECT}" -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")
		fi
		if "${dc[@]}" logs mqtt-ingest worker --since "${LOGS_SINCE}" 2>&1 |
			grep -Ei 'too many clients|maxclients|MaxClientsInSessionMode|pool exhausted|remaining connection slots|too many connections|FATAL.*too many connections'; then
			echo "telemetry_storm_load_test: DB / pool pressure signatures in logs" >&2
			GATE_FAIL_REASON="db_pool_signature_in_logs"
			DB_POOL_RESULT="fail"
			return 1
		fi
		for svc in mqtt-ingest worker; do
			if [[ "$(container_oom_killed "${svc}")" == "true" ]]; then
				echo "telemetry_storm_load_test: OOMKilled for ${svc}" >&2
				GATE_FAIL_REASON="oom_killed_${svc}"
				DB_POOL_RESULT="unknown"
				return 1
			fi
		done
	fi

	DB_POOL_RESULT="ok"
	note "post-run gates: OK (drops_non_droppable_delta=${drop_nd_delta}, critical_reject_delta=${crit_delta})"
	return 0
}

write_result_json() {
	local overall_pass="$1"
	local json_payload="$2"
	STORM_RESULT_JSON="${RESULT_JSON_FILE}" \
		STORM_RESULT_OVERALL="${overall_pass}" \
		STORM_RESULT_PAYLOAD="${json_payload}" \
		python3 <<'PY'
import json, os, time

out_path = os.environ["STORM_RESULT_JSON"]
overall = os.environ["STORM_RESULT_OVERALL"] == "true"
payload = json.loads(os.environ["STORM_RESULT_PAYLOAD"])
payload["overall_pass"] = overall
ts = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
payload["finished_at_utc"] = ts
payload["completed_at_utc"] = ts
payload["final_result"] = "pass" if overall else "fail"
# Normalize health for gates expecting pass|ok
hr = payload.get("health_result")
if hr == "ok":
    payload["health_result"] = "pass"
whr = payload.get("worker_health_result")
if whr == "ok":
    payload["worker_health_result"] = "pass"
dbr = payload.get("db_pool_result")
if dbr == "ok":
    payload["db_pool_result"] = "pass"
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(payload, f, indent=2)
    f.write("\n")
print("telemetry_storm_load_test: wrote", out_path)
PY
}

# --- JSON payload assembly (python reads env for nested fields passed as single json string) ---
build_result_payload() {
	python3 <<'PY'
import json, os

def fkey(k, default=None):
    v = os.environ.get(k)
    if v is None or v == "":
        return default
    try:
        return float(v)
    except ValueError:
        return default

dup_sum = fkey("DUPLICATE_PROJ_DELTA")
idem_c = fkey("IDEMPOTENCY_CONFLICT_DELTA")
crit_acc_delta = fkey("CRITICAL_ACCEPTED_DELTA")
# duplicate_detected: same idempotency_key reused with different payload (must stay 0).
duplicate_detected = (idem_c or 0) > 0
duplicate_critical_effects = 1.0 if duplicate_detected else 0.0
try:
    critical_accepted_int = int(round(float(crit_acc_delta))) if crit_acc_delta is not None else None
except (TypeError, ValueError):
    critical_accepted_int = None

strict_reason = (os.environ.get("STRICT_FAIL_REASON") or "") + ";" + (os.environ.get("GATE_FAIL_REASON") or "")
if "container_restart" in strict_reason or "oom_killed" in strict_reason:
    restart_result = "fail"
elif os.environ.get("DRY_RUN", "").lower() in ("1", "true", "yes"):
    restart_result = "skipped"
elif os.environ.get("ACCOUNTING_STATUS") == "dry_run_no_execute":
    restart_result = "skipped"
else:
    restart_result = "pass"

p = {
  "scenario": os.environ.get("SCENARIO_PRESET") or f'{os.environ["MACHINE_COUNT"]}x{os.environ["EVENTS_PER_MACHINE"]}',
  "scenario_preset": os.environ.get("SCENARIO_PRESET") or None,
  "load_test_env": os.environ.get("LOAD_TEST_ENV"),
  "dry_run": os.environ.get("DRY_RUN", "").lower() in ("1", "true", "yes"),
  "execute_load_test": os.environ.get("EXECUTE_LOAD_TEST", "").lower() in ("1", "true", "yes"),
  "machine_count": int(os.environ["MACHINE_COUNT"]),
  "events_per_machine": int(os.environ["EVENTS_PER_MACHINE"]),
  "total_events": int(os.environ["TOTAL_EVENTS"]),
  "mqtt_publishes_planned": int(os.environ.get("TOTAL_MQTT_PUBLISHES", os.environ["TOTAL_EVENTS"])),
  "critical_event_ratio": float(os.environ["CRITICAL_EVENT_RATIO"]),
  "critical_pct_bucket": int(os.environ["CRITICAL_PCT"]),
  "critical_expected_unique": int(os.environ["CRITICAL_EXPECTED_UNIQUE"]),
  "critical_expected": int(os.environ["CRITICAL_PUBLISH_PLANNED"]),
  "critical_duplicate_replays_planned": int(os.environ["CRITICAL_DUP_PLANNED"]),
  "critical_publishes_planned": int(os.environ["CRITICAL_PUBLISH_PLANNED"]),
  "critical_duplicate_replay_percent": int(os.environ["CRITICAL_DUPLICATE_REPLAY_PERCENT"]),
  "mosquitto_publish_failures": int(os.environ.get("MOSQUITTO_PUBLISH_FAILS", "0")),
  "accounting_status": os.environ.get("ACCOUNTING_STATUS", "unknown"),
  "critical_accepted_delta": fkey("CRITICAL_ACCEPTED_DELTA"),
  "critical_accepted": critical_accepted_int if critical_accepted_int is not None else fkey("CRITICAL_ACCEPTED_DELTA"),
  "critical_lost": fkey("CRITICAL_LOST"),
  "duplicate_detected": duplicate_detected,
  "duplicate_critical_effects": duplicate_critical_effects,
  "duplicate_projection_ack_delta_sum": dup_sum,
  "dispatch_telemetry_ok_delta": fkey("DISPATCH_OK_DELTA"),
  "idempotency_conflict_delta": fkey("IDEMPOTENCY_CONFLICT_DELTA"),
  "max_lag": fkey("MAX_LAG"),
  "health_result": os.environ.get("HEALTH_RESULT", "unknown"),
  "worker_health_result": os.environ.get("WORKER_HEALTH_RESULT", "unknown"),
  "db_pool_result": os.environ.get("DB_POOL_RESULT", "unknown"),
  "restart_result": restart_result,
  "strict_accounting_failed_reason": os.environ.get("STRICT_FAIL_REASON") or None,
}
print(json.dumps(p))
PY
}

# --- main flow ---
note "telemetry storm load test"
echo "  LOAD_TEST_ENV=${LOAD_TEST_ENV} DRY_RUN=${DRY_RUN} EXECUTE_LOAD_TEST=${EXECUTE_LOAD_TEST}"
echo "  machines=${MACHINE_COUNT} events_per_machine=${EVENTS_PER_MACHINE} total_events=${TOTAL_EVENTS} rate_per_machine=${EVENT_RATE_PER_MACHINE}/s"
echo "  critical: expected_unique=${CRITICAL_EXPECTED_UNIQUE} dup_replays_planned=${CRITICAL_DUP_PLANNED} publishes_planned=${CRITICAL_PUBLISH_PLANNED}"
echo "  CRITICAL_DUPLICATE_REPLAY_PERCENT=${CRITICAL_DUPLICATE_REPLAY_PERCENT}"

if [[ -z "${MACHINE_IDS_FILE}" ]]; then
	warn "MACHINE_IDS_FILE unset — synthetic machine UUIDs; mqtt-ingest will fail GetMachineOrgSite unless those IDs exist in Postgres."
fi

SLEEP_WAVE="$(awk -v r="${EVENT_RATE_PER_MACHINE}" 'BEGIN {printf "%.6f", (r>0)?(1/r):1}')"

command -v python3 >/dev/null 2>&1 || fail "python3 is required"

MQTT_TOPIC_PREFIX="${MQTT_TOPIC_PREFIX#/}"
MQTT_TOPIC_PREFIX="${MQTT_TOPIC_PREFIX%/}"

if [[ -z "${MQTT_TOPIC_PREFIX}" ]]; then
	if truthy "${DRY_RUN}"; then
		MQTT_TOPIC_PREFIX="avf/v1"
		warn "MQTT_TOPIC_PREFIX unset; using example ${MQTT_TOPIC_PREFIX} for dry-run"
	else
		fail "MQTT_TOPIC_PREFIX is required"
	fi
fi

if ! truthy "${DRY_RUN}"; then
	[[ -n "${MQTT_BROKER_URL}" || -n "${MQTT_HOST}" ]] || fail "MQTT_BROKER_URL or MQTT_HOST required for execute"
	command -v mosquitto_pub >/dev/null 2>&1 || fail "mosquitto_pub not found; install mosquitto clients for EXECUTE"
	parse_mqtt_broker_url
fi

print_scrape_commands

TOTAL_MQTT_PUBLISHES=$((TOTAL_EVENTS + CRITICAL_DUP_PLANNED))
export MACHINE_COUNT EVENTS_PER_MACHINE TOTAL_EVENTS TOTAL_MQTT_PUBLISHES CRITICAL_EVENT_RATIO CRITICAL_PCT
export CRITICAL_EXPECTED_UNIQUE CRITICAL_DUP_PLANNED CRITICAL_PUBLISH_PLANNED
export CRITICAL_DUPLICATE_REPLAY_PERCENT SCENARIO_PRESET LOAD_TEST_ENV DRY_RUN EXECUTE_LOAD_TEST
export RESULT_JSON_FILE

if truthy "${DRY_RUN}"; then
	MOSQUITTO_PUBLISH_FAILS=0
	export MOSQUITTO_PUBLISH_FAILS
	note "dry-run: sample wire (first machine, wave 1)"
	idx=1
	wave=1
	mid="$(machine_uuid "${idx}")"
	boot="$(boot_uuid "${idx}")"
	et="$(event_type_for_wave "${idx}" "${wave}")"
	occurred="$(occurred_at_iso "${idx}" "${wave}")"
	payload="$(envelope_json "${mid}" "${boot}" "${wave}" "${idx}" "${et}" "${occurred}")"
	topic="${MQTT_TOPIC_PREFIX}/${mid}/telemetry"
	echo "  topic: ${topic}"
	echo "  payload: ${payload}"
	ACCOUNTING_STATUS="dry_run_no_execute"
	HEALTH_RESULT="skipped"
	WORKER_HEALTH_RESULT="skipped"
	DB_POOL_RESULT="skipped"
	export DB_POOL_RESULT
	STRICT_FAIL_REASON=""
	CRITICAL_ACCEPTED_DELTA=""
	CRITICAL_LOST=""
	DISPATCH_OK_DELTA=""
	IDEMPOTENCY_CONFLICT_DELTA=""
	DUPLICATE_PROJ_DELTA=""
	MAX_LAG=""
	export ACCOUNTING_STATUS HEALTH_RESULT WORKER_HEALTH_RESULT DB_POOL_RESULT STRICT_FAIL_REASON
	export CRITICAL_ACCEPTED_DELTA CRITICAL_LOST DISPATCH_OK_DELTA IDEMPOTENCY_CONFLICT_DELTA DUPLICATE_PROJ_DELTA MAX_LAG
	_payload="$(build_result_payload)"
	write_result_json "false" "${_payload}"
	note "dry-run complete — overall_pass=false (no execute certification). See ${RESULT_JSON_FILE}"
	exit 0
fi

before_m="$(mktemp)"
before_w="$(mktemp)"
after_m="$(mktemp)"
after_w="$(mktemp)"
FAILURES_FILE="$(mktemp)"
cleanup() { rm -f "${before_m}" "${before_w}" "${after_m}" "${after_w}" "${FAILURES_FILE}"; }
trap cleanup EXIT
: >"${FAILURES_FILE}"

ACCOUNTING_STATUS="pending"
CRITICAL_ACCEPTED_DELTA=""
CRITICAL_LOST=""
DISPATCH_OK_DELTA=""
IDEMPOTENCY_CONFLICT_DELTA=""
DUPLICATE_PROJ_DELTA=""
MAX_LAG=""
HEALTH_RESULT="skipped"
WORKER_HEALTH_RESULT="skipped"
STRICT_FAIL_REASON=""
GATE_FAIL_REASON=""
DB_POOL_RESULT="unknown"
export GATE_FAIL_REASON DB_POOL_RESULT

br_mqtt=0
br_worker=0
LOGS_SINCE="2m"
METRICS_BEFORE_OK=0
METRICS_AFTER_OK=0

if truthy "${RUN_POST_GATES}"; then
	if fetch_metrics_to_optional "${before_m}" mqtt && fetch_metrics_to_optional "${before_w}" worker; then
		METRICS_BEFORE_OK=1
	fi
	if ! truthy "${SKIP_DOCKER_GATES}"; then
		br_mqtt="$(compose_restart_count mqtt-ingest)"
		br_worker="$(compose_restart_count worker)"
		LOGS_SINCE="$(date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -Iseconds)"
	fi
fi

readiness_pids=()
READINESS_FAIL_FILES=()
if [[ -n "${API_READY_URL}" ]]; then
	f="$(mktemp)"
	READINESS_FAIL_FILES+=("${f}")
	readiness_monitor_loop "${API_READY_URL}" "${READINESS_MAX_FAIL_SECONDS}" "${READINESS_POLL_INTERVAL}" "${f}" "API" &
	readiness_pids+=("$!")
fi
if [[ -n "${WORKER_READY_URL}" ]]; then
	f="$(mktemp)"
	READINESS_FAIL_FILES+=("${f}")
	readiness_monitor_loop "${WORKER_READY_URL}" "${READINESS_MAX_FAIL_SECONDS}" "${READINESS_POLL_INTERVAL}" "${f}" "worker" &
	readiness_pids+=("$!")
fi

note "publishing ${TOTAL_EVENTS} logical messages (+ planned duplicate replays); ~${SLEEP_WAVE}s between waves"
published=0
for ((wave = 1; wave <= EVENTS_PER_MACHINE; wave++)); do
	for ((idx = 1; idx <= MACHINE_COUNT; idx++)); do
		publish_wave_message "${idx}" "${wave}" &
		if ((idx % WAVE_PARALLEL_LIMIT == 0)); then
			wait
		fi
		((published++)) || true
	done
	wait
	for f in "${READINESS_FAIL_FILES[@]:-}"; do
		if [[ -n "${f}" && -s "${f}" ]]; then
			for pid in "${readiness_pids[@]:-}"; do
				kill "${pid}" 2>/dev/null || true
			done
			for _p in "${readiness_pids[@]:-}"; do
				wait "${_p}" 2>/dev/null || true
			done
			for _rf in "${READINESS_FAIL_FILES[@]:-}"; do
				[[ -n "${_rf}" ]] && rm -f "${_rf}"
			done
			MOSQUITTO_PUBLISH_FAILS="$(wc -l <"${FAILURES_FILE}" | tr -d ' ')"
			export MOSQUITTO_PUBLISH_FAILS
			ACCOUNTING_STATUS="readiness_failed"
			STRICT_FAIL_REASON="readiness threshold exceeded during load (/health/ready)"
			HEALTH_RESULT="fail"
			WORKER_HEALTH_RESULT="${WORKER_HEALTH_RESULT:-skipped}"
			DB_POOL_RESULT="unknown"
			GATE_FAIL_REASON=""
			export ACCOUNTING_STATUS HEALTH_RESULT WORKER_HEALTH_RESULT DB_POOL_RESULT STRICT_FAIL_REASON GATE_FAIL_REASON
			_payload="$(build_result_payload)"
			write_result_json "false" "${_payload}"
			fail "readiness threshold exceeded during load (see ${RESULT_JSON_FILE})"
		fi
	done
	sleep "${SLEEP_WAVE}"
done

for pid in "${readiness_pids[@]:-}"; do
	kill "${pid}" 2>/dev/null || true
	wait "${pid}" 2>/dev/null || true
done
for f in "${READINESS_FAIL_FILES[@]:-}"; do
	[[ -n "${f}" ]] && rm -f "${f}"
done

MOSQUITTO_PUBLISH_FAILS="$(wc -l <"${FAILURES_FILE}" | tr -d ' ')"
export MOSQUITTO_PUBLISH_FAILS
if ((MOSQUITTO_PUBLISH_FAILS > 0)); then
	ACCOUNTING_STATUS="publish_failures"
	STRICT_FAIL_REASON="mosquitto_pub_failures=${MOSQUITTO_PUBLISH_FAILS}"
	_payload="$(build_result_payload)"
	write_result_json "false" "${_payload}"
	fail "mosquitto_pub failures=${MOSQUITTO_PUBLISH_FAILS} (see ${RESULT_JSON_FILE})"
fi

OVERALL_PASS=true

if truthy "${RUN_POST_GATES}"; then
	sleep "${STABILIZE_SECONDS}"
	if ! truthy "${SKIP_DOCKER_GATES}"; then
		if fetch_metrics_to_optional "${after_m}" mqtt && fetch_metrics_to_optional "${after_w}" worker; then
			METRICS_AFTER_OK=1
		fi
		ar_mqtt="$(compose_restart_count mqtt-ingest)"
		ar_worker="$(compose_restart_count worker)"
		GATE_FAIL_REASON=""
		if ! post_run_gates "${before_m}" "${before_w}" "${after_m}" "${after_w}" "${br_mqtt}" "${br_worker}" "${ar_mqtt}" "${ar_worker}"; then
			append_strict_fail_reason "${GATE_FAIL_REASON:-post_run_gates_failed}"
			OVERALL_PASS=false
		fi
	else
		warn "SKIP_DOCKER_GATES=true — skipping compose log/restart/non-droppable gates"
		ar_mqtt="${br_mqtt}"
		ar_worker="${br_worker}"
		DB_POOL_RESULT="skipped"
		if fetch_metrics_to_optional "${after_m}" mqtt && fetch_metrics_to_optional "${after_w}" worker; then
			METRICS_AFTER_OK=1
		fi
	fi

	if [[ -n "${API_READY_URL}" ]]; then
		if curl -fsS "${API_READY_URL}" | grep -qx ok; then
			HEALTH_RESULT="ok"
		else
			HEALTH_RESULT="fail"
			append_strict_fail_reason "API_READY_URL not ok after load (${API_READY_URL})"
			OVERALL_PASS=false
		fi
	fi
	if [[ -n "${WORKER_READY_URL}" ]]; then
		if curl -fsS "${WORKER_READY_URL}" | grep -qx ok; then
			WORKER_HEALTH_RESULT="ok"
		else
			WORKER_HEALTH_RESULT="fail"
			append_strict_fail_reason "WORKER_READY_URL not ok after load (${WORKER_READY_URL})"
			OVERALL_PASS=false
		fi
	fi

	# --- accounting ---
	if ((METRICS_BEFORE_OK == 1 && METRICS_AFTER_OK == 1)); then
		if ! grep -q 'avf_telemetry_ingest_received_total' "${after_m}" || ! grep -qE 'avf_telemetry_consumer_lag|avf_telemetry_projection_' "${after_w}"; then
			METRICS_AFTER_OK=0
		fi
	fi

	if ((METRICS_BEFORE_OK == 1 && METRICS_AFTER_OK == 1)); then
		DELTA_JSON="$(python3 "${ACCOUNTING_PY}" delta --before "${before_m}" --after "${after_m}")"
		WDELTA_JSON="$(python3 "${ACCOUNTING_PY}" delta --before "${before_w}" --after "${after_w}")"
		CRITICAL_ACCEPTED_DELTA="$(printf '%s' "${DELTA_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["ingest_received_critical_no_drop_delta"])')"
		DISPATCH_OK_DELTA="$(printf '%s' "${DELTA_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["mqtt_dispatch_telemetry_ok_delta"])')"
		IDEMPOTENCY_CONFLICT_DELTA="$(printf '%s' "${WDELTA_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["telemetry_idempotency_conflict_delta"])')"
		MAX_LAG="$(printf '%s' "${WDELTA_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["max_consumer_lag_after"])')"
		DUP_EDGE="$(printf '%s' "${WDELTA_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["telemetry_duplicate_edge_delta"])')"
		DUP_INV="$(printf '%s' "${WDELTA_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["telemetry_duplicate_inventory_delta"])')"
		DUP_RU="$(printf '%s' "${WDELTA_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["telemetry_duplicate_critical_rollup_delta"])')"
		DUP_REPLAY="$(printf '%s' "${WDELTA_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["telemetry_duplicate_idem_replay_delta"])')"
		DUPLICATE_PROJ_DELTA="$(python3 -c "print(float(${DUP_EDGE})+float(${DUP_INV})+float(${DUP_RU})+float(${DUP_REPLAY}))")"

		# shellcheck disable=SC2086
		CRITICAL_LOST="$(python3 -c "import math; planned=${CRITICAL_PUBLISH_PLANNED}; got=float('${CRITICAL_ACCEPTED_DELTA}'); print(max(0.0, planned - got))")"

		ACCOUNTING_STATUS="metrics_delta_ok"

		if awk -v l="${MAX_LAG_FAIL_THRESHOLD}" -v m="${MAX_LAG}" 'BEGIN {exit !(l>0 && m>l)}'; then
			append_strict_fail_reason "max_lag ${MAX_LAG} > threshold ${MAX_LAG_FAIL_THRESHOLD}"
			OVERALL_PASS=false
		fi

		if awk -v c="${IDEMPOTENCY_CONFLICT_DELTA}" 'BEGIN {exit !(c>0)}'; then
			append_strict_fail_reason "idempotency_conflict_delta=${IDEMPOTENCY_CONFLICT_DELTA} (duplicate business-effect risk — same key, different payload)"
			OVERALL_PASS=false
		fi

		if awk -v lost="${CRITICAL_LOST}" 'BEGIN {exit !(lost>0)}'; then
			append_strict_fail_reason "critical_lost=${CRITICAL_LOST} (ingest critical_no_drop delta ${CRITICAL_ACCEPTED_DELTA} < planned ${CRITICAL_PUBLISH_PLANNED})"
			OVERALL_PASS=false
		fi

		if awk -v d="${DISPATCH_OK_DELTA}" -v t="${TOTAL_MQTT_PUBLISHES}" 'BEGIN {exit !(d+0.5 < t)}'; then
			append_strict_fail_reason "dispatch_telemetry_ok_delta=${DISPATCH_OK_DELTA} < mqtt_publishes_planned=${TOTAL_MQTT_PUBLISHES}"
			OVERALL_PASS=false
		fi
	else
		ACCOUNTING_STATUS="metrics_unavailable"
		if truthy "${STRICT_ACCOUNTING}"; then
			append_strict_fail_reason "Prometheus metrics not available before/after (need METRICS_ENABLED=true on mqtt-ingest+worker or MQTT_INGEST_METRICS_URL and WORKER_METRICS_URL; compose scrape needs docker unless URLs set)"
			OVERALL_PASS=false
		fi
	fi
else
	ACCOUNTING_STATUS="post_gates_disabled"
	if truthy "${STRICT_ACCOUNTING}"; then
		append_strict_fail_reason "RUN_POST_GATES=false — cannot certify"
		OVERALL_PASS=false
	fi
fi

export ACCOUNTING_STATUS CRITICAL_ACCEPTED_DELTA CRITICAL_LOST DISPATCH_OK_DELTA
export IDEMPOTENCY_CONFLICT_DELTA DUPLICATE_PROJ_DELTA MAX_LAG
export HEALTH_RESULT WORKER_HEALTH_RESULT DB_POOL_RESULT STRICT_FAIL_REASON GATE_FAIL_REASON

_payload="$(build_result_payload)"
if truthy "${OVERALL_PASS}"; then
	write_result_json "true" "${_payload}"
	note "PASS — ${RESULT_JSON_FILE}"
	exit 0
else
	write_result_json "false" "${_payload}"
	fail "FAIL — ${STRICT_FAIL_REASON:-see ${RESULT_JSON_FILE}}"
fi
