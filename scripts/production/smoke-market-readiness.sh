#!/usr/bin/env bash
# Safe read-only production market-readiness smoke (no writes, orders, or vends).
# Usage:
#   BASE_URL=https://api.ldtv.dev ACCESS_TOKEN=<admin JWT> ./scripts/production/smoke-market-readiness.sh
# Optional: MACHINE_ACCESS_TOKEN=... MQTT_TEST=0
set -euo pipefail

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
ACCESS_TOKEN="${ACCESS_TOKEN:-}"
MACHINE_ACCESS_TOKEN="${MACHINE_ACCESS_TOKEN:-}"
MQTT_TEST="${MQTT_TEST:-0}"
OUTPUT_DIR="${OUTPUT_DIR:-.production-smoke-runs}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${OUTPUT_DIR}/${TS}"
mkdir -p "${RUN_DIR}"
SUMMARY="${RUN_DIR}/summary.txt"
: >"${SUMMARY}"

log() { echo "$1" | tee -a "${SUMMARY}"; }
fail() { log "FAIL: $1"; exit 1; }

curl_json() {
  local name="$1"
  local method="$2"
  local path="$3"
  local token="${4:-}"
  local out="${RUN_DIR}/${name}.json"
  local hdr=(-H "Accept: application/json" -H "X-Request-ID: smoke-${TS}-${name}")
  if [[ -n "${token}" ]]; then
    hdr+=(-H "Authorization: Bearer ${token}")
  fi
  local timing
  timing=$(curl -sS -o "${out}" -w "http_code=%{http_code} total=%{time_total} ttfb=%{time_starttransfer}" \
    -X "${method}" "${hdr[@]}" "${BASE_URL}${path}" 2>"${RUN_DIR}/${name}.err" || echo "curl_error=1")
  echo "${name}: ${timing}" >>"${SUMMARY}"
  local code
  code=$(echo "${timing}" | sed -n 's/.*http_code=\([0-9]*\).*/\1/p')
  [[ "${code}" == "200" ]] || fail "${name} expected 200 got ${code:-unknown}"
}

log "run_dir=${RUN_DIR}"
log "base_url=${BASE_URL}"

curl_json health_live GET /health/live ""
curl_json health_ready GET /health/ready ""
curl_json version GET /version ""

if [[ -z "${ACCESS_TOKEN}" ]]; then
  log "SKIP admin APIs (ACCESS_TOKEN unset)"
else
  curl_json categories GET "/v1/admin/categories?limit=5" "${ACCESS_TOKEN}"
  curl_json brands GET "/v1/admin/brands?limit=5" "${ACCESS_TOKEN}"
  curl_json tags GET "/v1/admin/tags?limit=5" "${ACCESS_TOKEN}"
  curl_json products GET "/v1/admin/products?limit=5" "${ACCESS_TOKEN}"
  curl_json machines GET "/v1/admin/machines?limit=5" "${ACCESS_TOKEN}"
  curl_json media_assets GET "/v1/admin/media/assets?limit=5" "${ACCESS_TOKEN}" || log "WARN: media/assets optional"
fi

if [[ -n "${MACHINE_ACCESS_TOKEN}" ]]; then
  log "machine token provided — gRPC/REST machine reads are manual (no destructive REST in this script)"
else
  log "SKIP machine-specific reads (MACHINE_ACCESS_TOKEN unset)"
fi

if [[ "${MQTT_TEST}" == "1" ]]; then
  log "MQTT_TEST=1 requires explicit broker env — not run in default smoke"
else
  log "SKIP MQTT (MQTT_TEST not 1)"
fi

log "PASS: market-readiness smoke completed"
echo "Report: ${RUN_DIR}"
