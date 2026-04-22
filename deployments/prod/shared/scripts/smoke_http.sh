#!/usr/bin/env bash
set -Eeuo pipefail

LABEL="${1:-}"
URL="${2:-}"
PATTERN="${3:-}"

[[ -n "${LABEL}" ]] || { echo "usage: smoke_http.sh <label> <url> <pattern>" >&2; exit 1; }
[[ -n "${URL}" ]] || { echo "usage: smoke_http.sh <label> <url> <pattern>" >&2; exit 1; }
[[ -n "${PATTERN}" ]] || { echo "usage: smoke_http.sh <label> <url> <pattern>" >&2; exit 1; }

ATTEMPTS="${SMOKE_HTTP_ATTEMPTS:-6}"
DELAY_SECS="${SMOKE_HTTP_DELAY_SECS:-5}"
CONNECT_TIMEOUT="${SMOKE_HTTP_CONNECT_TIMEOUT_SECS:-5}"
MAX_TIME="${SMOKE_HTTP_MAX_TIME_SECS:-15}"

last_output=""

for attempt in $(seq 1 "${ATTEMPTS}"); do
	if last_output="$(curl -fsS --connect-timeout "${CONNECT_TIMEOUT}" --max-time "${MAX_TIME}" "${URL}" 2>&1)"; then
		if printf '%s' "${last_output}" | grep -Eq "${PATTERN}"; then
			echo "PASS: ${LABEL}"
			exit 0
		fi
		last_output="response did not match pattern ${PATTERN}: ${last_output}"
	fi

	if [[ "${attempt}" -lt "${ATTEMPTS}" ]]; then
		sleep "${DELAY_SECS}"
	fi
done

echo "FAIL: ${LABEL}" >&2
if [[ -n "${last_output}" ]]; then
	echo "${last_output}" | sed 's/^/  output: /' >&2
fi
exit 1
