#!/usr/bin/env bash
# Staging-only telemetry storm evidence suite: runs 100x100, 500x200, 1000x500 sequentially.
# Stops on first failure. Writes per-scenario JSON + telemetry-storm-suite-result.json under STORM_SUITE_ARTIFACT_DIR.
#
# Execute mode (real load + certification): requires staging env, broker creds, metrics URLs, API readiness URL.
# Plan-only mode (--plan-only): dry-runs all three scenarios; no MQTT credentials or metrics required.
#
# Never targets production: LOAD_TEST_ENV=production is refused. No production URLs are defaulted.
#
# STORM_SCENARIO_MODE: all | 100x100 | 500x200 | 1000x500 (default all). Partial runs still write suite summary
# for selected scenarios only; artifact filenames remain telemetry-storm-result-<preset>.json.
#
# shellcheck disable=SC1090,SC1091
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
STORM_SCRIPT="${SCRIPT_DIR}/telemetry_storm_load_test.sh"

die() { echo "run_staging_telemetry_storm_suite: error: $*" >&2; exit 1; }
note() { echo "==> $*"; }

truthy() {
	case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')" in
	true | 1 | yes) return 0 ;;
	*) return 1 ;;
	esac
}

is_staging_env() {
	[[ "$(printf '%s' "${LOAD_TEST_ENV:-}" | tr '[:upper:]' '[:lower:]')" == "staging" ]]
}

redact_broker_log_line() {
	# Log presence only; never print userinfo from URL or passwords.
	if [[ -n "${MQTT_BROKER_URL:-}" ]]; then
		echo "  MQTT_BROKER_URL: set (redacted)"
	else
		echo "  MQTT_BROKER_URL: (unset — using MQTT_HOST/MQTT_PORT if configured)"
	fi
	if [[ -n "${MQTT_USERNAME:-}" ]]; then
		echo "  MQTT_USERNAME: set (redacted)"
	else
		echo "  MQTT_USERNAME: unset"
	fi
	echo "  MQTT_PASSWORD: $([[ -n "${MQTT_PASSWORD:-}" ]] && echo 'set (redacted)' || echo unset)"
}

require_mqtt_credentials() {
	if [[ -n "${MQTT_USERNAME:-}" && -n "${MQTT_PASSWORD:-}" ]]; then
		return 0
	fi
	python3 - <<'PY' || die "MQTT_USERNAME and MQTT_PASSWORD required unless userinfo is embedded in MQTT_BROKER_URL"
import os, urllib.parse, sys
url = (os.environ.get("MQTT_BROKER_URL") or "").strip()
if not url:
    sys.exit(1)
u = urllib.parse.urlsplit(url)
if u.username or (u.password is not None and str(u.password) != ""):
    sys.exit(0)
sys.exit(1)
PY
}

apply_metrics_urls() {
	if [[ -n "${MQTT_INGEST_METRICS_URL:-}" && -n "${WORKER_METRICS_URL:-}" ]]; then
		export MQTT_INGEST_METRICS_URL WORKER_METRICS_URL
		return 0
	fi
	[[ -n "${METRICS_BASE_URL:-}" ]] || die "set MQTT_INGEST_METRICS_URL and WORKER_METRICS_URL, or METRICS_BASE_URL (e.g. http://staging-ops.internal with METRICS_MQTT_INGEST_PORT / METRICS_WORKER_PORT)"
	local base mport wport
	base="${METRICS_BASE_URL%/}"
	mport="${METRICS_MQTT_INGEST_PORT:-9093}"
	wport="${METRICS_WORKER_PORT:-9091}"
	export MQTT_INGEST_METRICS_URL="${base}:${mport}/metrics"
	export WORKER_METRICS_URL="${base}:${wport}/metrics"
	note "derived metrics URLs from METRICS_BASE_URL (ports ${mport} / ${wport})"
}

validate_execute_prereqs() {
	is_staging_env || die "LOAD_TEST_ENV must be staging (got ${LOAD_TEST_ENV:-empty})"
	truthy "${EXECUTE_LOAD_TEST}" || die "EXECUTE_LOAD_TEST must be true"
	[[ "$(printf '%s' "${DRY_RUN:-}" | tr '[:upper:]' '[:lower:]')" == "false" ]] ||
		die "DRY_RUN must be false for staging execute suite (got ${DRY_RUN:-empty})"
	[[ -n "${MACHINE_IDS_FILE:-}" ]] || die "MACHINE_IDS_FILE is required"
	[[ -f "${MACHINE_IDS_FILE}" ]] || die "MACHINE_IDS_FILE not found: ${MACHINE_IDS_FILE}"
	[[ -n "${MQTT_BROKER_URL:-}" || -n "${MQTT_HOST:-}" ]] || die "MQTT_BROKER_URL or MQTT_HOST is required"
	[[ -n "${API_READY_URL:-}" ]] || die "API_READY_URL is required (e.g. https://staging-api.example/health/ready)"
	[[ -n "${MQTT_TOPIC_PREFIX:-}" ]] || die "MQTT_TOPIC_PREFIX is required for execute mode"
	require_mqtt_credentials
	apply_metrics_urls
	if [[ -n "${LOAD_TEST_ENV:-}" ]] && [[ "$(printf '%s' "${LOAD_TEST_ENV}" | tr '[:upper:]' '[:lower:]')" == "production" ]]; then
		die "LOAD_TEST_ENV=production is not allowed for this suite"
	fi
}

refuse_production() {
	if [[ -n "${LOAD_TEST_ENV:-}" ]] && [[ "$(printf '%s' "${LOAD_TEST_ENV}" | tr '[:upper:]' '[:lower:]')" == "production" ]]; then
		die "LOAD_TEST_ENV=production refused by run_staging_telemetry_storm_suite.sh"
	fi
}

# Refuse URLs that look like documented production / prod-only hosts (extend via STORM_URL_EXTRA_DENY_REGEX).
validate_no_production_shaped_urls() {
	if truthy "${STORM_ALLOW_PRODUCTION_SHAPED_URL:-}"; then
		return 0
	fi
	python3 - <<'PY'
import os, re, sys

def deny(msg: str) -> None:
    print(f"run_staging_telemetry_storm_suite: refused URL: {msg}", file=sys.stderr)
    sys.exit(1)

extra = (os.environ.get("STORM_URL_EXTRA_DENY_REGEX") or "").strip()
patterns = [
    r"api\.ldtv\.dev",
    r"\bproduction[\w.-]*\.",
    r"[\w.-]+\.prod\.",
    r"://prod\.",
]
if extra:
    try:
        patterns.append(extra)
    except re.error as e:
        deny(f"invalid STORM_URL_EXTRA_DENY_REGEX: {e}")

def check(label: str, val: str) -> None:
    if not val.strip():
        return
    low = val.lower()
    for p in patterns:
        try:
            if re.search(p, low):
                deny(f"{label} matches forbidden pattern (staging-only suite)")
        except re.error:
            deny(f"internal regex error for pattern {p!r}")

check("MQTT_BROKER_URL", os.environ.get("MQTT_BROKER_URL") or "")
check("MQTT_HOST", os.environ.get("MQTT_HOST") or "")
check("API_READY_URL", os.environ.get("API_READY_URL") or "")
check("WORKER_READY_URL", os.environ.get("WORKER_READY_URL") or "")
check("METRICS_BASE_URL", os.environ.get("METRICS_BASE_URL") or "")
check("MQTT_INGEST_METRICS_URL", os.environ.get("MQTT_INGEST_METRICS_URL") or "")
check("WORKER_METRICS_URL", os.environ.get("WORKER_METRICS_URL") or "")
PY
}

select_scenarios() {
	local mode
	mode="$(printf '%s' "${STORM_SCENARIO_MODE:-all}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
	case "${mode}" in
	all)
		SCENARIOS=(100x100 500x200 1000x500)
		SCENARIO_LIST_JOINED="100x100 500x200 1000x500"
		;;
	100x100)
		SCENARIOS=(100x100)
		SCENARIO_LIST_JOINED="100x100"
		;;
	500x200)
		SCENARIOS=(500x200)
		SCENARIO_LIST_JOINED="500x200"
		;;
	1000x500)
		SCENARIOS=(1000x500)
		SCENARIO_LIST_JOINED="1000x500"
		;;
	*) die "STORM_SCENARIO_MODE must be all, 100x100, 500x200, or 1000x500 (got ${STORM_SCENARIO_MODE:-empty})" ;;
	esac
	export STORM_SCENARIO_MODE="${mode}"
}

usage() {
	cat <<'EOF'
Usage: run_staging_telemetry_storm_suite.sh [--plan-only]

Execute mode (sequential staging storm evidence):
  Required env:
    LOAD_TEST_ENV=staging
    EXECUTE_LOAD_TEST=true
    DRY_RUN=false
    MACHINE_IDS_FILE
    MQTT_BROKER_URL (or MQTT_HOST + MQTT_PORT)
    MQTT_USERNAME + MQTT_PASSWORD (unless credentials embedded in MQTT_BROKER_URL)
    MQTT_TOPIC_PREFIX
    API_READY_URL
    MQTT_INGEST_METRICS_URL + WORKER_METRICS_URL
    OR METRICS_BASE_URL (optional METRICS_MQTT_INGEST_PORT=9093, METRICS_WORKER_PORT=9091)

  Optional: WORKER_READY_URL, STORM_SCENARIO_MODE (all|100x100|500x200|1000x500), EVENT_RATE_PER_MACHINE,
    CRITICAL_EVENT_RATIO, and all other telemetry_storm_load_test.sh tunables.

  Artifacts: STORM_SUITE_ARTIFACT_DIR (default: <repo>/telemetry-storm-suite-artifacts/<UTC timestamp>)

Plan-only (no secrets / no MQTT):
  run_staging_telemetry_storm_suite.sh --plan-only
  Optional: STORM_SUITE_ARTIFACT_DIR, MACHINE_IDS_FILE (export if you want dry-run to reference real UUIDs)
EOF
}

write_suite_summary() {
	python3 - <<'PY'
import json, os, time
from pathlib import Path

artifact_dir = os.environ["SUITE_ARTIFACT_DIR"]
out_path = Path(artifact_dir) / "telemetry-storm-suite-result.json"
suite_pass = os.environ.get("SUITE_PASS", "false") == "true"
failed_at = (os.environ.get("FAILED_AT") or "").strip() or None
mode = os.environ["SUITE_MODE"]
scenarios = os.environ["SCENARIO_LIST"].split()

rows = []
for scen in scenarios:
    rel = f"telemetry-storm-result-{scen}.json"
    p = Path(artifact_dir) / rel
    row = {
        "scenario": scen,
        "artifact": rel,
        "final_result": None,
        "critical_lost": None,
        "duplicate_critical_effects": None,
        "db_pool_result": None,
        "health_result": None,
        "restart_result": None,
    }
    if p.is_file():
        try:
            doc = json.loads(p.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            doc = {}
        fr = doc.get("final_result")
        if fr is None and doc.get("overall_pass") is not None:
            fr = "pass" if doc.get("overall_pass") else "fail"
        row["final_result"] = fr
        row["critical_lost"] = doc.get("critical_lost")
        row["duplicate_critical_effects"] = doc.get("duplicate_critical_effects")
        row["db_pool_result"] = doc.get("db_pool_result")
        row["health_result"] = doc.get("health_result")
        row["restart_result"] = doc.get("restart_result")
    rows.append(row)

doc = {
    "completed_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "load_test_env": os.environ.get("LOAD_TEST_ENV", "staging"),
    "suite_mode": mode,
    "storm_scenario_mode": os.environ.get("STORM_SCENARIO_MODE", "all"),
    "artifact_dir": str(Path(artifact_dir).resolve()),
    "final_suite_pass": suite_pass,
    "failed_at_scenario": failed_at,
    "scenarios": rows,
}
out_path.write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
print("run_staging_telemetry_storm_suite: wrote", out_path)
PY
}

PLAN_ONLY=false
if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
	usage
	exit 0
fi
if [[ "${1:-}" == "--plan-only" ]]; then
	PLAN_ONLY=true
fi

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACT_DIR="${STORM_SUITE_ARTIFACT_DIR:-${REPO_ROOT}/telemetry-storm-suite-artifacts/${TIMESTAMP}}"
mkdir -p "${ARTIFACT_DIR}"
export STORM_SUITE_ARTIFACT_DIR="${ARTIFACT_DIR}"

select_scenarios
SUITE_PASS=true
FAILED_AT=""

if [[ "${PLAN_ONLY}" == "true" ]]; then
	refuse_production
	note "plan-only: dry-run scenarios: ${SCENARIO_LIST_JOINED} (no MQTT execute, no credentials required)"
	export LOAD_TEST_ENV="${LOAD_TEST_ENV:-staging}"
	is_staging_env || die "LOAD_TEST_ENV must be staging for plan-only (got ${LOAD_TEST_ENV})"
	[[ -z "${MACHINE_IDS_FILE:-}" ]] || export MACHINE_IDS_FILE
	for scen in "${SCENARIOS[@]}"; do
		note "plan-only scenario ${scen}"
		SCENARIO_PRESET="${scen}" \
			LOAD_TEST_ENV=staging \
			DRY_RUN=true \
			EXECUTE_LOAD_TEST=false \
			RESULT_JSON_FILE="${ARTIFACT_DIR}/telemetry-storm-result-${scen}.json" \
			bash "${STORM_SCRIPT}"
	done
	SUITE_PASS=false
	FAILED_AT=""
	export SUITE_ARTIFACT_DIR="${ARTIFACT_DIR}"
	export SUITE_PASS=false
	export FAILED_AT=""
	export SUITE_MODE="plan_only"
	export SCENARIO_LIST="${SCENARIO_LIST_JOINED}"
	export LOAD_TEST_ENV
	export STORM_SCENARIO_MODE
	write_suite_summary
	note "plan-only complete — artifacts under ${ARTIFACT_DIR} (per-scenario overall_pass is false by design for dry-run)"
	exit 0
fi

validate_execute_prereqs
refuse_production
validate_no_production_shaped_urls

note "staging storm execute suite — artifacts: ${ARTIFACT_DIR}"
echo "  LOAD_TEST_ENV=${LOAD_TEST_ENV} EXECUTE_LOAD_TEST=${EXECUTE_LOAD_TEST} DRY_RUN=${DRY_RUN}"
echo "  STORM_SCENARIO_MODE=${STORM_SCENARIO_MODE}"
echo "  EVENT_RATE_PER_MACHINE=${EVENT_RATE_PER_MACHINE:-2} CRITICAL_EVENT_RATIO=${CRITICAL_EVENT_RATIO:-0.1}"
redact_broker_log_line
echo "  API_READY_URL: set (host redacted in logs; value not echoed)"
echo "  MACHINE_IDS_FILE=${MACHINE_IDS_FILE}"
echo "  MQTT_TOPIC_PREFIX=${MQTT_TOPIC_PREFIX}"
echo "  metrics: derived or explicit (hosts not echoed when using secrets)"

for scen in "${SCENARIOS[@]}"; do
	note "scenario ${scen} (stop on failure)"
	SCENARIO_PRESET="${scen}" \
		RESULT_JSON_FILE="${ARTIFACT_DIR}/telemetry-storm-result-${scen}.json" \
		EVENT_RATE_PER_MACHINE="${EVENT_RATE_PER_MACHINE:-2}" \
		CRITICAL_EVENT_RATIO="${CRITICAL_EVENT_RATIO:-0.1}" \
		bash "${STORM_SCRIPT}" || {
		SUITE_PASS=false
		FAILED_AT="${scen}"
		break
	}
done

export SUITE_ARTIFACT_DIR="${ARTIFACT_DIR}"
export SUITE_PASS="${SUITE_PASS}"
export FAILED_AT="${FAILED_AT}"
export SUITE_MODE="execute"
export SCENARIO_LIST="${SCENARIO_LIST_JOINED}"
export LOAD_TEST_ENV
export STORM_SCENARIO_MODE
write_suite_summary

if ! truthy "${SUITE_PASS}"; then
	die "suite failed${FAILED_AT:+ at scenario ${FAILED_AT}} — see ${ARTIFACT_DIR}/telemetry-storm-suite-result.json"
fi
note "suite PASS — ${ARTIFACT_DIR}"
