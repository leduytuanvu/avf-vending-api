#!/usr/bin/env bash
# shellcheck shell=bash
# Emit a Postman environment JSON on stdout from E2E test-data + secrets.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_SCRIPT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

OUT_PATH=""
DATA_ROOT=""

usage() {
  cat <<EOF
Usage: ./tests/e2e/postman/generate-local-env.sh [--out path] [RUN_DIR]

  Reads test-data.json + secrets.private.json from RUN_DIR or \$E2E_RUN_DIR.

  Variables set (Postman env format):
    base_url, admin_token, machine_token, organization_id, site_id, machine_id,
    product_id, order_id, slot_id (from slotCode), allow_production_writes,
    plus allow_mutation / allow_production_mutation / confirm_production_run for collection prerequest scripts.

EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out) OUT_PATH="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) DATA_ROOT="$1"; shift ;;
  esac
done

: "${BASE_URL:=http://127.0.0.1:8080}"
: "${E2E_TARGET:=local}"
: "${E2E_ALLOW_WRITES:=true}"

root="${DATA_ROOT:-${E2E_RUN_DIR:-}}"
if [[ -z "$root" ]] || [[ ! -d "$root" ]]; then
  echo "FATAL: set E2E_RUN_DIR or pass run directory containing test-data.json" >&2
  exit 2
fi

TD="${root}/test-data.json"
SEC="${root}/secrets.private.json"
[[ -f "$TD" ]] || { echo "FATAL: missing ${TD}" >&2; exit 2; }
[[ -f "$SEC" ]] || echo '{}' >"$SEC"

jq_get() { jq -r --arg k "$1" '.[$k] // empty' "$TD"; }
sec_get() { jq -r --arg k "$1" '.[$k] // empty' "$SEC"; }

org="$(jq_get organizationId)"
site="$(jq_get siteId)"
mid="$(jq_get machineId)"
pid="$(jq_get productId)"
slot="$(jq_get slotCode)"

admin_env="${ADMIN_TOKEN:-}"
[[ -z "$admin_env" ]] && admin_env="$(sec_get adminAccessToken)"
[[ -z "$admin_env" ]] && admin_env="$(sec_get admin_token)"

mt="$(sec_get machineToken)"
[[ -z "$mt" ]] && mt="$(sec_get machine_token)"

oid="$(jq_get vmCashSuccessOrderId)"
[[ -z "$oid" || "$oid" == "null" ]] && oid="$(jq_get orderId)"

allow_prod_writes="false"
confirm=""
if [[ "${E2E_TARGET}" == "production" ]] && [[ "${E2E_ALLOW_WRITES}" == "true" ]] && [[ "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" == "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" ]]; then
  allow_prod_writes="true"
  confirm="I_UNDERSTAND_PRODUCTION_MUTATION"
fi

# Collection uses app_env + payment_env heuristics in prerequest; keep local/staging sane.
app_env="development"
[[ "${E2E_TARGET}" == "production" ]] && app_env="production"

writes="${E2E_ALLOW_WRITES:-true}"
body="$(jq -nc \
  --arg base "${BASE_URL}" \
  --arg adm "$admin_env" \
  --arg mt "$mt" \
  --arg org "$org" \
  --arg site "$site" \
  --arg mid "$mid" \
  --arg pid "$pid" \
  --arg oid "$oid" \
  --arg slot "$slot" \
  --arg apw "$allow_prod_writes" \
  --arg writes "$writes" \
  --arg confirm "$confirm" \
  --arg app_env "$app_env" \
  '{
    id: "avf-env-generated-local",
    name: "AVF Generated (E2E)",
    values: [
      {key:"env_name", value:"generated", type:"default", enabled:true},
      {key:"app_env", value:$app_env, type:"default", enabled:true},
      {key:"auth_type", value:"public", type:"default", enabled:true},
      {key:"base_url", value:$base, type:"default", enabled:true},
      {key:"api_prefix", value:"/v1", type:"default", enabled:true},
      {key:"swagger_url", value:($base + "/swagger/doc.json"), type:"default", enabled:true},
      {key:"payment_env", value:(if $app_env == "production" then "live" else "sandbox" end), type:"default", enabled:true},
      {key:"mqtt_topic_prefix", value:(if $app_env == "production" then "avf/devices" else "avf-dev/devices" end), type:"default", enabled:true},
      {key:"allow_mutation", value:$writes, type:"default", enabled:true},
      {key:"allow_production_mutation", value:$apw, type:"default", enabled:true},
      {key:"confirm_production_run", value:$confirm, type:"default", enabled:true},
      {key:"allow_production_writes", value:$apw, type:"default", enabled:true},
      {key:"admin_token", value:$adm, type:"secret", enabled:true},
      {key:"machine_token", value:$mt, type:"secret", enabled:true},
      {key:"organization_id", value:$org, type:"default", enabled:true},
      {key:"site_id", value:$site, type:"default", enabled:true},
      {key:"machine_id", value:$mid, type:"default", enabled:true},
      {key:"product_id", value:$pid, type:"default", enabled:true},
      {key:"order_id", value:$oid, type:"default", enabled:true},
      {key:"slot_id", value:$slot, type:"default", enabled:true}
    ]
  }')"

if [[ -n "$OUT_PATH" ]]; then
  printf '%s\n' "$body" >"$OUT_PATH"
else
  printf '%s\n' "$body"
fi
