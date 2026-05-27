#!/usr/bin/env bash
# Measure public HTTP latency for AVF API health and admin read endpoints.
# Usage:
#   BASE_URL=https://api.ldtv.dev ACCESS_TOKEN=<token> ./scripts/production/measure-http-latency.sh
#   OUTPUT_DIR=.production-latency-runs ./scripts/production/measure-http-latency.sh
set -euo pipefail

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
ACCESS_TOKEN="${ACCESS_TOKEN:-}"
OUTPUT_DIR="${OUTPUT_DIR:-.production-latency-runs}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${OUTPUT_DIR}/${TS}"
mkdir -p "${RUN_DIR}"

CURL_FORMAT='dns=%{time_namelookup} connect=%{time_connect} tls=%{time_appconnect} ttfb=%{time_starttransfer} total=%{time_total} http_code=%{http_code}\n'

measure() {
  local name="$1"
  local method="$2"
  local path="$3"
  local auth="${4:-no}"
  local out="${RUN_DIR}/${name}.txt"
  local hdr_auth=()

  if [[ "${auth}" == "yes" && -z "${ACCESS_TOKEN}" ]]; then
    echo "${name}: SKIP (ACCESS_TOKEN not set)" | tee -a "${RUN_DIR}/summary.txt"
    return 0
  fi
  if [[ "${auth}" == "yes" ]]; then
    hdr_auth=(-H "Authorization: Bearer ${ACCESS_TOKEN}")
  fi

  {
    echo "# ${method} ${path}"
    echo "# $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    curl -sS -o /dev/null -w "${CURL_FORMAT}" \
      -X "${method}" \
      "${hdr_auth[@]}" \
      -H "Accept: application/json" \
      -H "X-Request-ID: latency-${TS}-${name}" \
      "${BASE_URL}${path}" 2>&1 || echo "curl_exit=$?"
    curl -sS -D - -o /dev/null \
      -X "${method}" \
      "${hdr_auth[@]}" \
      -H "Accept: application/json" \
      -H "X-Request-ID: latency-${TS}-${name}-headers" \
      "${BASE_URL}${path}" 2>/dev/null | grep -E '^(HTTP/|x-request-id:|X-Request-ID:)' || true
  } >"${out}"

  local line
  line="$(grep -E '^dns=' "${out}" | tail -1 || true)"
  echo "${name}: ${line}" | tee -a "${RUN_DIR}/summary.txt"
}

echo "run_dir=${RUN_DIR}" | tee "${RUN_DIR}/meta.env"
echo "base_url=${BASE_URL}" >>"${RUN_DIR}/meta.env"

measure health_live GET /health/live no
measure health_ready GET /health/ready no
measure version GET /version no

measure admin_products GET '/v1/admin/products?limit=5' yes
measure admin_machines GET '/v1/admin/machines?limit=5' yes
measure admin_categories GET '/v1/admin/categories?limit=5' yes
measure admin_brands GET '/v1/admin/brands?limit=5' yes
measure admin_tags GET '/v1/admin/tags?limit=5' yes

echo "Results written under ${RUN_DIR}"
echo "Summary: ${RUN_DIR}/summary.txt"

# Fail if health endpoints are not HTTP 200
for h in health_live health_ready; do
  f="${RUN_DIR}/${h}.txt"
  if ! grep -q 'http_code=200' "${f}" 2>/dev/null; then
    echo "ERROR: ${h} did not return http_code=200" >&2
    exit 1
  fi
done
