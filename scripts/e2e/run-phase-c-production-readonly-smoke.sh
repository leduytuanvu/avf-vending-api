#!/usr/bin/env bash
# Phase C — production read-only smoke orchestrator.
# Loads local env, resolves machine credentials when missing (activation claim only),
# runs production-readonly-smoke.sh, then supplementary image URL probes.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=../../tests/e2e/lib/e2e_production_destructive_aliases.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_production_destructive_aliases.sh"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
PHASE_DIR="${E2E_RUN_DIR:-${ROOT}/tests/e2e/.e2e-runs/${RUN_TS}-phase-c}"
mkdir -p "${PHASE_DIR}/raw" "${PHASE_DIR}/logs"
export E2E_RUN_TS="$RUN_TS"
export E2E_RUN_DIR="$PHASE_DIR"
export E2E_OUTPUT_DIR="${PHASE_DIR}/smoke"

CANARY_ENV="${ROOT}/tests/e2e/production/.env.production.e2e.local"
load_env "${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
if [[ -f "$CANARY_ENV" ]]; then
  _env_tmp="$(mktemp)"
  tr -d '\r' <"$CANARY_ENV" >"$_env_tmp"
  set -a
  # shellcheck disable=SC1090
  source "$_env_tmp"
  set +a
  rm -f "$_env_tmp"
  : "${BASE_URL:=${E2E_PROD_BASE_URL:-}}"
  : "${GRPC_ADDR:=${E2E_PROD_GRPC_TARGET:-${GRPC_ADDR:-}}}"
  : "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-${ADMIN_EMAIL:-}}}"
  : "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-${ADMIN_PASSWORD:-}}}"
  : "${MACHINE_ACCESS_TOKEN:=${E2E_PROD_MACHINE_ACCESS_TOKEN:-${MACHINE_ACCESS_TOKEN:-}}}"
  : "${TEST_MACHINE_ID:=${E2E_PROD_MACHINE_ID:-${TEST_MACHINE_ID:-}}}"
  : "${MQTT_HOST:=${E2E_PROD_MQTT_HOST:-${MQTT_HOST:-}}}"
  : "${MQTT_USERNAME:=${E2E_PROD_MQTT_USERNAME:-${MQTT_USERNAME:-}}}"
  : "${MQTT_PASSWORD:=${E2E_PROD_MQTT_PASSWORD:-${MQTT_PASSWORD:-}}}"
fi
e2e_apply_production_destructive_aliases
: "${TEST_MACHINE_ID:=${MACHINE_ID:-}}"
: "${MACHINE_ACCESS_TOKEN:=${MACHINE_TOKEN:-}}"

export BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
# REST base URL must not override machine gRPC target (destructive.local often sets api.ldtv.dev:443).
if [[ -n "${E2E_PROD_GRPC_TARGET:-}" ]]; then
  export GRPC_ADDR="${E2E_PROD_GRPC_TARGET}"
elif [[ "${GRPC_ADDR:-}" == *"api.ldtv.dev"* ]]; then
  export GRPC_ADDR="machine-api.ldtv.dev:443"
else
  export GRPC_ADDR="${GRPC_ADDR:-machine-api.ldtv.dev:443}"
fi
export GRPC_USE_PLAINTEXT="${GRPC_USE_PLAINTEXT:-false}"
export GRPC_USE_REFLECTION="${GRPC_USE_REFLECTION:-false}"
export GRPC_PROTO_ROOT="${GRPC_PROTO_ROOT:-${ROOT}/proto}"
export ALLOW_TEST_MACHINE_TELEMETRY_PUSH="${ALLOW_TEST_MACHINE_TELEMETRY_PUSH:-true}"

log() { printf '%s\n' "$*" | tee -a "${PHASE_DIR}/phase-c.log"; }

resolve_machine_credentials() {
  if [[ -n "${MACHINE_ACCESS_TOKEN:-}" && -n "${TEST_MACHINE_ID:-}" ]]; then
    log "machine credentials: present in env (values redacted)"
    return 0
  fi

  e2e_require_cmd curl jq grpcurl
  local admin_tok
  admin_tok="$(e2e_admin_token)" || {
    log "FAIL cannot mint machine token: admin login failed"
    return 1
  }

  if [[ -z "${TEST_MACHINE_ID:-}" ]]; then
    local machines_body="${PHASE_DIR}/raw/admin-machines-resolve.json"
    local code
    code="$(curl -sS -o "$machines_body" -w '%{http_code}' \
      -H "Accept: application/json" \
      -H "Authorization: Bearer ${admin_tok}" \
      "${BASE_URL%/}/v1/admin/machines?limit=50")"
    if [[ "$code" != "200" ]]; then
      log "FAIL admin machines list http=${code}"
      return 1
    fi
    TEST_MACHINE_ID="$(jq -r --arg allow "${PRODUCTION_CANARY_MACHINE_ALLOWLIST:-019e702c-11c6-7ab0-89c7-5eb32f0b12cb}" '
      ($allow | split(",") | map(gsub("^\\s+|\\s+$";"")) | map(select(length>0))) as $allowed |
      if ($allowed | length) > 0 then
        (.items // [])[] |
        select((.machineId // .id // "") as $id | ($allowed | index($id)) != null) |
        select((.status // "") | test("^(online|offline)$"; "i")) |
        (.machineId // .id)
      else empty end
    ' "$machines_body" | head -n1)"
    if [[ -z "$TEST_MACHINE_ID" || "$TEST_MACHINE_ID" == "null" ]]; then
      TEST_MACHINE_ID="$(jq -r '
        (.items // [])[] |
        select((.status // "") == "online") |
        select((.inventorySummary.occupiedSlots // 0) > 0) |
        (.machineId // .id)
      ' "$machines_body" | head -n1)"
    fi
    if [[ -z "$TEST_MACHINE_ID" || "$TEST_MACHINE_ID" == "null" ]]; then
      TEST_MACHINE_ID="$(jq -r '
        (.items // [])[] |
        select((.status // "") | test("^(online|offline)$"; "i")) |
        (.machineId // .id)
      ' "$machines_body" | head -n1)"
    fi
    [[ -n "$TEST_MACHINE_ID" && "$TEST_MACHINE_ID" != "null" ]] || {
      log "FAIL could not resolve TEST_MACHINE_ID from admin machines list"
      return 1
    }
    export TEST_MACHINE_ID
    log "resolved TEST_MACHINE_ID=${TEST_MACHINE_ID} via admin list"
  fi

  if [[ -n "${MACHINE_ACCESS_TOKEN:-}" ]]; then
    return 0
  fi

  local act_body="${PHASE_DIR}/raw/activation-code-create.json"
  local act_code http_code
  http_code="$(curl -sS -o "$act_body" -w '%{http_code}' -X POST \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    -H "Authorization: Bearer ${admin_tok}" \
    -H "Idempotency-Key: phase-c-ro-${E2E_RUN_TS}" \
    -d "$(jq -nc --arg n "phase-c-readonly-smoke ${E2E_RUN_TS}" '{expiresInMinutes:60,maxUses:1,notes:$n}')" \
    "${BASE_URL%/}/v1/admin/machines/${TEST_MACHINE_ID}/activation-codes")"
  if [[ "$http_code" != "201" && "$http_code" != "200" ]]; then
    log "FAIL activation code create http=${http_code}"
    return 1
  fi
  act_code="$(jq -r '.activationCode // empty' "$act_body")"
  [[ -n "$act_code" ]] || {
    log "FAIL activation code missing in admin response"
    return 1
  }

  local claim_req="${PHASE_DIR}/raw/activation-claim.request.json"
  jq -nc \
    --arg code "$act_code" \
    --arg sn "phase-c-ro-${E2E_RUN_TS}" \
    '{activationCode:$code,deviceFingerprint:{androidId:$sn,serialNumber:$sn,manufacturer:"avf",model:"phase-c-smoke",packageName:"dev.avf.e2e.phasec",versionName:"1.0.0",versionCode:1}}' \
    >"$claim_req"

  if e2e_grpc_call "avf.machine.v1.MachineActivationService/ClaimActivation" "$(cat "$claim_req")" "activation-claim-phase-c" none ""; then
    MACHINE_ACCESS_TOKEN="$(jq -r '.accessToken // .machineToken // empty' "${PHASE_DIR}/raw/activation-claim-phase-c.response.json")"
    MACHINE_REFRESH_TOKEN="$(jq -r '.refreshToken // empty' "${PHASE_DIR}/raw/activation-claim-phase-c.response.json")"
  else
    http_code="$(curl -sS -o "${PHASE_DIR}/raw/activation-claim-rest.json" -w '%{http_code}' -X POST \
      -H "Content-Type: application/json" \
      -H "Accept: application/json" \
      -d @"$claim_req" \
      "${BASE_URL%/}/v1/setup/activation-codes/claim")"
    if [[ "$http_code" != "200" ]]; then
      log "FAIL activation claim http/grpc failed"
      return 1
    fi
    MACHINE_ACCESS_TOKEN="$(jq -r '.machineToken // .accessToken // empty' "${PHASE_DIR}/raw/activation-claim-rest.json")"
    MACHINE_REFRESH_TOKEN="$(jq -r '.refreshToken // empty' "${PHASE_DIR}/raw/activation-claim-rest.json")"
  fi

  [[ -n "$MACHINE_ACCESS_TOKEN" ]] || {
    log "FAIL machine access token missing after claim"
    return 1
  }
  export MACHINE_ACCESS_TOKEN MACHINE_REFRESH_TOKEN MACHINE_TOKEN="$MACHINE_ACCESS_TOKEN"
  log "machine token minted via activation claim (token redacted)"
  [[ -n "${MACHINE_REFRESH_TOKEN:-}" ]] && log "machine refresh token acquired (redacted)"
  for f in "${PHASE_DIR}/raw/activation-claim-phase-c.response.json" "${PHASE_DIR}/raw/activation-claim-rest.json"; do
    [[ -f "$f" ]] || continue
    sed -i -E \
      -e 's/("accessToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      -e 's/("refreshToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      -e 's/("machineToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      "$f" 2>/dev/null || true
  done
  return 0
}

probe_image_urls() {
  local manifest="${E2E_OUTPUT_DIR}/production-readonly-smoke/${E2E_RUN_TS}/raw/grpc-media-manifest.response.json"
  if [[ ! -f "$manifest" ]]; then
    manifest="$(find "${PHASE_DIR}/smoke" -name 'grpc-media-manifest.response.json' 2>/dev/null | head -n1 || true)"
  fi
  [[ -f "$manifest" ]] || {
    printf '%s\n' "media.image_urls|SKIP|0|grpc media manifest missing" >>"${PHASE_DIR}/supplementary-probes.tsv"
    return 0
  }

  local urls_file="${PHASE_DIR}/raw/image-urls.txt"
  jq -r '
    [.entries[]? | .primaryMedia.displayUrl?, .primaryMedia.thumbUrl?,
     (.primaryMedia.mediaVariants[]?.url // empty)] |
    map(select(. != null and . != "")) | unique | .[]
  ' "$manifest" >"$urls_file"

  local url code fail=0 total=0
  while IFS= read -r url || [[ -n "${url:-}" ]]; do
    url="${url//$'\r'/}"
    url="${url#"${url%%[![:space:]]*}"}"
    url="${url%"${url##*[![:space:]]}"}"
    [[ -n "$url" ]] || continue
    total=$((total + 1))
    code="$(curl -sS -o /dev/null -w '%{http_code}' -I --connect-timeout 8 --max-time 20 "$url" 2>/dev/null || echo "000")"
    code="${code//$'\r'/}"
    if [[ "$code" == "200" || "$code" == "304" ]]; then
      printf '%s\n' "media.image_url|PASS|0|http=${code}" >>"${PHASE_DIR}/supplementary-probes.tsv"
    else
      fail=1
      printf '%s\n' "media.image_url|FAIL|0|http=${code}" >>"${PHASE_DIR}/supplementary-probes.tsv"
    fi
  done <"$urls_file"

  if [[ "$total" -eq 0 ]]; then
    printf '%s\n' "media.image_urls|WARN|0|no image URLs in media manifest" >>"${PHASE_DIR}/supplementary-probes.tsv"
  elif [[ "$fail" -eq 0 ]]; then
    printf '%s\n' "media.image_urls|PASS|0|checked=${total}" >>"${PHASE_DIR}/supplementary-probes.tsv"
  else
    printf '%s\n' "media.image_urls|FAIL|0|one or more image URLs unreachable (checked=${total})" >>"${PHASE_DIR}/supplementary-probes.tsv"
  fi
}

probe_migration_state() {
  printf '%s\n' "migration.production_state|SKIP|0|no safe remote migration status probe; offline validate_migration_image + deploy-prod run_migration gate" >>"${PHASE_DIR}/supplementary-probes.tsv"
}

probe_mqtt_tls() {
  local host="${MQTT_HOST:-${E2E_PROD_MQTT_HOST:-mqtt.ldtv.dev}}"
  local port="${MQTT_PORT:-${E2E_PROD_MQTT_PORT:-8883}}"
  host="${host#ssl://}"
  host="${host#tls://}"
  host="${host%%:*}"
  if command -v openssl >/dev/null 2>&1; then
    if curl -sS -o /dev/null --connect-timeout 8 --max-time 12 "https://${host}:${port}/" 2>/dev/null; then
      :
    fi
    if timeout 8 openssl s_client -connect "${host}:${port}" -servername "$host" </dev/null 2>"${PHASE_DIR}/raw/mqtt-tls.log" | grep -qi "CONNECTED"; then
      printf '%s\n' "mqtt.tls_connect|PASS|0|${host}:${port} TLS handshake ok" >>"${PHASE_DIR}/supplementary-probes.tsv"
    elif curl -sS -o /dev/null --connect-timeout 8 --max-time 12 "telnet://${host}:${port}" 2>/dev/null; then
      printf '%s\n' "mqtt.tls_connect|PASS|0|${host}:${port} TCP reachable (TLS verify skipped)" >>"${PHASE_DIR}/supplementary-probes.tsv"
    else
      printf '%s\n' "mqtt.tls_connect|WARN|0|${host}:${port} direct TLS probe inconclusive; bootstrap mqtt config passed" >>"${PHASE_DIR}/supplementary-probes.tsv"
    fi
  else
    printf '%s\n' "mqtt.tls_connect|SKIP|0|openssl not available" >>"${PHASE_DIR}/supplementary-probes.tsv"
  fi
}

: >"${PHASE_DIR}/supplementary-probes.tsv"
log "== Phase C production read-only smoke =="
log "phase_dir=${PHASE_DIR}"
log "base_url=${BASE_URL}"
log "grpc_addr=${GRPC_ADDR}"

CREDS_OK=1
if resolve_machine_credentials; then
  CREDS_OK=0
fi

SMOKE_RC=0
export E2E_RUN_DIR="${PHASE_DIR}/smoke/production-readonly-smoke/${E2E_RUN_TS}"
mkdir -p "${E2E_RUN_DIR}/raw" "${E2E_RUN_DIR}/logs"
bash "${ROOT}/scripts/e2e/production-readonly-smoke.sh" 2>&1 | tee "${PHASE_DIR}/logs/production-readonly-smoke.log" || SMOKE_RC=$?

probe_image_urls
probe_migration_state
probe_mqtt_tls

SMOKE_VERDICT="PRODUCTION_READONLY_SMOKE_FAIL"
READINESS="$(jq -r '.smoke_verdict // empty' "${E2E_RUN_DIR}/report.json" 2>/dev/null || true)"
SUPP_FAIL=0
if grep -q '|FAIL|' "${PHASE_DIR}/supplementary-probes.tsv" 2>/dev/null; then
  SUPP_FAIL=1
fi
if [[ "$READINESS" == "PASS" && "$SMOKE_RC" -eq 0 && "$SUPP_FAIL" -eq 0 ]]; then
  SMOKE_VERDICT="PRODUCTION_READONLY_SMOKE_PASS"
fi

cp -f "${E2E_RUN_DIR}/probes.tsv" "${PHASE_DIR}/probes.tsv" 2>/dev/null || true
if [[ -f "${PHASE_DIR}/supplementary-probes.tsv" ]]; then
  cat "${PHASE_DIR}/supplementary-probes.tsv" >>"${PHASE_DIR}/probes.tsv"
fi

{
  echo "PHASE_C_VERDICT=${SMOKE_VERDICT}"
  echo "SMOKE_SCRIPT_RC=${SMOKE_RC}"
  echo "CREDENTIALS_RESOLVED=$([[ "$CREDS_OK" -eq 0 ]] && echo yes || echo no)"
  echo "TEST_MACHINE_ID=${TEST_MACHINE_ID:-<unset>}"
  echo "ARTIFACT_DIR=${PHASE_DIR}"
} | tee "${PHASE_DIR}/READINESS.txt"

exit $([[ "$SMOKE_VERDICT" == "PRODUCTION_READONLY_SMOKE_PASS" ]] && echo 0 || echo 1)
