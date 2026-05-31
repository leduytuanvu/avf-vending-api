#!/usr/bin/env bash
# shellcheck shell=bash
# Safe production read-only smoke — no orders, vends, or payment mutations.
#
# Usage:
#   BASE_URL=https://api.ldtv.dev \
#   GRPC_ADDR=machine-api.ldtv.dev:443 \
#   GRPC_USE_PLAINTEXT=false \
#   MACHINE_ACCESS_TOKEN=<test machine JWT> \
#   TEST_MACHINE_ID=<uuid> \
#   MACHINE_REFRESH_TOKEN=<optional refresh token> \
#   ADMIN_EMAIL=... ADMIN_PASSWORD=... \
#   bash scripts/e2e/production-readonly-smoke.sh
#
# Strict production canary gate (default true for https://api.ldtv.dev):
#   PRODUCTION_SMOKE_STRICT_CANARY=true
#
# Optional MQTT config validation (no publish):
#   MQTT_BROKER_URL=ssl://... MQTT_TOPIC_PREFIX=avf/devices

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"
# shellcheck source=lib/readonly-smoke-verdict.sh
source "${ROOT}/scripts/e2e/lib/readonly-smoke-verdict.sh"

e2e_require_cmd curl jq grpcurl
e2e_init_run_dir "production-readonly-smoke"

BASE_URL="${BASE_URL:-${PRODUCTION_BASE_URL:-https://api.ldtv.dev}}"
BASE_URL="${BASE_URL%/}"
export BASE_URL

PRODUCTION_SMOKE_STRICT_CANARY="$(e2e_strict_canary_default_for_base_url "$BASE_URL")"
export PRODUCTION_SMOKE_STRICT_CANARY

: >"${E2E_RUN_DIR}/probes.tsv"
FAILURES=0
CASH_ONLY_OK=0

pass_probe() { e2e_record_probe "$1" "PASS" "${2:-}" "${3:-0}"; echo "PASS  $1 ${2:+($2)}"; }
fail_probe() { e2e_record_probe "$1" "FAIL" "${2:-}" "${3:-0}"; echo "FAIL  $1 ${2:-}"; FAILURES=$((FAILURES + 1)); }
skip_probe() { e2e_record_probe "$1" "SKIP" "${2:-}" "0"; echo "SKIP  $1 ${2:-}"; }

skip_or_fail_strict() {
  local name="$1"
  local reason="$2"
  local lat="${3:-0}"
  if [[ "$PRODUCTION_SMOKE_STRICT_CANARY" == "true" ]] && e2e_is_strict_canary_probe "$name"; then
    fail_probe "$name" "strict canary required: ${reason}" "$lat"
  else
    skip_probe "$name" "$reason"
  fi
}

echo "== Production read-only smoke =="
echo "base_url=${BASE_URL}"
echo "strict_canary=${PRODUCTION_SMOKE_STRICT_CANARY}"
echo "run_dir=${E2E_RUN_DIR}"

for path in /health/live /health/ready /version; do
  url="${BASE_URL}${path}"
  read -r code lat < <(e2e_curl_get "http${path//\//-}" "$url" "")
  if [[ "$code" == "200" ]]; then
    pass_probe "http${path}" "status=${code}" "$lat"
  else
    fail_probe "http${path}" "expected 200 got ${code}" "$lat"
  fi
done

# Payment runtime visibility (read-only) — required for any GO-CANARY-ONLY claim on deployed API.
PAYMENT_RUNTIME_PROBE="SKIP"
VERSION_BODY="${E2E_RUN_DIR}/raw/version-payment-runtime.body"
url="${BASE_URL}/version"
read -r code lat < <(e2e_curl_get "version-payment-runtime" "$url" "")
if [[ "$code" == "200" ]]; then
  if jq -e '.payment_runtime // .paymentRuntime' "$VERSION_BODY" >/dev/null 2>&1; then
    mode="$(jq -r '.payment_runtime.payment_mode // .paymentRuntime.paymentMode // empty' "$VERSION_BODY")"
    if [[ -n "$mode" ]]; then
      pass_probe "version.payment_runtime" "payment_mode=${mode}" "$lat"
      PAYMENT_RUNTIME_PROBE="PASS"
      if cash_detail="$(e2e_validate_cash_only_payment_runtime "$VERSION_BODY" "$PRODUCTION_SMOKE_STRICT_CANARY" 2>&1)"; then
        pass_probe "version.payment_runtime.cash_only_contract" "cash-only contract ok"
      else
        fail_probe "version.payment_runtime.cash_only_contract" "${cash_detail}"
        PAYMENT_RUNTIME_PROBE="FAIL"
      fi
    else
      fail_probe "version.payment_runtime" "payment_runtime present but payment_mode empty" "$lat"
      PAYMENT_RUNTIME_PROBE="FAIL"
    fi
  elif [[ "$PRODUCTION_SMOKE_STRICT_CANARY" == "true" ]]; then
    fail_probe "version.payment_runtime" "payment_runtime absent on deployed production (strict canary)" "$lat"
    PAYMENT_RUNTIME_PROBE="FAIL"
  else
    skip_probe "version.payment_runtime" "field absent on this deployment"
  fi
else
  fail_probe "version.payment_runtime" "version unreachable" "$lat"
  PAYMENT_RUNTIME_PROBE="FAIL"
fi

ADMIN_TOK=""
if ADMIN_TOK="$(e2e_admin_token 2>/dev/null)"; then
  pass_probe "admin.auth" "token acquired"
  for admin_path in "/v1/admin/machines?limit=3" "/v1/admin/payment/providers"; do
    read -r code lat < <(e2e_curl_get "admin${admin_path//[\/?=&]/-}" "${BASE_URL}${admin_path}" "$ADMIN_TOK")
    if [[ "$code" == "200" ]]; then
      pass_probe "admin${admin_path}" "status=${code}" "$lat"
    else
      fail_probe "admin${admin_path}" "expected 200 got ${code}" "$lat"
    fi
  done
else
  skip_or_fail_strict "admin.auth" "ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD not set"
fi

if [[ -n "${GRPC_ADDR:-${GRPC_TARGET:-}}" ]]; then
  export GRPC_ADDR="${GRPC_ADDR:-${GRPC_TARGET}}"

  # Activation dry-run: invalid code must not activate a machine.
  invalid_claim="$(jq -nc '{activationCode:"E2E_READONLY_INVALID_DO_NOT_ACTIVATE",deviceFingerprint:{androidId:"e2e-readonly",serialNumber:"readonly-smoke",manufacturer:"avf",model:"harness",packageName:"dev.avf.e2e",versionName:"1.0.0",versionCode:1}}')"
  if e2e_grpc_call "avf.machine.v1.MachineActivationService/ClaimActivation" "$invalid_claim" "activation-dry-run" none ""; then
    mid="$(jq -r '.machineId // empty' "${E2E_RUN_DIR}/raw/activation-dry-run.response.json" 2>/dev/null || true)"
    if [[ -n "$mid" ]]; then
      fail_probe "grpc.activation_dry_run" "unexpected success — would have activated machine ${mid}" "${E2E_GRPC_LAST_LAT:-0}"
    else
      pass_probe "grpc.activation_dry_run" "empty success body" "${E2E_GRPC_LAST_LAT:-0}"
    fi
  else
    pass_probe "grpc.activation_dry_run" "rejected invalid activation code (expected)" "${E2E_GRPC_LAST_LAT:-0}"
  fi

  if [[ -n "${MACHINE_REFRESH_TOKEN:-}" ]]; then
    refresh_body="$(jq -nc --arg rt "${MACHINE_REFRESH_TOKEN}" '{refreshToken:$rt}')"
    if e2e_grpc_call "avf.machine.v1.MachineTokenService/RefreshMachineToken" "$refresh_body" "token-refresh" none ""; then
      new_tok="$(jq -r '.accessToken // empty' "${E2E_RUN_DIR}/raw/token-refresh.response.json")"
      if [[ -n "$new_tok" ]]; then
        pass_probe "grpc.token_refresh" "access token returned" "${E2E_GRPC_LAST_LAT:-0}"
        MACHINE_ACCESS_TOKEN="$new_tok"
        export MACHINE_ACCESS_TOKEN
      else
        fail_probe "grpc.token_refresh" "missing accessToken in response" "${E2E_GRPC_LAST_LAT:-0}"
      fi
    else
      fail_probe "grpc.token_refresh" "RefreshMachineToken failed rc=${E2E_GRPC_LAST_RC:-1}" "${E2E_GRPC_LAST_LAT:-0}"
    fi
  else
    skip_probe "grpc.token_refresh" "MACHINE_REFRESH_TOKEN unset"
  fi

  if [[ -n "${MACHINE_ACCESS_TOKEN:-}" && -n "${TEST_MACHINE_ID:-}" ]]; then
    meta="$(jq -nc --arg mid "${TEST_MACHINE_ID}" --arg rid "e2e-ro-${E2E_RUN_TS}" '{machineId:$mid,requestId:$rid}')"
    gb_body="$(jq -nc --argjson meta "$meta" '{meta:$meta}')"
    if e2e_grpc_call "avf.machine.v1.MachineBootstrapService/GetBootstrap" "$gb_body" "grpc-bootstrap" machine ""; then
      pass_probe "grpc.bootstrap" "ok" "${E2E_GRPC_LAST_LAT:-0}"
      mqtt_url="$(jq -r '.mqtt.brokerUrl // .mqtt.broker_url // empty' "${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json")"
      mqtt_prefix="$(jq -r '.mqtt.topicPrefix // .mqtt.topic_prefix // empty' "${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json")"
      if [[ -n "$mqtt_url" && -n "$mqtt_prefix" ]]; then
        pass_probe "grpc.bootstrap.mqtt_config" "broker+prefix present"
        if [[ -n "${MQTT_BROKER_URL:-}" && "$MQTT_BROKER_URL" != "$mqtt_url" ]]; then
          fail_probe "mqtt.config_match" "bootstrap broker ${mqtt_url} != env MQTT_BROKER_URL"
        else
          pass_probe "mqtt.config_match" "bootstrap MQTT config present"
        fi
        mqtt_layout="$(jq -r '.mqtt.topicLayout // .mqtt.topic_layout // empty' "${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json")"
        if [[ -n "$mqtt_layout" ]]; then
          pass_probe "mqtt.topic_layout" "layout=${mqtt_layout}"
        else
          skip_probe "mqtt.topic_layout" "field absent on bootstrap"
        fi
        mqtt_tls="$(jq -r '.mqtt.tlsRequired // .mqtt.tls_required // empty' "${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json")"
        if [[ -n "$mqtt_tls" && "$mqtt_tls" != "null" ]]; then
          pass_probe "mqtt.tls_required" "tls_required=${mqtt_tls}"
        else
          skip_probe "mqtt.tls_required" "field absent on bootstrap"
        fi
        mqtt_cid="$(jq -r '.mqtt.clientIdPolicy // .mqtt.client_id_policy // empty' "${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json")"
        if [[ -n "$mqtt_cid" ]]; then
          pass_probe "mqtt.client_id_policy" "policy=${mqtt_cid}"
        else
          skip_probe "mqtt.client_id_policy" "field absent on bootstrap"
        fi
      else
        fail_probe "grpc.bootstrap.mqtt_config" "missing mqtt block"
      fi
      pm="$(jq -r '.paymentMethods // .payment_methods // empty' "${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json")"
      if [[ -n "$pm" && "$pm" != "null" ]]; then
        pass_probe "grpc.bootstrap.payment_methods" "$(jq -c '.paymentMethods // .payment_methods' "${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json")"
      else
        skip_or_fail_strict "grpc.bootstrap.payment_methods" "field absent"
      fi
    else
      fail_probe "grpc.bootstrap" "GetBootstrap failed" "${E2E_GRPC_LAST_LAT:-0}"
    fi

    cat_body="$(jq -nc --arg mid "${TEST_MACHINE_ID}" --argjson meta "$meta" '{machineId:$mid,includeUnavailable:false,meta:$meta}')"
    media_delta_body="$(jq -nc --arg mid "${TEST_MACHINE_ID}" --argjson meta "$meta" '{machineId:$mid,basisMediaFingerprint:"",meta:$meta}')"
    inv_body="$(jq -nc --argjson meta "$meta" '{meta:$meta}')"

    if e2e_grpc_call "avf.machine.v1.MachineCatalogService/GetCatalogSnapshot" "$cat_body" "grpc-catalog" machine ""; then
      pass_probe "grpc.catalog" "ok" "${E2E_GRPC_LAST_LAT:-0}"
    else
      fail_probe "grpc.catalog" "rpc failed" "${E2E_GRPC_LAST_LAT:-0}"
    fi
    if e2e_grpc_call "avf.machine.v1.MachineMediaService/GetMediaManifest" "$cat_body" "grpc-media-manifest" machine ""; then
      pass_probe "grpc.media_manifest" "ok" "${E2E_GRPC_LAST_LAT:-0}"
    else
      fail_probe "grpc.media_manifest" "rpc failed" "${E2E_GRPC_LAST_LAT:-0}"
    fi
    if e2e_grpc_call "avf.machine.v1.MachineMediaService/GetMediaDelta" "$media_delta_body" "grpc-media-delta" machine ""; then
      pass_probe "grpc.media_delta" "ok" "${E2E_GRPC_LAST_LAT:-0}"
    else
      fail_probe "grpc.media_delta" "rpc failed" "${E2E_GRPC_LAST_LAT:-0}"
    fi
    if e2e_grpc_call "avf.machine.v1.MachineInventoryService/GetInventorySnapshot" "$inv_body" "grpc-inventory" machine ""; then
      pass_probe "grpc.inventory" "ok" "${E2E_GRPC_LAST_LAT:-0}"
    else
      fail_probe "grpc.inventory" "rpc failed" "${E2E_GRPC_LAST_LAT:-0}"
    fi
    plano_body="$(jq -nc --argjson meta "$meta" '{meta:$meta}')"
    if e2e_grpc_call "avf.machine.v1.MachineInventoryService/GetPlanogram" "$plano_body" "grpc-planogram" machine ""; then
      pass_probe "grpc.planogram" "ok" "${E2E_GRPC_LAST_LAT:-0}"
    else
      fail_probe "grpc.planogram" "rpc failed" "${E2E_GRPC_LAST_LAT:-0}"
    fi

    # Safety contract: this harness must never invoke commerce or command mutations.
    pass_probe "safety.no_create_order" "not invoked"
    pass_probe "safety.no_payment_session" "not invoked"
    pass_probe "safety.no_cash_confirm" "not invoked"
    pass_probe "safety.no_start_vend" "not invoked"
    pass_probe "safety.no_mqtt_command_publish" "not invoked"

    # Test-machine-only telemetry heartbeat (non-commerce write).
    if [[ "${ALLOW_TEST_MACHINE_TELEMETRY_PUSH:-true}" == "true" ]]; then
      tel_idem="e2e-readonly-telemetry-${E2E_RUN_TS}"
      tel_ts="$(e2e_now_utc)"
      tel_ctx="$(jq -nc --arg ik "$tel_idem" --arg ce "$tel_idem-ce" --arg ts "$tel_ts" '{idempotencyKey:$ik,clientEventId:$ce,clientCreatedAt:$ts}')"
      tel_body="$(jq -nc --argjson ctx "$tel_ctx" --argjson meta "$meta" --arg eid "$tel_idem" --arg ts "$tel_ts" \
        '{context:$ctx, meta:$meta, events:[{eventId:$eid,eventType:"production_e2e_smoke_heartbeat",occurredAt:$ts}]}')"
      if e2e_grpc_call "avf.machine.v1.MachineTelemetryService/PushTelemetryBatch" "$tel_body" "grpc-telemetry-smoke" machine "$tel_idem"; then
        pass_probe "grpc.telemetry_smoke" "test-machine heartbeat accepted" "${E2E_GRPC_LAST_LAT:-0}"
      else
        fail_probe "grpc.telemetry_smoke" "PushTelemetryBatch failed" "${E2E_GRPC_LAST_LAT:-0}"
      fi
    else
      skip_probe "grpc.telemetry_smoke" "ALLOW_TEST_MACHINE_TELEMETRY_PUSH=false"
    fi
  else
    if [[ "$PRODUCTION_SMOKE_STRICT_CANARY" == "true" ]]; then
      skip_or_fail_strict "grpc.bootstrap" "MACHINE_ACCESS_TOKEN and TEST_MACHINE_ID required"
      skip_or_fail_strict "grpc.bootstrap.payment_methods" "MACHINE_ACCESS_TOKEN and TEST_MACHINE_ID required"
      skip_or_fail_strict "grpc.catalog" "MACHINE_ACCESS_TOKEN and TEST_MACHINE_ID required"
      skip_or_fail_strict "grpc.media_manifest" "MACHINE_ACCESS_TOKEN and TEST_MACHINE_ID required"
      skip_or_fail_strict "grpc.inventory" "MACHINE_ACCESS_TOKEN and TEST_MACHINE_ID required"
      skip_or_fail_strict "grpc.planogram" "MACHINE_ACCESS_TOKEN and TEST_MACHINE_ID required"
    else
      skip_probe "grpc.machine_runtime" "MACHINE_ACCESS_TOKEN and TEST_MACHINE_ID required"
    fi
  fi
else
  if [[ "$PRODUCTION_SMOKE_STRICT_CANARY" == "true" ]]; then
    skip_or_fail_strict "grpc.bootstrap" "GRPC_ADDR / GRPC_TARGET unset"
    skip_or_fail_strict "grpc.bootstrap.payment_methods" "GRPC_ADDR / GRPC_TARGET unset"
    skip_or_fail_strict "grpc.catalog" "GRPC_ADDR / GRPC_TARGET unset"
    skip_or_fail_strict "grpc.media_manifest" "GRPC_ADDR / GRPC_TARGET unset"
    skip_or_fail_strict "grpc.inventory" "GRPC_ADDR / GRPC_TARGET unset"
    skip_or_fail_strict "grpc.planogram" "GRPC_ADDR / GRPC_TARGET unset"
  else
    skip_probe "grpc" "GRPC_ADDR / GRPC_TARGET unset"
  fi
fi

if [[ "$PAYMENT_RUNTIME_PROBE" == "PASS" ]]; then
  if e2e_validate_cash_only_payment_runtime "$VERSION_BODY" "$PRODUCTION_SMOKE_STRICT_CANARY" >/dev/null 2>&1; then
    CASH_ONLY_OK=0
  else
    CASH_ONLY_OK=1
  fi
else
  CASH_ONLY_OK=1
fi

e2e_compute_readonly_smoke_verdicts "$PRODUCTION_SMOKE_STRICT_CANARY" "${E2E_RUN_DIR}/probes.tsv" "$FAILURES" "$CASH_ONLY_OK"
SMOKE_VERDICT="${E2E_SMOKE_VERDICT}"
READINESS_VERDICT="${E2E_READINESS_VERDICT}"
EXIT_CODE="${E2E_EXIT_CODE}"

{
  echo ""
  echo "== Readiness =="
  echo "READINESS_VERDICT=${READINESS_VERDICT}"
  echo "SMOKE_VERDICT=${SMOKE_VERDICT}"
  echo "STRICT_CANARY=${PRODUCTION_SMOKE_STRICT_CANARY}"
} | tee "${E2E_RUN_DIR}/READINESS.txt"

e2e_finalize_report "production-readonly-smoke" "$SMOKE_VERDICT" "$EXIT_CODE" "$READINESS_VERDICT" "$PRODUCTION_SMOKE_STRICT_CANARY"
exit "$EXIT_CODE"
