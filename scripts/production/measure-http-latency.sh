#!/usr/bin/env bash
# HTTP latency baseline for market-readiness (see docs/operations/production-latency-runbook.md).
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
  local name="$1" method="$2" path="$3" auth="${4:-no}"
  local out="${RUN_DIR}/${name}.txt"
  local hdr=()
  [[ "${auth}" == "yes" && -n "${ACCESS_TOKEN}" ]] || { echo "${name}: SKIP" | tee -a "${RUN_DIR}/summary.txt"; return 0; }
  [[ "${auth}" == "yes" ]] && hdr=(-H "Authorization: Bearer ${ACCESS_TOKEN}")
  { echo "# ${method} ${path}"; curl -sS -o /dev/null -w "${CURL_FORMAT}" -X "${method}" "${hdr[@]}" "${BASE_URL}${path}"; } >"${out}"
  grep -E '^dns=' "${out}" | tee -a "${RUN_DIR}/summary.txt"
}
measure health_live GET /health/live no
measure health_ready GET /health/ready no
measure version GET /version no
measure admin_products GET '/v1/admin/products?limit=5' yes
measure admin_machines GET '/v1/admin/machines?limit=5' yes
for h in health_live health_ready; do
  grep -q 'http_code=200' "${RUN_DIR}/${h}.txt" || { echo "FAIL ${h}" >&2; exit 1; }
done
echo "OK ${RUN_DIR}"
