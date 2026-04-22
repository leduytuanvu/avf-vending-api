#!/usr/bin/env bash
set -Eeuo pipefail

EXIT_CODE_USAGE=20
EXIT_CODE_REQUEST_FAILURE=21
EXIT_CODE_EXPECTATION_FAILURE=22

usage() {
	echo "usage: smoke_http.sh [--json] [--method METHOD] [--expect-status CSV] [--pattern REGEX] [--header 'K: V'] [--connect-to-host HOST] [--connect-to-port PORT] <label> <url> [pattern]" >&2
	exit "${EXIT_CODE_USAGE}"
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

JSON_MODE=0
METHOD="${SMOKE_HTTP_METHOD:-GET}"
EXPECT_STATUSES="${SMOKE_HTTP_EXPECT_STATUSES:-200}"
PATTERN="${SMOKE_HTTP_PATTERN:-}"
CONNECT_TO_HOST="${SMOKE_HTTP_CONNECT_TO_HOST:-}"
CONNECT_TO_PORT="${SMOKE_HTTP_CONNECT_TO_PORT:-}"
declare -a EXTRA_HEADERS=()
declare -a POSITIONAL=()

while [[ $# -gt 0 ]]; do
	case "$1" in
	--json)
		JSON_MODE=1
		shift
		;;
	--method)
		[[ $# -ge 2 ]] || usage
		METHOD="$2"
		shift 2
		;;
	--expect-status)
		[[ $# -ge 2 ]] || usage
		EXPECT_STATUSES="$2"
		shift 2
		;;
	--pattern)
		[[ $# -ge 2 ]] || usage
		PATTERN="$2"
		shift 2
		;;
	--header)
		[[ $# -ge 2 ]] || usage
		EXTRA_HEADERS+=("$2")
		shift 2
		;;
	--connect-to-host)
		[[ $# -ge 2 ]] || usage
		CONNECT_TO_HOST="$2"
		shift 2
		;;
	--connect-to-port)
		[[ $# -ge 2 ]] || usage
		CONNECT_TO_PORT="$2"
		shift 2
		;;
	--help | -h)
		usage
		;;
	--)
		shift
		while [[ $# -gt 0 ]]; do
			POSITIONAL+=("$1")
			shift
		done
		;;
	*)
		POSITIONAL+=("$1")
		shift
		;;
	esac
done

if [[ "${#POSITIONAL[@]}" -lt 2 || "${#POSITIONAL[@]}" -gt 3 ]]; then
	usage
fi

LABEL="${POSITIONAL[0]}"
URL="${POSITIONAL[1]}"
if [[ "${#POSITIONAL[@]}" -eq 3 ]]; then
	PATTERN="${POSITIONAL[2]}"
fi

[[ -n "${LABEL}" ]] || usage
[[ -n "${URL}" ]] || usage

ATTEMPTS="${SMOKE_HTTP_ATTEMPTS:-6}"
DELAY_SECS="${SMOKE_HTTP_DELAY_SECS:-5}"
CONNECT_TIMEOUT="${SMOKE_HTTP_CONNECT_TIMEOUT_SECS:-5}"
MAX_TIME="${SMOKE_HTTP_MAX_TIME_SECS:-15}"

last_body=""
last_detail="not-run"
last_http_status=""
result="fail"
exit_code="${EXIT_CODE_REQUEST_FAILURE}"

for attempt in $(seq 1 "${ATTEMPTS}"); do
	body_file="$(mktemp)"
	curl_args=(
		--silent
		--show-error
		--request "${METHOD}"
		--output "${body_file}"
		--write-out "%{http_code}"
		--connect-timeout "${CONNECT_TIMEOUT}"
		--max-time "${MAX_TIME}"
	)

	for header in "${EXTRA_HEADERS[@]}"; do
		curl_args+=(--header "${header}")
	done

	if [[ -n "${CONNECT_TO_HOST}" ]]; then
		url_host="$(python3 -c 'import sys; from urllib.parse import urlparse; parsed = urlparse(sys.argv[1]); print(parsed.hostname or "")' "${URL}")"
		url_port="$(python3 -c 'import sys; from urllib.parse import urlparse; parsed = urlparse(sys.argv[1]); print(parsed.port or (443 if parsed.scheme == "https" else 80))' "${URL}")"
		curl_args+=(--connect-to "${url_host}:${url_port}:${CONNECT_TO_HOST}:${CONNECT_TO_PORT:-${url_port}}")
	fi

	set +e
	last_http_status="$(curl "${curl_args[@]}" "${URL}" 2>&1)"
	curl_rc=$?
	set -e

	last_body="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace"), end="")' "${body_file}" 2>/dev/null || true)"
	rm -f "${body_file}"

	if ((curl_rc != 0)); then
		last_detail="curl failed with exit code ${curl_rc}"
		exit_code="${EXIT_CODE_REQUEST_FAILURE}"
	elif ! python3 -c 'import sys; expected = [item.strip() for item in sys.argv[1].split(",") if item.strip()]; actual = sys.argv[2].strip(); raise SystemExit(0 if actual in expected else 1)' "${EXPECT_STATUSES}" "${last_http_status}"; then
		last_detail="unexpected HTTP status ${last_http_status} (expected ${EXPECT_STATUSES})"
		exit_code="${EXIT_CODE_EXPECTATION_FAILURE}"
	elif [[ -n "${PATTERN}" ]] && ! printf '%s' "${last_body}" | grep -Eq "${PATTERN}"; then
		last_detail="response body did not match required pattern"
		exit_code="${EXIT_CODE_EXPECTATION_FAILURE}"
	else
		result="pass"
		last_detail="ok"
		exit_code=0
		break
	fi

	if [[ "${attempt}" -lt "${ATTEMPTS}" ]]; then
		sleep "${DELAY_SECS}"
	fi
done

if [[ "${JSON_MODE}" == "1" ]]; then
	printf '{'
	printf '"label":"%s",' "$(json_escape "${LABEL}")"
	printf '"url":"%s",' "$(json_escape "${URL}")"
	printf '"method":"%s",' "$(json_escape "${METHOD}")"
	printf '"expected_statuses":"%s",' "$(json_escape "${EXPECT_STATUSES}")"
	printf '"http_status":"%s",' "$(json_escape "${last_http_status}")"
	printf '"result":"%s",' "$(json_escape "${result}")"
	printf '"detail":"%s"' "$(json_escape "${last_detail}")"
	if [[ -n "${PATTERN}" ]]; then
		printf ',"pattern":"%s"' "$(json_escape "${PATTERN}")"
	fi
	printf '}\n'
else
	if [[ "${result}" == "pass" ]]; then
		echo "PASS: ${LABEL} (${METHOD} ${URL} -> ${last_http_status})"
	else
		echo "FAIL: ${LABEL}" >&2
		echo "  detail: ${last_detail}" >&2
		if [[ -n "${last_http_status}" ]]; then
			echo "  http_status: ${last_http_status}" >&2
		fi
	fi
fi

exit "${exit_code}"
