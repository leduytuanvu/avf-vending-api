#!/usr/bin/env bash
# Optional read-only HTTP observability gate (no external SaaS required).
#
# Required:
#   OBSERVABILITY_CHECK_URL   e.g. https://status.example.com/health or internal LB URL
# Optional:
#   OBSERVABILITY_CHECK_TIMEOUT_SEC     default 15
#   OBSERVABILITY_EXPECT_HTTP_STATUS      default 200
#   OBSERVABILITY_HEALTHY_SUBSTRING      if set, response body must contain this literal substring
#   OBSERVABILITY_UNHEALTHY_BODY_REGEX  if set, exit 1 when body matches (e.g. '"status":"unhealthy"')
set -Eeuo pipefail

URL="${OBSERVABILITY_CHECK_URL:?OBSERVABILITY_CHECK_URL is required}"
TIMEOUT="${OBSERVABILITY_CHECK_TIMEOUT_SEC:-15}"
EXPECT_STATUS="${OBSERVABILITY_EXPECT_HTTP_STATUS:-200}"
TMP_BODY="$(mktemp)"
trap 'rm -f "${TMP_BODY}"' EXIT

code="000"
set +e
code="$(curl -sS -o "${TMP_BODY}" -w "%{http_code}" --max-time "${TIMEOUT}" -L "${URL}" 2>/dev/null)"
curl_rc=$?
set -e

if [[ "${curl_rc}" -ne 0 ]]; then
  echo "observability_http_check: curl failed (rc=${curl_rc}) for ${URL}" >&2
  exit 1
fi

if [[ "${code}" != "${EXPECT_STATUS}" ]]; then
  echo "observability_http_check: HTTP ${code} (expected ${EXPECT_STATUS}) for ${URL}" >&2
  head -c 4000 "${TMP_BODY}" >&2 || true
  exit 1
fi

if [[ -n "${OBSERVABILITY_UNHEALTHY_BODY_REGEX:-}" ]]; then
  if grep -qE "${OBSERVABILITY_UNHEALTHY_BODY_REGEX}" "${TMP_BODY}"; then
    echo "observability_http_check: response matched OBSERVABILITY_UNHEALTHY_BODY_REGEX" >&2
    head -c 4000 "${TMP_BODY}" >&2 || true
    exit 1
  fi
fi

if [[ -n "${OBSERVABILITY_HEALTHY_SUBSTRING:-}" ]]; then
  if ! grep -qF -- "${OBSERVABILITY_HEALTHY_SUBSTRING}" "${TMP_BODY}"; then
    echo "observability_http_check: body missing required substring (healthy marker)" >&2
    head -c 4000 "${TMP_BODY}" >&2 || true
    exit 1
  fi
fi

echo "observability_http_check: OK (${code}) ${URL}"
