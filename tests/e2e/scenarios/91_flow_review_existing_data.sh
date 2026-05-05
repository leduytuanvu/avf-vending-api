#!/usr/bin/env bash
# shellcheck shell=bash
# E2E-FR-91: Read-only probes using reused test-data (no mutations).

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_flow_review.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_flow_review.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_grpc.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_grpc.sh"

FLOW_ID="E2E-FR-91"
SCEN="$(basename "${BASH_SOURCE[0]}")"
IMP="$(e2e_improvement_findings_path)"
IMP_BEFORE="$(wc -l <"${IMP}" 2>/dev/null | tr -d ' ')"
[[ -z "${IMP_BEFORE}" ]] && IMP_BEFORE=0

start_step "flow-review-existing-data"

LIVE_OUT="${E2E_RUN_DIR}/reports/flow-review-live.json"
TD="${E2E_RUN_DIR}/test-data.json"
mkdir -p "${E2E_RUN_DIR}/reports" "${E2E_RUN_DIR}/rest" "${E2E_RUN_DIR}/grpc"

ORG="$(jq -r '.organizationId // empty' "$TD" 2>/dev/null)"
SITE="$(jq -r '.siteId // empty' "$TD" 2>/dev/null)"
MID="$(jq -r '.machineId // empty' "$TD" 2>/dev/null)"
PID="$(jq -r '.productId // empty' "$TD" 2>/dev/null)"
[[ -z "${PID:-}" || "$PID" == "null" ]] && PID="$(jq -r '.productIds[0] // empty' "$TD" 2>/dev/null)"
SLOT="$(jq -r '.slotCode // empty' "$TD" 2>/dev/null)"
[[ -z "${SLOT:-}" || "$SLOT" == "null" ]] && SLOT="$(jq -r '.slotCodes[0] // .slotIds[0] // empty' "$TD" 2>/dev/null)"

check_field() {
  local key="$1" val="$2"
  if [[ -z "${val:-}" || "$val" == "null" ]]; then
    log_missing_field_issue "P1" "$FLOW_ID" "$SCEN" "reuse-${key}" "data" "test-data.json" \
      "Reused test-data missing non-empty ${key} — read-only flow review cannot validate downstream API surfaces" \
      "Incomplete coverage signal against real env" \
      "Regenerate capture or hand-edit ${key}" \
      "$TD"
    return 1
  fi
  return 0
}

check_field "organizationId" "${ORG}" || true
check_field "siteId" "${SITE}" || true
check_field "machineId" "${MID}" || true
check_field "productId" "${PID}" || true
check_field "slotCode" "${SLOT}" || true

MT=""
MT="$(get_secret machineToken 2>/dev/null || true)"
if [[ -z "${MT:-}" ]] && [[ -n "${MACHINE_TOKEN:-}" ]]; then
  MT="${MACHINE_TOKEN}"
fi
if [[ -z "${MT:-}" ]]; then
  log_data_setup_issue "P2" "$FLOW_ID" "$SCEN" "machine-token" "secrets" "machineToken" \
    "No machine JWT in secrets.private.json, MACHINE_TOKEN env, or usable test-data secret — machine-scoped read probes skipped" \
    "Cannot validate sale-catalog or gRPC GetBootstrap in this pass" \
    "Populate secrets.private.json with machineToken or export MACHINE_TOKEN" \
    "${E2E_RUN_DIR}/secrets.private.json"
fi

if [[ "$MT" == *"*"* ]] || [[ "$MT" == "<"* ]]; then
  log_data_setup_issue "P2" "$FLOW_ID" "$SCEN" "machine-token-masked" "data" "machineToken" \
    "Machine token appears masked/placeholder in this run — real JWT required for authenticated read probes" \
    "Sale-catalog / gRPC calls will likely 401" \
    "Use secrets.private.json or env with raw JWT" \
    "$TD"
  MT=""
fi

# --- Public read-only REST ---
e2e_http_get_capture "fr91-health-live" "/health/live" "required" "false" || true
e2e_http_get_capture "fr91-version" "/version" "required" "false" || true

# --- Machine sale-catalog (read-only) ---
if [[ -n "${MT:-}" ]] && [[ -n "${MID:-}" ]] && [[ "${MID}" != "null" ]]; then
  _path="/v1/machines/${MID}/sale-catalog?include_images=true"
  _dir="$(e2e_http_log_dir)"
  mkdir -p "$_dir"
  _url="${BASE_URL%/}${_path}"
  _hdr="${_dir}/fr91-sale-cat.response.headers.txt"
  _body="${_dir}/fr91-sale-cat.response.body"
  _out="$(curl -sS -L --max-redirs 3 -D "$_hdr" -o "$_body" -w '%{http_code}|%{time_total}' \
    -H "Authorization: Bearer ${MT}" "$_url" 2>/dev/null || true)"
  _code="${_out%%|*}"
  if [[ "${_code}" != "200" ]]; then
    log_observability_issue "P2" "$FLOW_ID" "$SCEN" "sale-catalog-readonly" "REST" "${_path}" \
      "Read-only sale-catalog GET returned HTTP ${_code} — correlation/product visibility not verified" \
      "Kiosk REST parity harder to assert remotely" \
      "Verify token scopes; ensure planogram published for machine" \
      "${_dir}/fr91-sale-cat.response.body"
  fi
  append_event_jsonl "http:fr91-sale-cat" "passed" "GET ${_path} HTTP ${_code}"
else
  append_event_jsonl "http:fr91-sale-cat" "skipped" "no machine token"
fi

# --- Optional admin read (inventory topology) ---
if [[ -n "${ADMIN_TOKEN:-}" ]] && [[ -n "${MID:-}" ]] && [[ -n "${ORG:-}" ]]; then
  e2e_http_get_capture "fr91-admin-slots" "/v1/admin/organizations/${ORG}/machines/${MID}/slots" "optional404" "true" || true
else
  log_observability_issue "P3" "$FLOW_ID" "$SCEN" "admin-read-optional" "REST" "/v1/admin/.../slots" \
    "ADMIN_TOKEN not set — admin read-only inventory topology not exercised in flow review" \
    "Narrower observability vs full stack" \
    "Export ADMIN_TOKEN for lab read-only token if appropriate" \
    "$TD"
fi

# --- gRPC GetBootstrap (read-only) ---
grpc_ok=""
if [[ -n "${MT:-}" ]] && [[ -n "${MID:-}" ]] && command -v grpcurl >/dev/null 2>&1 && e2e_grpc_tcp_open; then
  export MACHINE_TOKEN="$MT"
  export MACHINE_ID="$MID"
  META="$(jq -nc --arg o "${ORG:-}" --arg m "$MID" --arg rid "fr91-$(date +%s)" '{organizationId:$o, machineId:$m, requestId:$rid}')"
  BOOT_BODY="$(jq -nc --argjson meta "$META" '{meta:$meta}')"
  if e2e_grpc_call "avf.machine.v1.MachineBootstrapService/GetBootstrap" "$BOOT_BODY" "fr91-bootstrap" ""; then
    grpc_ok="true"
  else
    log_observability_issue "P2" "$FLOW_ID" "$SCEN" "grpc-bootstrap-readonly" "gRPC" "GetBootstrap" \
      "GetBootstrap failed in read-only flow review — vending bootstrap contract not verified live" \
      "Field parity / config hints (MQTT topic roots) opaque" \
      "Check GRPC_ADDR, TLS/plaintext, metadata, and credentials" \
      "${E2E_RUN_DIR}/grpc/fr91-bootstrap.meta.json"
  fi
else
  if ! command -v grpcurl >/dev/null 2>&1; then
    log_docs_issue "P3" "$FLOW_ID" "$SCEN" "grpcurl-missing" "gRPC" "grpcurl" \
      "grpcurl not installed — skipping live gRPC read probe" \
      "Weaker live vending contract signal" \
      "Install grpcurl for Phase 6 parity" \
      "${LIVE_OUT}"
  elif ! e2e_grpc_tcp_open; then
    log_observability_issue "P2" "$FLOW_ID" "$SCEN" "grpc-down" "gRPC" "${GRPC_ADDR:-}" \
      "gRPC port not reachable from harness — bootstrap read skipped" \
      "Cannot correlate REST test data with machine stream" \
      "Start listener or point GRPC_ADDR at env" \
      "${LIVE_OUT}"
  fi
fi

# --- MQTT credentials hint (no broker connects here) ---
if jq -e '.mqtt.username // .mqttUsername // empty' "$TD" >/dev/null 2>&1; then
  :
else
  if [[ -n "${MQTT_USERNAME:-}" ]]; then
    :
  else
    log_observability_issue "P3" "$FLOW_ID" "$SCEN" "mqtt-creds-data" "MQTT" "—" \
      "test-data.json has no mqtt username field and MQTT_USERNAME env unset — broker auth readiness not visible" \
      "Operator cannot tell from data alone if Phase 7 creds are wired" \
      "Document mqtt block in reusable captures" \
      "$TD"
  fi
fi

jq -nc \
  --arg gen "$(now_utc)" \
  --arg mode "reuse-data" \
  --arg org "${ORG:-}" \
  --arg mid "${MID:-}" \
  --arg grpc "${grpc_ok:-false}" \
  --argjson has_token "$([[ -n "${MT:-}" ]] && echo true || echo false)" \
  '{
    generatedAt:$gen,
    mode:$mode,
    organizationIdPresent:($org != ""),
    machineIdPresent:($mid != ""),
    machineJwtPresent:$has_token,
    grpcBootstrapProbed:($grpc|test("true"))
  }' >"${LIVE_OUT}"

end_step passed "flow-review existing-data pass"

IMP_AFTER="$(wc -l <"${IMP}" 2>/dev/null | tr -d ' ')"
[[ -z "${IMP_AFTER}" ]] && IMP_AFTER=0
if [[ "${IMP_AFTER}" -eq "${IMP_BEFORE}" ]]; then
  log_no_improvement_findings "$FLOW_ID" "$SCEN" "existing-data-clean"
fi

e2e_flow_review_scenario_complete "$FLOW_ID" "$SCEN" "flow-review-live" "existing_data_review_pass"

exit 0
