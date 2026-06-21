#!/usr/bin/env bash
# Read-only audit of production test orders from PROD_WRITE_LEDGER.
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"
E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"

e2e_require_cmd curl jq
RUN_DIR="${E2E_RUN_DIR:-${1:-}}"
[[ -n "$RUN_DIR" ]] || {
  E2E_RUN_TS="$(date -u +%Y%m%dT%H%M%SZ)"
  RUN_DIR="${ROOT}/../reports/20260621T211500Z/reconciliation"
}
export E2E_RUN_DIR="$RUN_DIR"
export E2E_RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "${E2E_RUN_DIR}/raw"

load_env "${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
export BASE_URL="${BASE_URL:-https://api.ldtv.dev}"

ORDERS=(
  "019eebf5-08af-7874-9a70-bfc07a6a1c02:gate4-probe"
  "019eec00-3516-76e8-9ccb-682faed7f078:canary-a"
  "019eec05-4ae1-7a99-a75a-488cd1e02130:canary-b"
)

ADMIN_TOK="$(e2e_admin_token)" || {
  echo "FATAL: admin auth failed (check ADMIN_EMAIL/ADMIN_PASSWORD or ADMIN_TOKEN)" >&2
  exit 2
}

redact_json() {
  local in="$1" out="$2"
  jq 'walk(
    if type == "string" and (length > 40) then .[0:8] + "…" else . end
  ) | del(.accessToken, .access_token, .refreshToken, .refresh_token)' "$in" >"$out" 2>/dev/null || cp "$in" "$out"
}

for entry in "${ORDERS[@]}"; do
  oid="${entry%%:*}"
  label="${entry##*:}"
  echo "AUDIT order=${oid} label=${label}"
  for path in "commerce-order" "commerce-reconciliation" "admin-timeline"; do
    case "$path" in
      commerce-order)
        url="${BASE_URL%/}/v1/commerce/orders/${oid}"
        name="${label}-checkout"
        ;;
      commerce-reconciliation)
        url="${BASE_URL%/}/v1/commerce/orders/${oid}/reconciliation"
        name="${label}-reconciliation"
        ;;
      admin-timeline)
        url="${BASE_URL%/}/v1/admin/orders/${oid}/timeline?limit=50"
        name="${label}-timeline"
        ;;
    esac
    body="${E2E_RUN_DIR}/raw/${name}.body.tmp"
    code="$(curl -sS -o "$body" -w '%{http_code}' \
      -H "Authorization: Bearer ${ADMIN_TOK}" \
      -H "Accept: application/json" \
      --connect-timeout 8 --max-time 25 \
      "$url")"
    redact_json "$body" "${E2E_RUN_DIR}/raw/${name}.json"
    jq -nc --arg oid "$oid" --arg label "$label" --arg path "$path" \
      --argjson code "$code" \
      --arg file "${name}.json" \
      '{orderId:$oid, label:$label, endpoint:$path, http_code:$code, artifact:$file}' \
      >>"${E2E_RUN_DIR}/audit-index.ndjson"
    rm -f "$body"
  done
done

# Machine inventory snapshot (single read for slot A1 context)
e2e_curl_get "machine-inventory" "${BASE_URL%/}/v1/admin/machines/019e702c-11c6-7ab0-89c7-5eb32f0b12cb/inventory" "$ADMIN_TOK" >/dev/null || true
[[ -f "${E2E_RUN_DIR}/raw/machine-inventory.body" ]] && \
  redact_json "${E2E_RUN_DIR}/raw/machine-inventory.body" "${E2E_RUN_DIR}/raw/machine-inventory.json"

echo "AUDIT_DONE run_dir=${E2E_RUN_DIR}"
