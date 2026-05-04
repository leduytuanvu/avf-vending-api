#!/usr/bin/env bash
# Production blackbox smoke: tiered (health, business-readonly, optional business-safe-synthetic).
# Only GET/HEAD - never real payment capture, dispense, inventory mutation, or MQTT commands.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROD_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
cd "${PROD_DIR}"

SMOKE_PYTHON="${SMOKE_PYTHON:-python3}"

EMITTER_PY="${REPO_ROOT}/scripts/smoke/emit_production_smoke_json.py"
JSON_MODE=0
SMOKE_LEVEL="${SMOKE_LEVEL:-business-readonly}"
SHOW_HELP=0
	while [[ $# -gt 0 ]]; do
	case "$1" in
		--json)
			JSON_MODE=1
			SMOKE_JSON=1
			export SMOKE_JSON
			shift
			;;
		--level)
			shift
			SMOKE_LEVEL="${1-}"
			shift
			;;
		--help | -h)
			SHOW_HELP=1
			shift
			;;
		*)
			break
			;;
	esac
done
if [[ "${SMOKE_JSON:-0}" == "1" ]]; then
	JSON_MODE=1
fi
if [[ "${JSON_MODE}" == "1" ]]; then
	SMOKE_JSON=1
	export SMOKE_JSON
fi

if [[ "${SHOW_HELP}" == "1" ]]; then
	cat <<'EOF'
Usage: smoke_prod.sh [--json] [--level LEVEL] [--help]

Tiers (SMOKE_LEVEL):
  health                  — reachability, /health/ready, /health/live, /version
  business-readonly       — health + read-only business signal (default for production)
  business-safe-synthetic | full — above + optional synthetic GET (no side effects) when enabled

Environment (high level):
  SMOKE_BASE_URL / SMOKE_API_DOMAIN / API_DOMAIN, SMOKE_CONNECT_TO_HOST, SMOKE_CONNECT_TO_PORT
  SMOKE_LEVEL, SMOKE_ENABLE_BUSINESS_SYNTHETIC (0/1)
  SMOKE_BUSINESS_READONLY_BEARER_TOKEN (optional Bearer for /v1/... GET probes)
  SMOKE_BUSINESS_READONLY_SPECS — explicit checks: "name|path|statuses|regex" separated by ";;"
  SMOKE_DB_READ_PATH, SMOKE_DB_READ_MATCH_REGEX (legacy DB read probe)
  SMOKE_SYNTHETIC_GET_PATH, SMOKE_SYNTHETIC_BEARER_TOKEN, SMOKE_SYNTHETIC_MATCH_REGEX
  SMOKE_PYTHON — interpreter for emit_production_smoke_json.py and URL helpers (default: python3)

This script only issues GET. It must never trigger real payment, dispense, inventory
changes, or MQTT. See docs/operations/production-smoke-tests.md
EOF
	exit 0
fi

if [[ $# -gt 0 ]]; then
	echo "error: unexpected arguments: $* (try --help)" >&2
	exit 2
fi

[[ -f "${EMITTER_PY}" ]] || {
	echo "error: missing JSON emitter: ${EMITTER_PY}" >&2
	exit 2
}

human_log() {
	if [[ "${JSON_MODE:-0}" == "1" ]]; then
		printf '%s\n' "$*" >&2
	else
		printf '%s\n' "$*"
	fi
}

EXIT_CODE_CONFIG_FAILURE=30
EXIT_CODE_SMOKE_FAILURE=31

CHECKS_FILE="$(mktemp)"
SKIP_REASONS_FILE="$(mktemp)"
CONFIG_FAILURES=0
FAILURES=0
SKIPS=0
SMOKE_JSON_EMITTED=0

emit_smoke_json_minimal_critical() {
	local reason="${1:-unexpected_script_exit}"
	export SMOKE_EMIT_FALLBACK_REASON="${reason}"
	export SMOKE_COMPLETED_AT_UTC="${SMOKE_COMPLETED_AT_UTC:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
	if "${SMOKE_PYTHON}" -c '
import json, os, sys
reason = os.environ.get("SMOKE_EMIT_FALLBACK_REASON", "unexpected")
payload = {
    "level": os.environ.get("SMOKE_LEVEL", "unknown"),
    "started_at_utc": os.environ.get("SMOKE_STARTED_AT_UTC", ""),
    "completed_at_utc": os.environ.get("SMOKE_COMPLETED_AT_UTC", ""),
    "overall_status": "fail",
    "final_result": "fail",
    "base_url": os.environ.get("SMOKE_BASE_URL", ""),
    "connect_to_host": os.environ.get("SMOKE_CONNECT_TO_HOST", ""),
    "smoke_label": os.environ.get("SMOKE_LABEL", ""),
    "health_result": os.environ.get("SMOKE_HEALTH_RESULT", "not-run"),
    "business_readonly_result": os.environ.get("SMOKE_BUSINESS_READONLY_RESULT", "not-run"),
    "business_synthetic_result": os.environ.get("SMOKE_BUSINESS_SYNTHETIC_RESULT", "not-run"),
    "zero_side_effects_claim": True,
    "base_url_result": os.environ.get("SMOKE_BASE_URL_RESULT", "not-run"),
    "critical_read_result": os.environ.get("SMOKE_CRITICAL_READ_RESULT", "not-run"),
    "optional_db_read_result": os.environ.get("SMOKE_OPTIONAL_DB_READ_RESULT", "not-run"),
    "checks": [],
    "failed_checks": [],
    "skipped_checks": [],
    "skipped_reasons": [],
    "emitter_fallback": True,
    "fallback_detail": reason,
}
json.dump(payload, sys.stdout, indent=2)
sys.stdout.write("\n")
' 2>/dev/null; then
		return 0
	fi
	printf '%s\n' '{"overall_status":"fail","final_result":"fail","level":"unknown","checks":[],"failed_checks":[],"skipped_checks":[],"skipped_reasons":[],"emitter_fallback":true,"fallback_detail":"python_emitter_unavailable"}'
}

try_emit_json_or_minimal() {
	if [[ "${JSON_MODE:-0}" != "1" ]]; then
		return 0
	fi
	if [[ "${SMOKE_JSON_EMITTED:-0}" == "1" ]]; then
		return 0
	fi
	export SMOKE_CHECKS_FILE="${CHECKS_FILE}"
	export SMOKE_SKIPPED_REASONS_FILE="${SKIP_REASONS_FILE}"
	export SMOKE_LEVEL
	export SMOKE_STARTED_AT_UTC
	export SMOKE_BASE_URL="${BASE_URL:-}"
	export SMOKE_LABEL="${SMOKE_LABEL:-}"
	export SMOKE_CONNECT_TO_HOST="${SMOKE_CONNECT_TO_HOST:-}"
	export SMOKE_OVERALL_STATUS
	export SMOKE_BUSINESS_SYNTHETIC_RESULT
	export SMOKE_ZERO_SIDE_EFFECTS_CLAIM
	export SMOKE_BASE_URL_RESULT SMOKE_CRITICAL_READ_RESULT SMOKE_OPTIONAL_DB_READ_RESULT
	export SMOKE_HEALTH_RESULT SMOKE_BUSINESS_READONLY_RESULT
	SMOKE_COMPLETED_AT_UTC="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
	export SMOKE_COMPLETED_AT_UTC
	if [[ ! -f "${EMITTER_PY}" ]]; then
		human_log "WARN: missing JSON emitter; minimal fail JSON on stdout only"
		emit_smoke_json_minimal_critical "missing_emitter_py"
		SMOKE_JSON_EMITTED=1
		return 0
	fi
	set +e
	"${SMOKE_PYTHON}" "${EMITTER_PY}"
	local _emit_rc=$?
	set -e
	if [[ "${_emit_rc}" -ne 0 ]]; then
		human_log "WARN: smoke JSON emitter exited ${_emit_rc}; minimal fallback JSON on stdout"
		emit_smoke_json_minimal_critical "emitter_exit_${_emit_rc}"
	fi
	SMOKE_JSON_EMITTED=1
}

# Tier outcomes for evidence (emitter)
SMOKE_HEALTH_RESULT="not-run"
SMOKE_BUSINESS_READONLY_RESULT="not-run"
SMOKE_BUSINESS_SYNTHETIC_RESULT="not-run"
SMOKE_BASE_URL_RESULT="not-run"
SMOKE_CRITICAL_READ_RESULT="not-run"
SMOKE_OPTIONAL_DB_READ_RESULT="not-run"
SMOKE_ZERO_SIDE_EFFECTS_CLAIM="true"
HEALTH_TIER_OK=1
BR_TIER_OK=1
SYN_TIER_OK=1

append_skip_reason() {
	local code="$1"
	local detail="$2"
	printf '%s\t%s\n' "${code}" "${detail}" >> "${SKIP_REASONS_FILE}"
}

cleanup() {
	rm -f "${CHECKS_FILE}" "${SKIP_REASONS_FILE}"
}

on_smoke_shell_exit() {
	try_emit_json_or_minimal
	cleanup
}
trap on_smoke_shell_exit EXIT

json_escape() {
	local value="${1-}"
	value="${value//\\/\\\\}"
	value="${value//\"/\\\"}"
	value="${value//$'\n'/\\n}"
	value="${value//$'\r'/\\r}"
	value="${value//$'\t'/\\t}"
	printf '%s' "${value}"
}

note() { human_log "==> $*"; }
pass() { human_log "PASS: $1"; }
skip() { human_log "SKIP: $1"; }
warn() { human_log "WARN: $*"; }

read_env_optional() {
	local key="$1"
	local env_file="${SMOKE_ENV_FILE:-.env.production}"
	local line
	if [[ ! -f "${env_file}" ]]; then
		printf ''
		return 0
	fi
	line="$(grep -E "^${key}=" "${env_file}" | tail -n1 || true)"
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

record_check() {
	local name="$1"
	local status="$2"
	local url="$3"
	local http_status="$4"
	local detail="$5"
	printf '%s\t%s\t%s\t%s\t%s\n' "${name}" "${status}" "${url}" "${http_status}" "${detail}" >> "${CHECKS_FILE}"
	case "${status}" in
		fail) FAILURES=$((FAILURES + 1)) ;;
		skip) SKIPS=$((SKIPS + 1)) ;;
	esac
}

record_config_failure() {
	local name="$1"
	local detail="$2"
	CONFIG_FAILURES=$((CONFIG_FAILURES + 1))
	record_check "${name}" "fail" "" "" "${detail}"
	human_log "FAIL: ${name} — ${detail}"
}

record_skip() {
	local name="$1"
	local detail="$2"
	local url="${3:-}"
	record_check "${name}" "skip" "${url}" "" "${detail}"
	skip "${name} (${detail})"
}

csv_has_status() {
	local expected_csv="$1"
	local actual="$2"
	local item
	IFS=',' read -r -a expected_items <<< "${expected_csv}"
	for item in "${expected_items[@]}"; do
		if [[ "${item}" == "${actual}" ]]; then
			return 0
		fi
	done
	return 1
}

curl_check() {
	local name="$1"
	local url="$2"
	local expected_statuses="$3"
	local body_regex="${4:-}"
	local result_var="$5"
	local body_file http_status curl_rc detail curl_args url_host url_port

	body_file="$(mktemp)"
	curl_args=(
		--silent
		--show-error
		--output "${body_file}"
		--write-out "%{http_code}"
		--max-time "${SMOKE_TIMEOUT_SECS:-10}"
	)

	if [[ -n "${SMOKE_CONNECT_TO_HOST:-}" ]]; then
		url_host="$("${SMOKE_PYTHON}" -c 'import sys; from urllib.parse import urlparse; parsed = urlparse(sys.argv[1]); print(parsed.hostname or "")' "${url}")"
		url_port="$("${SMOKE_PYTHON}" -c 'import sys; from urllib.parse import urlparse; parsed = urlparse(sys.argv[1]); print(parsed.port or (443 if parsed.scheme == "https" else 80))' "${url}")"
		curl_args+=(--connect-to "${url_host}:${url_port}:${SMOKE_CONNECT_TO_HOST}:${SMOKE_CONNECT_TO_PORT:-${url_port}}")
	fi

	if [[ "${SMOKE_CURL_AUTH_MODE:-}" == "readonly" && -n "${SMOKE_BUSINESS_READONLY_BEARER_TOKEN:-}" ]]; then
		curl_args+=(-H "Authorization: Bearer ${SMOKE_BUSINESS_READONLY_BEARER_TOKEN}")
	elif [[ "${SMOKE_CURL_AUTH_MODE:-}" == "synthetic" && -n "${SMOKE_SYNTHETIC_BEARER_TOKEN:-}" ]]; then
		curl_args+=(-H "Authorization: Bearer ${SMOKE_SYNTHETIC_BEARER_TOKEN}")
	fi

	set +e
	http_status="$(curl "${curl_args[@]}" "${url}")"
	curl_rc=$?
	set -e

	if ((curl_rc != 0)); then
		detail="curl failed with exit code ${curl_rc}"
		record_check "${name}" "fail" "${url}" "curl-exit-${curl_rc}" "${detail}"
		eval "${result_var}=failed"
		human_log "FAIL: ${name} — ${detail}"
		rm -f "${body_file}"
		return 1
	fi

	if ! csv_has_status "${expected_statuses}" "${http_status}"; then
		detail="unexpected HTTP status ${http_status} (expected ${expected_statuses})"
		record_check "${name}" "fail" "${url}" "${http_status}" "${detail}"
		eval "${result_var}=failed"
		human_log "FAIL: ${name} — ${detail}"
		rm -f "${body_file}"
		return 1
	fi

	if [[ -n "${body_regex}" ]]; then
		if ! grep -Eq "${body_regex}" "${body_file}"; then
			detail="response body did not match required pattern"
			record_check "${name}" "fail" "${url}" "${http_status}" "${detail}"
			eval "${result_var}=failed"
			human_log "FAIL: ${name} — ${detail}"
			rm -f "${body_file}"
			return 1
		fi
	fi

	record_check "${name}" "pass" "${url}" "${http_status}" "ok"
	eval "${result_var}=passed"
	pass "${name}"
	rm -f "${body_file}"
	return 0
}

# GET /swagger/doc.json — 404 is skip (not mounted); 5xx / curl error is fail; 200 + body = pass
openapi_readonly_default_probe() {
	local url="${BASE_URL}/swagger/doc.json"
	local body http_status curl_rc curl_args=() url_host url_port
	body="$(mktemp)"
	curl_args=(
		--silent
		--show-error
		--output "${body}"
		--write-out "%{http_code}"
		--max-time "${SMOKE_TIMEOUT_SECS:-10}"
	)
	if [[ -n "${SMOKE_CONNECT_TO_HOST:-}" ]]; then
		url_host="$("${SMOKE_PYTHON}" -c 'import sys; from urllib.parse import urlparse; print(urlparse(sys.argv[1]).hostname or "")' "${url}")"
		url_port="$("${SMOKE_PYTHON}" -c 'import sys; from urllib.parse import urlparse; p=urlparse(sys.argv[1]); print(p.port or (443 if p.scheme == "https" else 80))' "${url}")"
		curl_args+=(--connect-to "${url_host}:${url_port}:${SMOKE_CONNECT_TO_HOST}:${SMOKE_CONNECT_TO_PORT:-${url_port}}")
	fi
	set +e
	http_status="$(curl "${curl_args[@]}" "${url}")"
	curl_rc=$?
	set -e
	if ((curl_rc != 0)); then
		record_check "business-readonly: openapi public document" "fail" "${url}" "curl-${curl_rc}" "curl failed"
		rm -f "${body}"
		return 1
	fi
	if [[ "${http_status}" == "404" ]]; then
		record_check "business-readonly: openapi public document" "skip" "${url}" "404" "OpenAPI JSON not served at /swagger/doc.json (intentional in some prod configs)"
		append_skip_reason "openapi_not_mounted" "GET /swagger/doc.json returned 404; use SMOKE_BUSINESS_READONLY_SPECS or SMOKE_DB_READ_PATH"
		rm -f "${body}"
		return 2
	fi
	if [[ "${http_status}" == "200" ]] && grep -Eq 'openapi|swagger|"paths"' "${body}"; then
		record_check "business-readonly: openapi public document" "pass" "${url}" "200" "ok"
		rm -f "${body}"
		return 0
	fi
	record_check "business-readonly: openapi public document" "fail" "${url}" "${http_status}" "unexpected response for OpenAPI discovery"
	rm -f "${body}"
	return 1
}

SMOKE_STARTED_AT_UTC="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
INVALID_SMOKE_LEVEL=0
case "${SMOKE_LEVEL}" in
	health | business-readonly | business-safe-synthetic | full) ;;
	*)
		INVALID_SMOKE_LEVEL=1
		record_config_failure "smoke level" "invalid SMOKE_LEVEL='${SMOKE_LEVEL}' (use health, business-readonly, business-safe-synthetic, full)"
		;;
esac

API_DOMAIN="${SMOKE_API_DOMAIN:-${PRODUCTION_API_DOMAIN:-$(read_env_optional API_DOMAIN)}}"
BASE_URL="${SMOKE_BASE_URL:-${PUBLIC_BASE_URL:-$(read_env_optional PUBLIC_BASE_URL)}}"
if [[ -z "${BASE_URL}" && -n "${API_DOMAIN}" ]]; then
	BASE_URL="https://${API_DOMAIN}"
fi
if [[ -z "${BASE_URL}" ]]; then
	record_config_failure "base URL configuration" "set SMOKE_BASE_URL, PUBLIC_BASE_URL, or API_DOMAIN"
	BASE_URL="https://missing-base-url.invalid"
fi
BASE_URL="${BASE_URL%/}"

CRITICAL_READ_PATH="${SMOKE_CRITICAL_READ_PATH:-/version}"
CRITICAL_READ_MATCH_REGEX="${SMOKE_CRITICAL_READ_MATCH_REGEX:-\"version\"[[:space:]]*:}"
DB_READ_PATH="${SMOKE_DB_READ_PATH:-}"
DB_READ_MATCH_REGEX="${SMOKE_DB_READ_MATCH_REGEX:-}"

ELIGIBLE_SYNTHETIC=0
case "${SMOKE_LEVEL}" in
	business-safe-synthetic | full) ELIGIBLE_SYNTHETIC=1 ;;
esac

if ((INVALID_SMOKE_LEVEL == 1)); then
	SMOKE_HEALTH_RESULT="fail"
	SMOKE_BUSINESS_READONLY_RESULT="not-run"
	SMOKE_BUSINESS_SYNTHETIC_RESULT="not-run"
	SMOKE_OVERALL_STATUS="fail"
	exit "${EXIT_CODE_CONFIG_FAILURE}"
fi

# --- Tier: health (all valid levels) ---
{
	note "blackbox smoke level=${SMOKE_LEVEL} against ${BASE_URL}"
	if [[ -n "${SMOKE_CONNECT_TO_HOST:-}" ]]; then
		note "forcing edge connection to ${SMOKE_CONNECT_TO_HOST}:${SMOKE_CONNECT_TO_PORT:-443}"
	fi

	SMOKE_CURL_AUTH_MODE=none
	if ! curl_check \
		"public base URL reachability" \
		"${BASE_URL}" \
		"${SMOKE_BASE_URL_ACCEPT_STATUSES:-200,301,302,307,308,401,403,404}" \
		"" \
		SMOKE_BASE_URL_RESULT; then
		HEALTH_TIER_OK=0
	fi

	if ! curl_check \
		"health/ready" \
		"${BASE_URL}/health/ready" \
		"200" \
		"^ok" \
		HRDY; then
		HEALTH_TIER_OK=0
	fi

	if ! curl_check \
		"health/live" \
		"${BASE_URL}/health/live" \
		"200" \
		"^ok" \
		HLIV; then
		HEALTH_TIER_OK=0
	fi

	if ! curl_check \
		"critical read-only API smoke" \
		"${BASE_URL}${CRITICAL_READ_PATH}" \
		"200" \
		"${CRITICAL_READ_MATCH_REGEX}" \
		SMOKE_CRITICAL_READ_RESULT; then
		HEALTH_TIER_OK=0
	fi
}

# Optional strict git metadata (off by default)
if [[ -n "${SMOKE_VERSION_REQUIRE_GIT_SHA:-}" && "${SMOKE_VERSION_REQUIRE_GIT_SHA}" == "1" ]]; then
	SMOKE_CURL_AUTH_MODE=none
	# best-effort second check: version payload includes git_sha
	if ! curl_check \
		"version includes git_sha" \
		"${BASE_URL}${CRITICAL_READ_PATH}" \
		"200" \
		"\"git_sha\"[[:space:]]*:" \
		vgit; then
		HEALTH_TIER_OK=0
	fi
fi

if ((HEALTH_TIER_OK == 1)); then
	SMOKE_HEALTH_RESULT="pass"
else
	SMOKE_HEALTH_RESULT="fail"
fi

# Skipped tiers for health-only
case "${SMOKE_LEVEL}" in
	health)
		SMOKE_BUSINESS_READONLY_RESULT="skipped"
		SMOKE_BUSINESS_SYNTHETIC_RESULT="skipped"
		SMOKE_OPTIONAL_DB_READ_RESULT="skipped"
		append_skip_reason "business_readonly_not_run" "SMOKE_LEVEL=health (business checks not in scope for this run)"
		append_skip_reason "synthetic_not_run" "SMOKE_LEVEL=health"
		;;
esac

# --- business-readonly ---
if [[ "${SMOKE_LEVEL}" != "health" ]]; then
	RO_DID_PASS=0
	if [[ -n "${SMOKE_BUSINESS_READONLY_SPECS:-}" ]]; then
		SMOKE_CURL_AUTH_MODE=readonly
		remaining="${SMOKE_BUSINESS_READONLY_SPECS}"
		while [[ -n "${remaining}" ]]; do
			chunk="${remaining%%::*}"
			if [[ "${remaining}" == *::* ]]; then
				remaining="${remaining#*::}"
			else
				remaining=""
			fi
			IFS='|' read -r br_name br_path br_statuses br_regex <<< "${chunk}||||"
			br_name="$(printf '%s' "${br_name}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
			[[ -n "${br_name}" ]] || continue
			br_path="$(printf '%s' "${br_path}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
			[[ -n "${br_path}" ]] || {
				BR_TIER_OK=0
				record_config_failure "business_readonly spec" "empty path in SMOKE_BUSINESS_READONLY_SPECS"
				break
			}
			[[ -n "${br_statuses}" ]] || br_statuses="200"
			brv="brv_${RANDOM}"
			if ! curl_check "${br_name}" "${BASE_URL}${br_path}" "${br_statuses}" "${br_regex}" "${brv}"; then
				BR_TIER_OK=0
			else
				RO_DID_PASS=1
			fi
		done
		SMOKE_CURL_AUTH_MODE=none
		# optional DB in addition
		if [[ -n "${DB_READ_PATH}" ]]; then
			SMOKE_CURL_AUTH_MODE=none
			if curl_check \
				"optional DB-backed read smoke" \
				"${BASE_URL}${DB_READ_PATH}" \
				"${SMOKE_DB_READ_ACCEPT_STATUSES:-200}" \
				"${DB_READ_MATCH_REGEX}" \
				SMOKE_OPTIONAL_DB_READ_RESULT; then
				RO_DID_PASS=1
			else
				BR_TIER_OK=0
			fi
		else
			SMOKE_OPTIONAL_DB_READ_RESULT="skipped"
			record_skip "optional DB-backed read smoke" "SMOKE_DB_READ_PATH not set"
		fi
	else
		# Default discovery: OpenAPI JSON (404 = optional skip in production) + optional DB
		RO_DID_PASS=0
		openapi_optional_not_mounted=0
		SMOKE_CURL_AUTH_MODE=none
		openapi_readonly_default_probe
		oprc=$?
		if [[ "${oprc}" -eq 0 ]]; then
			RO_DID_PASS=1
		elif [[ "${oprc}" -eq 1 ]]; then
			BR_TIER_OK=0
		elif [[ "${oprc}" -eq 2 ]]; then
			# 404 at /swagger/doc.json: optional; critical tier already proved API + /version
			openapi_optional_not_mounted=1
		fi
		if [[ -n "${DB_READ_PATH}" ]]; then
			if curl_check \
				"optional DB-backed read smoke" \
				"${BASE_URL}${DB_READ_PATH}" \
				"${SMOKE_DB_READ_ACCEPT_STATUSES:-200}" \
				"${DB_READ_MATCH_REGEX}" \
				SMOKE_OPTIONAL_DB_READ_RESULT; then
				RO_DID_PASS=1
			else
				BR_TIER_OK=0
			fi
		else
			SMOKE_OPTIONAL_DB_READ_RESULT="skipped"
			record_skip "optional DB-backed read smoke" "SMOKE_DB_READ_PATH not set"
		fi
		if ((RO_DID_PASS == 0)); then
			if [[ "${openapi_optional_not_mounted}" -eq 1 && -z "${DB_READ_PATH}" ]]; then
				# No SPECS/DB: OpenAPI unmounted is acceptable; keep business-readonly pass with skip only
				RO_DID_PASS=1
			else
				BR_TIER_OK=0
				append_skip_reason "business_readonly_no_signal" "no successful business-readonly probe — set SMOKE_BUSINESS_READONLY_SPECS, SMOKE_DB_READ_PATH, or expose a passing /swagger/doc.json"
				record_config_failure "business-readonly" "at least one business-readonly check must pass (OpenAPI, DB read, or explicit SPECS)"
			fi
		fi
	fi

	if ((BR_TIER_OK == 1)); then
		SMOKE_BUSINESS_READONLY_RESULT="pass"
	else
		SMOKE_BUSINESS_READONLY_RESULT="fail"
	fi
fi

# --- business-safe-synthetic (optional) ---
if [[ "${ELIGIBLE_SYNTHETIC}" -eq 1 ]]; then
	if [[ "${SMOKE_ENABLE_BUSINESS_SYNTHETIC:-0}" != "1" ]]; then
		SMOKE_BUSINESS_SYNTHETIC_RESULT="skipped"
		append_skip_reason "synthetic_tier_disabled" "SMOKE_ENABLE_BUSINESS_SYNTHETIC is not 1 (default off)"
	else
		synth_path="${SMOKE_SYNTHETIC_GET_PATH:-}"
		if [[ -z "${synth_path}" ]]; then
			SMOKE_BUSINESS_SYNTHETIC_RESULT="skipped"
			SMOKE_ZERO_SIDE_EFFECTS_CLAIM="true"
			append_skip_reason "synthetic_not_configured" "no vetted safe synthetic GET path (SMOKE_SYNTHETIC_GET_PATH) — public API has no unauthenticated production dry-run; skipped, not pass"
		else
			# only GET, never POST
			SMOKE_CURL_AUTH_MODE=synthetic
			sy_regex="${SMOKE_SYNTHETIC_MATCH_REGEX:-.}"
			if ! curl_check \
				"business-safe-synthetic GET" \
				"${BASE_URL}${synth_path}" \
				"200" \
				"${sy_regex}" \
				sy_r; then
				SYN_TIER_OK=0
				SMOKE_BUSINESS_SYNTHETIC_RESULT="fail"
				SMOKE_ZERO_SIDE_EFFECTS_CLAIM="false"
			else
				SMOKE_BUSINESS_SYNTHETIC_RESULT="pass"
			fi
			SMOKE_CURL_AUTH_MODE=none
		fi
	fi
else
	if [[ "${SMOKE_LEVEL}" == "health" ]]; then
		:
	else
		SMOKE_BUSINESS_SYNTHETIC_RESULT="skipped"
		append_skip_reason "synthetic_not_eligible" "raise SMOKE_LEVEL to business-safe-synthetic to attempt synthetic tier"
	fi
fi

# Overall
SMOKE_OVERALL_STATUS="pass"
if ((CONFIG_FAILURES > 0)) || ((FAILURES > 0)); then
	SMOKE_OVERALL_STATUS="fail"
fi
# Tier policy: must not claim success if health or BR failed (when in scope)
if [[ "${SMOKE_LEVEL}" != "health" ]]; then
	if [[ "${SMOKE_HEALTH_RESULT}" == "fail" || "${SMOKE_BUSINESS_READONLY_RESULT}" == "fail" ]]; then
		SMOKE_OVERALL_STATUS="fail"
	fi
fi
if [[ "${ELIGIBLE_SYNTHETIC}" -eq 1 && "${SMOKE_ENABLE_BUSINESS_SYNTHETIC:-0}" == "1" && -n "${SMOKE_SYNTHETIC_GET_PATH:-}" && "${SMOKE_BUSINESS_SYNTHETIC_RESULT}" == "fail" ]]; then
	SMOKE_OVERALL_STATUS="fail"
fi
if [[ "${SMOKE_LEVEL}" == "health" ]]; then
	if [[ "${SMOKE_HEALTH_RESULT}" == "fail" ]]; then
		SMOKE_OVERALL_STATUS="fail"
	fi
fi

OVERALL_RC=0
if ((CONFIG_FAILURES > 0)); then
	OVERALL_RC="${EXIT_CODE_CONFIG_FAILURE}"
elif ((FAILURES > 0)); then
	OVERALL_RC="${EXIT_CODE_SMOKE_FAILURE}"
elif [[ "${SMOKE_OVERALL_STATUS}" == "fail" ]]; then
	OVERALL_RC="${EXIT_CODE_SMOKE_FAILURE}"
else
	OVERALL_RC=0
fi

if [[ "${JSON_MODE}" != "1" ]] && [[ "${OVERALL_RC}" -eq 0 ]]; then
	human_log "smoke_prod: PASS (level=${SMOKE_LEVEL})"
fi
exit "${OVERALL_RC}"
