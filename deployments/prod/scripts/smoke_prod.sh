#!/usr/bin/env bash
# Read-only blackbox production smoke checks.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

JSON_MODE=0
if [[ "${SMOKE_JSON:-0}" == "1" ]]; then
	JSON_MODE=1
fi
while [[ $# -gt 0 ]]; do
	case "$1" in
		--json)
			JSON_MODE=1
			shift
			;;
		*)
			break
			;;
	esac
done

EXIT_CODE_CONFIG_FAILURE=30
EXIT_CODE_SMOKE_FAILURE=31

CHECKS_FILE="$(mktemp)"
CONFIG_FAILURES=0
FAILURES=0
SKIPS=0

BASE_URL_RESULT="not-run"
CRITICAL_READ_RESULT="not-run"
OPTIONAL_DB_READ_RESULT="not-run"

cleanup() {
	rm -f "${CHECKS_FILE}"
}
trap cleanup EXIT

emit_info() {
	if [[ "${JSON_MODE}" == "1" ]]; then
		echo "$*" >&2
	else
		echo "$*"
	fi
}

json_escape() {
	local value="${1-}"
	value="${value//\\/\\\\}"
	value="${value//\"/\\\"}"
	value="${value//$'\n'/\\n}"
	value="${value//$'\r'/\\r}"
	value="${value//$'\t'/\\t}"
	printf '%s' "${value}"
}

note() {
	emit_info "==> $*"
}

pass() {
	emit_info "PASS: $1"
}

skip() {
	emit_info "SKIP: $1"
}

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
		fail)
			FAILURES=$((FAILURES + 1))
			;;
		skip)
			SKIPS=$((SKIPS + 1))
			;;
	esac
}

record_config_failure() {
	local name="$1"
	local detail="$2"
	CONFIG_FAILURES=$((CONFIG_FAILURES + 1))
	record_check "${name}" "fail" "" "" "${detail}"
	echo "FAIL: ${name} — ${detail}" >&2
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
		url_host="$(python3 -c 'import sys; from urllib.parse import urlparse; parsed = urlparse(sys.argv[1]); print(parsed.hostname or "")' "${url}")"
		url_port="$(python3 -c 'import sys; from urllib.parse import urlparse; parsed = urlparse(sys.argv[1]); print(parsed.port or (443 if parsed.scheme == "https" else 80))' "${url}")"
		curl_args+=(--connect-to "${url_host}:${url_port}:${SMOKE_CONNECT_TO_HOST}:${SMOKE_CONNECT_TO_PORT:-${url_port}}")
	fi

	set +e
	http_status="$(curl "${curl_args[@]}" "${url}")"
	curl_rc=$?
	set -e

	if (( curl_rc != 0 )); then
		detail="curl failed with exit code ${curl_rc}"
		record_check "${name}" "fail" "${url}" "curl-exit-${curl_rc}" "${detail}"
		eval "${result_var}=failed"
		echo "FAIL: ${name} — ${detail}" >&2
		rm -f "${body_file}"
		return 1
	fi

	if ! csv_has_status "${expected_statuses}" "${http_status}"; then
		detail="unexpected HTTP status ${http_status} (expected ${expected_statuses})"
		record_check "${name}" "fail" "${url}" "${http_status}" "${detail}"
		eval "${result_var}=failed"
		echo "FAIL: ${name} — ${detail}" >&2
		rm -f "${body_file}"
		return 1
	fi

	if [[ -n "${body_regex}" ]]; then
		if ! grep -Eq "${body_regex}" "${body_file}"; then
			detail="response body did not match required pattern"
			record_check "${name}" "fail" "${url}" "${http_status}" "${detail}"
			eval "${result_var}=failed"
			echo "FAIL: ${name} — ${detail}" >&2
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

emit_json_summary() {
	local overall_status="pass"
	local first="1"
	local name status url http_status detail
	if (( FAILURES > 0 )); then
		overall_status="fail"
	fi

	printf '{\n'
	printf '  "overall_status": "%s",\n' "$(json_escape "${overall_status}")"
	printf '  "base_url": "%s",\n' "$(json_escape "${BASE_URL}")"
	printf '  "connect_to_host": "%s",\n' "$(json_escape "${SMOKE_CONNECT_TO_HOST:-}")"
	printf '  "base_url_result": "%s",\n' "$(json_escape "${BASE_URL_RESULT}")"
	printf '  "critical_read_result": "%s",\n' "$(json_escape "${CRITICAL_READ_RESULT}")"
	printf '  "optional_db_read_result": "%s",\n' "$(json_escape "${OPTIONAL_DB_READ_RESULT}")"
	printf '  "checks": ['
	while IFS=$'\t' read -r name status url http_status detail; do
		[[ -n "${name}" ]] || continue
		if [[ "${first}" == "1" ]]; then
			first="0"
		else
			printf ','
		fi
		printf '\n    {"name":"%s","status":"%s","url":"%s","http_status":"%s","detail":"%s"}' \
			"$(json_escape "${name}")" \
			"$(json_escape "${status}")" \
			"$(json_escape "${url}")" \
			"$(json_escape "${http_status}")" \
			"$(json_escape "${detail}")"
	done < "${CHECKS_FILE}"
	if [[ "${first}" == "0" ]]; then
		printf '\n'
	fi
	printf '  ],\n'
	printf '  "failed_checks": ['
	first="1"
	while IFS=$'\t' read -r name status url http_status detail; do
		[[ "${status}" == "fail" ]] || continue
		if [[ "${first}" == "1" ]]; then
			first="0"
		else
			printf ','
		fi
		printf '\n    {"name":"%s","url":"%s","http_status":"%s","detail":"%s"}' \
			"$(json_escape "${name}")" \
			"$(json_escape "${url}")" \
			"$(json_escape "${http_status}")" \
			"$(json_escape "${detail}")"
	done < "${CHECKS_FILE}"
	if [[ "${first}" == "0" ]]; then
		printf '\n'
	fi
	printf '  ],\n'
	printf '  "skipped_checks": ['
	first="1"
	while IFS=$'\t' read -r name status url http_status detail; do
		[[ "${status}" == "skip" ]] || continue
		if [[ "${first}" == "1" ]]; then
			first="0"
		else
			printf ','
		fi
		printf '\n    {"name":"%s","detail":"%s"}' \
			"$(json_escape "${name}")" \
			"$(json_escape "${detail}")"
	done < "${CHECKS_FILE}"
	if [[ "${first}" == "0" ]]; then
		printf '\n'
	fi
	printf '  ]\n'
	printf '}\n'
}

if [[ $# -gt 0 ]]; then
	record_config_failure "smoke invocation" "unexpected arguments: $*"
fi

command -v curl >/dev/null 2>&1 || record_config_failure "smoke dependency" "curl is required on PATH"
command -v python3 >/dev/null 2>&1 || record_config_failure "smoke dependency" "python3 is required on PATH"

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

note "blackbox smoke against ${BASE_URL}"
if [[ -n "${SMOKE_CONNECT_TO_HOST:-}" ]]; then
	note "forcing edge connection to ${SMOKE_CONNECT_TO_HOST}:${SMOKE_CONNECT_TO_PORT:-443}"
fi

curl_check \
	"public base URL reachability" \
	"${BASE_URL}" \
	"${SMOKE_BASE_URL_ACCEPT_STATUSES:-200,301,302,307,308,401,403,404}" \
	"" \
	BASE_URL_RESULT

curl_check \
	"critical read-only API smoke" \
	"${BASE_URL}${CRITICAL_READ_PATH}" \
	"200" \
	"${CRITICAL_READ_MATCH_REGEX}" \
	CRITICAL_READ_RESULT

if [[ -n "${DB_READ_PATH}" ]]; then
	curl_check \
		"optional DB-backed read smoke" \
		"${BASE_URL}${DB_READ_PATH}" \
		"${SMOKE_DB_READ_ACCEPT_STATUSES:-200}" \
		"${DB_READ_MATCH_REGEX}" \
		OPTIONAL_DB_READ_RESULT
else
	OPTIONAL_DB_READ_RESULT="skipped"
	record_skip "optional DB-backed read smoke" "SMOKE_DB_READ_PATH not set"
fi

if [[ "${JSON_MODE}" == "1" ]]; then
	emit_json_summary
fi

if (( CONFIG_FAILURES > 0 )); then
	exit "${EXIT_CODE_CONFIG_FAILURE}"
fi
if (( FAILURES > 0 )); then
	exit "${EXIT_CODE_SMOKE_FAILURE}"
fi

if [[ "${JSON_MODE}" != "1" ]]; then
	echo "smoke_prod: PASS"
fi
