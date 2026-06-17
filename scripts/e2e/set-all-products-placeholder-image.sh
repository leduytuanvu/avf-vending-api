#!/usr/bin/env bash
# Bulk-assign a shared placeholder product image to every admin catalog product (temporary test media).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

PLACEHOLDER_URL="${1:-https://res.cloudinary.com/dz4qz0tk9/image/upload/v1779376340/avf-vending/products/019e4b18-346b-78b7-a370-4b8339120062.png}"

if [[ "${E2E_ALLOW_WRITES:-false}" != "true" ]]; then
  echo "FATAL: E2E_ALLOW_WRITES=true required" >&2
  exit 2
fi

E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=../../tests/e2e/lib/e2e_production_destructive_aliases.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_production_destructive_aliases.sh"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

e2e_strict_mode
e2e_require_cmd curl jq python3

export E2E_RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
export E2E_RUN_DIR="${E2E_RUN_DIR:-${ROOT}/reports/e2e/set-all-products-placeholder-image/${E2E_RUN_TS}}"
mkdir -p "${E2E_RUN_DIR}/raw"

READINESS="${E2E_RUN_DIR}/SETUP_READINESS.txt"
FAILURES=()

write_readiness() {
  local verdict="$1"
  {
    echo "VERDICT=${verdict}"
    echo "UTC=$(e2e_now_utc)"
    echo "PLACEHOLDER_URL=${PLACEHOLDER_URL}"
    echo "E2E_RUN_DIR=${E2E_RUN_DIR}"
    if [[ ${#FAILURES[@]} -gt 0 ]]; then
      echo "FAILURES:"
      printf '%s\n' "${FAILURES[@]}"
    fi
  } >"$READINESS"
}

fail_step() {
  FAILURES+=("$1")
  echo "FAIL: $1" >&2
}

load_env "${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
CANARY_ENV="${ROOT}/tests/e2e/production/.env.production.e2e.local"
if [[ -f "$CANARY_ENV" ]]; then
  _env_tmp="$(mktemp)"
  tr -d '\r' <"$CANARY_ENV" >"$_env_tmp"
  set -a
  # shellcheck disable=SC1090
  source "$_env_tmp"
  set +a
  rm -f "$_env_tmp"
  : "${BASE_URL:=${E2E_PROD_BASE_URL:-${BASE_URL:-}}}"
  : "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-${ADMIN_EMAIL:-}}}"
  : "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-${ADMIN_PASSWORD:-}}}"
  : "${ADMIN_TOKEN:=${E2E_PROD_ADMIN_TOKEN:-${ADMIN_TOKEN:-}}}"
fi
e2e_apply_production_destructive_aliases

if [[ "${E2E_TARGET:-}" == "production" ]]; then
  if [[ "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" ]]; then
    fail_step "missing E2E_PRODUCTION_WRITE_CONFIRMATION"
    write_readiness FAIL
    exit 2
  fi
fi

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL

ADMIN_TOK=""
if ! ADMIN_TOK="$(e2e_admin_token)"; then
  auth_detail="admin auth failed"
  if [[ -n "${E2E_ADMIN_AUTH_HTTP_CODE:-}" ]]; then
    auth_detail="${auth_detail} (login http=${E2E_ADMIN_AUTH_HTTP_CODE})"
  elif [[ -z "${ADMIN_EMAIL:-}" || -z "${ADMIN_PASSWORD:-}" ]]; then
    auth_detail="${auth_detail} (set ADMIN_TOKEN or ADMIN_EMAIL/ADMIN_PASSWORD in E2E env)"
  fi
  fail_step "$auth_detail"
  write_readiness FAIL
  exit 2
fi

e2e_curl_json() {
  local method="$1" name="$2" url="$3" body="$4" idem="${5:-}"
  local out="${E2E_RUN_DIR}/raw/${name}.body"
  local meta="${E2E_RUN_DIR}/raw/${name}.meta"
  local code="000"
  printf '%s' "$body" >"${E2E_RUN_DIR}/raw/${name}.request.json"
  local -a hdr=( -H "Content-Type: application/json" -H "Accept: application/json" -H "Authorization: Bearer ${ADMIN_TOK}" )
  [[ -n "$idem" ]] && hdr+=( -H "Idempotency-Key: ${idem}" )
  code="$(curl -sS -o "$out" -w '%{http_code}' -X "$method" "${hdr[@]}" --connect-timeout 8 --max-time 60 -d "$body" "$url" 2>/dev/null)" || code="000"
  printf '%s' "$code" >"$meta"
  echo "$code"
}

e2e_curl_get() {
  local name="$1" url="$2"
  local meta="${E2E_RUN_DIR}/raw/${name}.meta"
  local code="000"
  code="$(curl -sS -o "${E2E_RUN_DIR}/raw/${name}.body" -w '%{http_code}' \
    -H "Accept: application/json" \
    -H "Authorization: Bearer ${ADMIN_TOK}" \
    --connect-timeout 8 --max-time 30 \
    "$url" 2>/dev/null)" || code="000"
  printf '%s' "$code" >"$meta"
  echo "$code"
}

MEDIA_BODY="$(jq -nc \
  --arg url "$PLACEHOLDER_URL" \
  '{url:$url,purpose:"product_image",filename:"good-mood-placeholder.png",contentType:"image/png"}')"
MEDIA_CODE="$(e2e_curl_json POST external-image "${BASE_URL}/v1/admin/media/external-images" "$MEDIA_BODY" "e2e-placeholder-media-${E2E_RUN_TS}")"
if [[ "$MEDIA_CODE" == "503" ]]; then
  fail_step "external image register http=503 (enable PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED and allowlist res.cloudinary.com on API)"
  write_readiness FAIL
  exit 2
fi
if [[ "$MEDIA_CODE" != "200" && "$MEDIA_CODE" != "201" ]]; then
  fail_step "external image register http=${MEDIA_CODE}"
  write_readiness FAIL
  exit 2
fi
MEDIA_ID="$(jq -r '.mediaId // .id // empty' "${E2E_RUN_DIR}/raw/external-image.body")"
[[ -n "$MEDIA_ID" ]] || {
  fail_step "external image mediaId missing"
  write_readiness FAIL
  exit 2
}

UPDATED=0
FAILED=0
PAGE=1
LIMIT=100
while :; do
  code="$(e2e_curl_get "products-page-${PAGE}" "${BASE_URL}/v1/admin/products?limit=${LIMIT}&page=${PAGE}")"
  [[ "$code" == "200" ]] || {
    fail_step "products list page=${PAGE} http=${code}"
    break
  }
  body="${E2E_RUN_DIR}/raw/products-page-${PAGE}.body"
  count="$(jq -r '(.items // []) | length' "$body")"
  [[ "$count" -gt 0 ]] || break
  while IFS= read -r pid; do
    [[ -n "$pid" ]] || continue
    patch_body="$(jq -nc --arg mid "$MEDIA_ID" '{primaryMediaId:$mid}')"
    safe_id="$(echo "$pid" | tr -c 'A-Za-z0-9._-' '_')"
    pcode="$(e2e_curl_json PATCH "product-${safe_id}" "${BASE_URL}/v1/admin/products/${pid}" "$patch_body" "e2e-prod-img-${pid}")"
    if [[ "$pcode" == "200" ]]; then
      UPDATED=$((UPDATED + 1))
    else
      FAILED=$((FAILED + 1))
      fail_step "product ${pid} patch http=${pcode}"
    fi
  done < <(jq -r '.items[]?.id // empty' "$body")
  if [[ "$count" -lt "$LIMIT" ]]; then
    break
  fi
  PAGE=$((PAGE + 1))
done

{
  echo "mediaId=${MEDIA_ID}"
  echo "updated=${UPDATED}"
  echo "failed=${FAILED}"
} >"${E2E_RUN_DIR}/SUMMARY.txt"

if [[ "$FAILED" -gt 0 ]]; then
  write_readiness PARTIAL
  exit 2
fi

write_readiness PASS
echo "PASS updated=${UPDATED} mediaId=${MEDIA_ID}"
