#!/usr/bin/env bash
# Validate production monitoring: ops /metrics expose expected Prometheus series and /health/ready responds.
# Does not print credentials. Fails on unreachable required endpoints or missing required metric names.
#
# Required env:
#   API_METRICS_URL          e.g. http://127.0.0.1:8081/metrics
#   MQTT_INGEST_METRICS_URL
#   WORKER_METRICS_URL
#
# Optional env (if unset, derived from the matching *_METRICS_URL by replacing trailing /metrics with /health/ready):
#   API_HEALTH_READY_URL
#   MQTT_INGEST_HEALTH_READY_URL
#   WORKER_HEALTH_READY_URL
#   RECONCILER_HEALTH_READY_URL   — if set, GET must return 2xx
#
# Optional observability targets (recorded under optional_endpoint_failures; do not fail the run):
#   PROMETHEUS_URL
#   ALERTMANAGER_URL
#
# Optional:
#   MONITORING_READINESS_RESULT_FILE  — default monitoring-readiness-result.json
#   CURL_EXTRA_ARGS                   — extra curl args (single line, e.g. "-k")
#
# Usage:
#   export API_METRICS_URL=... MQTT_INGEST_METRICS_URL=... WORKER_METRICS_URL=...
#   bash deployments/prod/scripts/check_monitoring_readiness.sh
#
set -euo pipefail

RESULT_FILE="${MONITORING_READINESS_RESULT_FILE:-monitoring-readiness-result.json}"
MISSING_FILE="$(mktemp)"
UNREACH_FILE="$(mktemp)"
OPT_FAIL_FILE="$(mktemp)"
WARN_FILE="$(mktemp)"
METRICS_TMP="$(mktemp)"
cleanup() {
	rm -f "${MISSING_FILE}" "${UNREACH_FILE}" "${OPT_FAIL_FILE}" "${WARN_FILE}" "${METRICS_TMP}"
}
trap cleanup EXIT

: "${API_METRICS_URL:?set API_METRICS_URL}"
: "${MQTT_INGEST_METRICS_URL:?set MQTT_INGEST_METRICS_URL}"
: "${WORKER_METRICS_URL:?set WORKER_METRICS_URL}"

CURL_ARR=()
if [[ -n "${CURL_EXTRA_ARGS:-}" ]]; then
	read -r -a CURL_ARR <<< "${CURL_EXTRA_ARGS}"
fi

note() {
	echo "check_monitoring_readiness: $*" >&2
}

curl_get() {
	local url="$1"
	local out="$2"
	curl -fsS --max-time 25 "${CURL_ARR[@]}" -o "${out}" "${url}" || return 1
	return 0
}

curl_code() {
	local url="$1"
	local code
	code="$(curl -sS --max-time 15 "${CURL_ARR[@]}" -o /dev/null -w '%{http_code}' "${url}")" || { echo "000"; return 0; }
	printf '%s' "${code}"
}

health_url_or_derive() {
	local explicit="$1"
	local metrics_url="$2"
	if [[ -n "${explicit}" ]]; then
		printf '%s' "${explicit}"
		return
	fi
	if [[ "${metrics_url}" == *"/metrics" ]]; then
		printf '%s' "${metrics_url%/metrics}/health/ready"
		return
	fi
	printf ''
}

metric_line_present() {
	local file="$1"
	local pattern="$2"
	grep -qE "^${pattern}" "${file}" 2>/dev/null
}

append_missing() {
	printf '%s\n' "$1" >> "${MISSING_FILE}"
}

append_unreachable() {
	printf '%s\n' "$1" >> "${UNREACH_FILE}"
}

append_opt_fail() {
	printf '%s\n' "$1" >> "${OPT_FAIL_FILE}"
}

append_warn() {
	printf '%s\n' "$1" >> "${WARN_FILE}"
}

check_health_endpoint() {
	local key="$1"
	local url="$2"
	if [[ -z "${url}" ]]; then
		append_unreachable "${key} (health URL not set and could not be derived from metrics URL — set *_HEALTH_READY_URL or use .../metrics URLs)"
		return 1
	fi
	local code
	code="$(curl_code "${url}")"
	if [[ "${code}" != "2"* ]]; then
		append_unreachable "${key} (HTTP ${code})"
		return 1
	fi
	return 0
}

check_metrics_endpoint() {
	local key="$1"
	local url="$2"
	shift 2
	local -a patterns=("$@")
	if ! curl_get "${url}" "${METRICS_TMP}"; then
		append_unreachable "${key} (metrics GET failed)"
		return 1
	fi
	local p
	for p in "${patterns[@]}"; do
		if ! metric_line_present "${METRICS_TMP}" "${p}"; then
			append_missing "${key}: ${p}"
		fi
	done
	return 0
}

API_H="${API_HEALTH_READY_URL:-}"
API_H="$(health_url_or_derive "${API_H}" "${API_METRICS_URL}")"
MQTT_H="${MQTT_INGEST_HEALTH_READY_URL:-}"
MQTT_H="$(health_url_or_derive "${MQTT_H}" "${MQTT_INGEST_METRICS_URL}")"
WORKER_H="${WORKER_HEALTH_READY_URL:-}"
WORKER_H="$(health_url_or_derive "${WORKER_H}" "${WORKER_METRICS_URL}")"

note "checking health endpoints"
# Intentionally use || true so every endpoint is probed; failures accumulate in unreachable_endpoints and fail the run.
check_health_endpoint "api_health_ready" "${API_H}" || true
check_health_endpoint "mqtt_ingest_health_ready" "${MQTT_H}" || true
check_health_endpoint "worker_health_ready" "${WORKER_H}" || true
if [[ -n "${RECONCILER_HEALTH_READY_URL:-}" ]]; then
	check_health_endpoint "reconciler_health_ready" "${RECONCILER_HEALTH_READY_URL}" || true
fi

note "checking API /metrics (Prometheus client runtime metrics; AVF telemetry counters are not registered on api)"
# Same pattern: probe all metric groups; missing lines append to missing_metrics and fail the run.
check_metrics_endpoint "api_metrics" "${API_METRICS_URL}" \
	'process_start_time_seconds(\s|$)' \
	|| true

note "checking mqtt-ingest /metrics (canonical names: avf_telemetry_ingest_*, avf_mqtt_ingest_dispatch_total)"
check_metrics_endpoint "mqtt_ingest_metrics" "${MQTT_INGEST_METRICS_URL}" \
	'avf_telemetry_ingest_received_total(\{|\s)' \
	'avf_telemetry_ingest_rejected_total(\{|\s)' \
	'avf_telemetry_ingest_dropped_total(\{|\s)' \
	'avf_telemetry_ingest_queue_depth(\{|\s)' \
	'avf_mqtt_ingest_dispatch_total(\{|\s)' \
	|| true

note "checking worker /metrics"
check_metrics_endpoint "worker_metrics" "${WORKER_METRICS_URL}" \
	'avf_telemetry_consumer_lag(\{|\s)' \
	'avf_telemetry_projection_failures_total(\{|\s)' \
	'avf_telemetry_idempotency_conflict_total(\{|\s)' \
	|| true

if [[ -n "${PROMETHEUS_URL:-}" ]]; then
	note "optional: Prometheus UI readiness"
	pbase="${PROMETHEUS_URL%/}"
	ok=0
	for path in "/-/ready" "/-/healthy"; do
		code="$(curl_code "${pbase}${path}")"
		if [[ "${code}" == "2"* ]]; then
			ok=1
			append_warn "prometheus_reachable path=${path} http=${code}"
			break
		fi
	done
	if [[ "${ok}" -eq 0 ]]; then
		append_opt_fail "prometheus (${pbase}/-/ready and /-/healthy not 2xx)"
	fi
fi

if [[ -n "${ALERTMANAGER_URL:-}" ]]; then
	note "optional: Alertmanager readiness"
	abase="${ALERTMANAGER_URL%/}"
	ok=0
	for path in "/-/ready" "/-/healthy"; do
		code="$(curl_code "${abase}${path}")"
		if [[ "${code}" == "2"* ]]; then
			ok=1
			append_warn "alertmanager_reachable path=${path} http=${code}"
			break
		fi
	done
	if [[ "${ok}" -eq 0 ]]; then
		append_opt_fail "alertmanager (/-/ready and /-/healthy not 2xx)"
	fi
fi

python3 - "${RESULT_FILE}" "${MISSING_FILE}" "${UNREACH_FILE}" "${OPT_FAIL_FILE}" "${WARN_FILE}" <<'PY'
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

result_path, miss_path, unreach_path, opt_path, warn_path = sys.argv[1:6]

def lines(p):
    path = Path(p)
    if not path.exists():
        return []
    return [ln.strip() for ln in path.read_text(encoding="utf-8").splitlines() if ln.strip()]

missing = lines(miss_path)
unreachable = lines(unreach_path)
opt_fail = lines(opt_path)
warnings = lines(warn_path)

final = "pass" if not missing and not unreachable else "fail"

doc = {
    "schema_version": 1,
    "completed_at_utc": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "final_result": final,
    "missing_metrics": missing,
    "unreachable_endpoints": unreachable,
    "optional_endpoint_failures": opt_fail,
    "warnings": warnings,
    "deferred_metric_groups": [
        {
            "name": "postgres_pool_saturation",
            "status": "not_checked",
            "reason": "Not emitted by the application; see docs/runbooks/production-observability-alerts.md (TODO metrics).",
        },
        {
            "name": "container_restart_oom",
            "status": "not_checked",
            "reason": "Not emitted by the application; use cadvisor/kube/docker events per runbook.",
        },
    ],
    "metric_name_reference": {
        "mqtt_ingest": "internal/app/telemetryapp/mqtt_ingest_prom.go (namespace avf)",
        "worker": "internal/app/telemetryapp/telemetry_worker_prom.go (namespace avf)",
    },
}

Path(result_path).write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
print(json.dumps({"final_result": final, "result_file": str(result_path)}))
PY

final_json="$(
	RESULT_FILE="${RESULT_FILE}" python3 -c "import json, os; print(json.load(open(os.environ['RESULT_FILE'], encoding='utf-8'))['final_result'])"
)"
note "wrote ${RESULT_FILE} final_result=${final_json}"
if [[ "${final_json}" != "pass" ]]; then
	exit 1
fi
exit 0
