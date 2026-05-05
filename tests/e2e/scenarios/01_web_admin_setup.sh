#!/usr/bin/env bash
# shellcheck shell=bash
# Web Admin setup: auth, org-scoped tenant entities, catalog, planogram+inventory (when planogram + operator session allow).
# Canonical paths per docs/swagger/swagger.json and Postman/OpenAPI.

set -euo pipefail

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${E2E_SCRIPT_DIR:=$(cd "${E2E_SCENARIO_DIR}/.." && pwd)}"
: "${E2E_REPO_ROOT:=$(cd "${E2E_SCRIPT_DIR}/../.." && pwd)}"

# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

FLOW_ID="WA-SETUP-01"
SEED_FILE="${E2E_SEED_FILE:-${E2E_REPO_ROOT}/tests/e2e/data/seed.local.example.json}"
TS="$(date +%s)-${RANDOM}"
SUFFIX="${TS}"

wa_ev() {
  local step="$1"
  local endpoint="$2"
  local status="$3"
  local msg="$4"
  local ids="${5:-{}}"
  e2e_append_test_event "$FLOW_ID" "$step" "REST" "$endpoint" "$status" "$msg" "$ids"
}

wa_fail() {
  local step="$1"
  local endpoint="$2"
  local msg="$3"
  wa_ev "$step" "$endpoint" "fail" "$msg" "{}"
  log_error "$msg"
  exit 1
}

if ! command -v jq >/dev/null 2>&1; then
  echo "FATAL: jq required" >&2
  exit 127
fi

if [[ ! -f "$SEED_FILE" ]]; then
  wa_fail "seed" "—" "Seed JSON not found: ${SEED_FILE}"
fi

if [[ "${E2E_ALLOW_WRITES:-false}" != "true" ]]; then
  wa_ev "safety" "—" "skip" "Set E2E_ALLOW_WRITES=true to run admin mutations" "{}"
  log_error "E2E_ALLOW_WRITES must be true for web admin setup (local/staging writes)."
  exit 2
fi

# load_env + e2e_target_safety_guard already ran in runner; production writes still blocked without confirmation.

ORG_NAME=$(jq -r '.organization.name // "E2E Organization"' "$SEED_FILE")
SITE_NAME_SEED=$(jq -r '.site.name // "E2E Site"' "$SEED_FILE")
SITE_TZ=$(jq -r '.organization.timezone // "UTC"' "$SEED_FILE")
CABINET=$(jq -r '.planogram.cabinet_code // "A"' "$SEED_FILE")
SLOT=$(jq -r '.planogram.slots[0].slot_code // "A1"' "$SEED_FILE")
PRICE_MINOR=$(jq -r '.planogram.slots[0].price_minor // 15000' "$SEED_FILE")
INV_QTY=$(jq -r '.planogram.slots[0].initial_quantity // 8' "$SEED_FILE")
CUR=$(jq -r '.planogram.slots[0].currency // "VND"' "$SEED_FILE")
MACHINE_MODEL=$(jq -r '.machine.model // "AVF-E2E"' "$SEED_FILE")

e2e_set_data slotCode "$SLOT"
e2e_set_data slotCabinetCode "$CABINET"
e2e_set_data currency "$CUR"
e2e_set_data seedSuffix "$SUFFIX"

# --- Auth ---
ORG_ID=""
if [[ -n "${ADMIN_TOKEN:-}" ]] && [[ -n "${ADMIN_TOKEN// }" ]]; then
  export ADMIN_TOKEN
  wa_ev "auth" "ADMIN_TOKEN env" "pass" "Using pre-configured ADMIN_TOKEN" "{}"
  if [[ "${E2E_REUSE_DATA:-false}" == "true" ]] && [[ -n "$(get_data organizationId)" ]] && [[ "$(get_data organizationId)" != "null" ]]; then
    ORG_ID="$(get_data organizationId)"
  else
    ORG_ID="${E2E_ORGANIZATION_ID:-}"
  fi
  if [[ -z "$ORG_ID" ]]; then
    wa_fail "auth" "—" "ADMIN_TOKEN set but organizationId unknown: set E2E_ORGANIZATION_ID or use --reuse-data with test-data containing organizationId"
  fi
  e2e_set_data organizationId "$ORG_ID"
else
  if [[ -z "${ADMIN_EMAIL:-}" ]] || [[ -z "${ADMIN_PASSWORD:-}" ]] || [[ -z "${E2E_ORGANIZATION_ID:-}" ]]; then
    wa_fail "auth" "POST /v1/auth/login" "Need ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD+E2E_ORGANIZATION_ID (login body per OpenAPI). Local DB must contain this org and user — seed via make/migrations or ops scripts."
  fi
  LOGIN_JSON="$(jq -nc \
    --arg email "$ADMIN_EMAIL" \
    --arg password "$ADMIN_PASSWORD" \
    --arg oid "$E2E_ORGANIZATION_ID" \
    '{email:$email,password:$password,organizationId:$oid}')"
  code="$(e2e_http_post_json_anon "wa-login" "/v1/auth/login" "$LOGIN_JSON")"
  if ! e2e_http_assert_status "wa-login" "200" "$code"; then
    wa_fail "auth" "POST /v1/auth/login" "Login failed (HTTP $code). Check credentials, organizationId, and API logs in rest/wa-login.response.json."
  fi
  export ADMIN_TOKEN="$(e2e_jq_resp wa-login -r '.tokens.accessToken // empty')"
  if [[ -z "$ADMIN_TOKEN" ]]; then
    wa_fail "auth" "POST /v1/auth/login" "No accessToken in login response"
  fi
  save_token adminAccessToken "$ADMIN_TOKEN"
  ORG_ID="$(e2e_jq_resp wa-login -r '.organizationId // empty')"
  [[ -n "$ORG_ID" ]] || ORG_ID="$E2E_ORGANIZATION_ID"
  e2e_set_data organizationId "$ORG_ID"
  wa_ev "auth" "POST /v1/auth/login" "pass" "Obtained JWT" "$(jq -nc --arg o "$ORG_ID" '{organizationId:$o}')"
fi

Q_ORG="organization_id=$(printf '%s' "$ORG_ID" | jq -sRr @uri)"

# --- Site ---
SITE_ID=""
if [[ "${E2E_REUSE_DATA:-false}" == "true" ]] && [[ -n "$(get_data siteId)" ]] && [[ "$(get_data siteId)" != "null" ]]; then
  SITE_ID="$(get_data siteId)"
  wa_ev "site" "reuse test-data siteId" "pass" "Reusing site" "$(jq -nc --arg o "$ORG_ID" --arg s "$SITE_ID" '{organizationId:$o,siteId:$s}')"
else
  SITE_CODE="E${SUFFIX}"
  SITE_CODE="${SITE_CODE:0:12}"
  SITE_BODY="$(jq -nc \
    --arg name "${SITE_NAME_SEED} ${SUFFIX}" \
    --arg code "$SITE_CODE" \
    --arg tz "$SITE_TZ" \
    --arg l1 "E2E ${SUFFIX}" \
    '{name:$name,code:$code,timezone:$tz,address:{line1:$l1}}')"
  path="/v1/admin/organizations/${ORG_ID}/sites"
  code="$(e2e_http_post_json "wa-site-create" "$path" "$SITE_BODY")"
  if ! e2e_http_assert_status "wa-site-create" "201" "$code"; then
    wa_fail "site" "$path" "Create site failed HTTP $code — see rest/wa-site-create.response.json"
  fi
  SITE_ID="$(e2e_jq_resp wa-site-create -r '.id // empty')"
  e2e_set_data siteId "$SITE_ID"
  wa_ev "site" "$path" "pass" "Created site" "$(jq -nc --arg o "$ORG_ID" --arg s "$SITE_ID" '{organizationId:$o,siteId:$s}')"
fi

# --- Machine ---
MACHINE_ID=""
M_SERIAL="E2E-SN-${SUFFIX}"
M_NAME="E2E Machine ${SUFFIX}"
if [[ "${E2E_REUSE_DATA:-false}" == "true" ]] && [[ -n "$(get_data machineId)" ]] && [[ "$(get_data machineId)" != "null" ]]; then
  MACHINE_ID="$(get_data machineId)"
  wa_ev "machine" "reuse test-data machineId" "pass" "Reusing machine" "$(jq -nc --arg m "$MACHINE_ID" '{machineId:$m}')"
else
  M_BODY="$(jq -nc \
    --arg name "$M_NAME" \
    --arg serial "$M_SERIAL" \
    --arg sid "$SITE_ID" \
    --arg model "$MACHINE_MODEL" \
    --arg code "m-${SUFFIX}" \
    '{name:$name,serialNumber:$serial,siteId:$sid,status:"draft",timezone:"UTC",cabinetType:"ambient",model:$model,code:$code}')"
  path="/v1/admin/organizations/${ORG_ID}/machines"
  code="$(e2e_http_post_json "wa-machine-create" "$path" "$M_BODY")"
  if ! e2e_http_assert_status "wa-machine-create" "201" "$code"; then
    wa_fail "machine" "$path" "Create machine failed HTTP $code — see rest/wa-machine-create.response.json (site valid? permissions?)"
  fi
  MACHINE_ID="$(e2e_jq_resp wa-machine-create -r '.id // empty')"
  [[ -n "$MACHINE_ID" ]] || MACHINE_ID="$(e2e_jq_resp wa-machine-create -r '.machineId // empty')"
  e2e_set_data machineId "$MACHINE_ID"
  e2e_set_data machineSerial "$M_SERIAL"
  wa_ev "machine" "$path" "pass" "Created machine" "$(jq -nc --arg m "$MACHINE_ID" '{machineId:$m}')"
fi

# --- Activation code ---
if [[ "${E2E_REUSE_DATA:-false}" == "true" ]] && [[ -n "$(get_data activationCodeId)" ]] && [[ "$(get_data activationCodeId)" != "null" ]]; then
  wa_ev "activation" "reuse" "pass" "Reuse activationCodeId in test-data" "$(jq -nc --arg a "$(get_data activationCodeId)" '{activationCodeId:$a}')"
else
  A_BODY="$(jq -nc \
    --arg mid "$MACHINE_ID" \
    '{machineId:$mid,expiresInMinutes:10080,maxUses:2,notes:"e2e web admin setup"}')"
  path="/v1/admin/organizations/${ORG_ID}/activation-codes"
  code="$(e2e_http_post_json_idem "wa-activation-create" "$path" "$A_BODY" "e2e-act-${SUFFIX}")"
  if ! e2e_http_assert_status "wa-activation-create" "201" "$code"; then
    wa_fail "activation" "$path" "Activation code issue failed HTTP $code — rest/wa-activation-create.response.json"
  fi
  AID="$(e2e_jq_resp wa-activation-create -r '.activationCodeId // empty')"
  ACODE="$(e2e_jq_resp wa-activation-create -r '.activationCode // empty')"
  e2e_set_data activationCodeId "$AID"
  if [[ -n "$ACODE" ]]; then
    save_token activationCodePlain "$ACODE"
  fi
  wa_ev "activation" "$path" "pass" "Issued activation code" "$(jq -nc --arg id "$AID" '{activationCodeId:$id}')"
fi

# --- Category ---
CAT_ID=""
CAT_SLUG="e2e-cat-${SUFFIX}"
if [[ "${E2E_REUSE_DATA:-false}" == "true" ]] && [[ -n "$(get_data categoryId)" ]] && [[ "$(get_data categoryId)" != "null" ]]; then
  CAT_ID="$(get_data categoryId)"
  wa_ev "category" "reuse" "pass" "Reusing categoryId" "$(jq -nc --arg c "$CAT_ID" '{categoryId:$c}')"
else
  C_BODY="$(jq -nc --arg name "${ORG_NAME} Cat ${SUFFIX}" --arg slug "$CAT_SLUG" '{name:$name,slug:$slug,parentId:null,active:true}')"
  path="/v1/admin/categories?${Q_ORG}"
  code="$(e2e_http_post_json_idem "wa-category-create" "$path" "$C_BODY" "e2e-cat-${SUFFIX}")"
  if ! e2e_http_assert_status "wa-category-create" "200" "$code"; then
    wa_fail "category" "$path" "Create category failed HTTP $code"
  fi
  CAT_ID="$(e2e_jq_resp wa-category-create -r '.id // empty')"
  e2e_set_data categoryId "$CAT_ID"
  wa_ev "category" "$path" "pass" "Created category" "$(jq -nc --arg c "$CAT_ID" '{categoryId:$c}')"
fi

# --- Brand (optional) ---
BRAND_ID=""
if [[ "${E2E_WEB_ADMIN_SKIP_BRAND:-}" == "1" ]]; then
  wa_ev "brand" "—" "skip" "E2E_WEB_ADMIN_SKIP_BRAND=1" "{}"
else
  BR_SLUG="e2e-brand-${SUFFIX}"
  B_BODY="$(jq -nc --arg name "E2E Brand ${SUFFIX}" --arg slug "$BR_SLUG" '{name:$name,slug:$slug,active:true}')"
  path="/v1/admin/brands?${Q_ORG}"
  code="$(e2e_http_post_json_idem "wa-brand-create" "$path" "$B_BODY" "e2e-brand-${SUFFIX}")"
  if e2e_http_assert_status "wa-brand-create" "200" "$code"; then
    BRAND_ID="$(e2e_jq_resp wa-brand-create -r '.id // empty')"
    e2e_set_data brandId "$BRAND_ID"
    wa_ev "brand" "$path" "pass" "Created brand" "$(jq -nc --arg b "$BRAND_ID" '{brandId:$b}')"
  else
    wa_ev "brand" "$path" "skip" "Brand create not available or conflict (HTTP $code)" "{}"
  fi
fi

# --- Tag (optional) ---
TAG_ID=""
if [[ "${E2E_WEB_ADMIN_SKIP_TAG:-}" == "1" ]]; then
  wa_ev "tag" "—" "skip" "E2E_WEB_ADMIN_SKIP_TAG=1" "{}"
else
  T_SLUG="e2e-tag-${SUFFIX}"
  T_BODY="$(jq -nc --arg name "E2E Tag ${SUFFIX}" --arg slug "$T_SLUG" '{name:$name,slug:$slug,active:true}')"
  path="/v1/admin/tags?${Q_ORG}"
  code="$(e2e_http_post_json_idem "wa-tag-create" "$path" "$T_BODY" "e2e-tag-${SUFFIX}")"
  if e2e_http_assert_status "wa-tag-create" "200" "$code"; then
    TAG_ID="$(e2e_jq_resp wa-tag-create -r '.id // empty')"
    e2e_set_data tagId "$TAG_ID"
    wa_ev "tag" "$path" "pass" "Created tag" "$(jq -nc --arg t "$TAG_ID" '{tagId:$t}')"
  else
    wa_ev "tag" "$path" "skip" "Tag create skipped (HTTP $code)" "{}"
  fi
fi

# --- Product (required idempotency per OpenAPI) ---
PROD_SKU="E2E-SKU-${SUFFIX}"
PRODUCT_ID=""
if [[ "${E2E_REUSE_DATA:-false}" == "true" ]] && [[ -n "$(get_data productId)" ]] && [[ "$(get_data productId)" != "null" ]]; then
  PRODUCT_ID="$(get_data productId)"
  wa_ev "product" "reuse" "pass" "Reusing productId" "$(jq -nc --arg p "$PRODUCT_ID" '{productId:$p}')"
else
  P_BODY="$(jq -nc \
    --arg name "E2E Product ${SUFFIX}" \
    --arg sku "$PROD_SKU" \
    --arg cid "$CAT_ID" \
    '{name:$name,sku:$sku,description:"E2E catalog seed",active:true,ageRestricted:false,allergenCodes:[],categoryId:$cid}')"
  if [[ -n "$BRAND_ID" ]]; then
    P_BODY="$(echo "$P_BODY" | jq --arg bid "$BRAND_ID" '. + {brandId:$bid}')"
  fi
  path="/v1/admin/products?${Q_ORG}"
  code="$(e2e_http_post_json_idem "wa-product-create" "$path" "$P_BODY" "e2e-prod-${SUFFIX}")"
  if ! e2e_http_assert_status "wa-product-create" "200" "$code"; then
    wa_fail "product" "$path" "Create product failed HTTP $code — check Idempotency-Key / SKU / category"
  fi
  PRODUCT_ID="$(e2e_jq_resp wa-product-create -r '.id // empty')"
  e2e_set_data productId "$PRODUCT_ID"
  e2e_set_data productSku "$PROD_SKU"
  wa_ev "product" "$path" "pass" "Created product" "$(jq -nc --arg p "$PRODUCT_ID" '{productId:$p}')"
fi

# --- Planogram template + operator session + draft/publish + stock (best-effort) ---
plc="$(e2e_http_get "wa-planogram-list" "/v1/admin/planograms?${Q_ORG}&limit=20")"
if [[ "$plc" != "200" ]]; then
  wa_ev "planogram" "GET /v1/admin/planograms" "skip" "List planograms HTTP $plc (forbidden / wrong org?)" "{}"
  log_warn "Planogram list not available (HTTP ${plc}); skipping machine planogram + inventory."
  log_flow_design_issue "P2" "$FLOW_ID" "01_web_admin_setup.sh" "planogram-list" "REST" "GET /v1/admin/planograms" "Cannot list planograms during setup — slot/publish pipeline blocked" "Incomplete machine readiness for vending automation" "Document org permissions; provide bootstrap planogram fixture" "${E2E_RUN_DIR}/rest/wa-planogram-list.meta.json"
  log_data_setup_issue "P2" "$FLOW_ID" "01_web_admin_setup.sh" "planogram-list" "data" "planogram prerequisite" "Setup stops early; reuse-data may hide missing org fixtures" "Flaky Phase 4/8" "Seed minimum org planogram or add diagnostics endpoint" "${E2E_RUN_DIR}/rest/wa-planogram-list.meta.json"
  wa_ev "inventory" "—" "skip" "No planogram list" "{}"
  e2e_flow_review_scenario_complete "$FLOW_ID" "01_web_admin_setup.sh" "flow-review-early" "stopped_after_planogram_list_failure"
  exit 0
fi
PG_ID="$(jq -r '(.items // [])[] | select(.status=="published") | .id' "${E2E_RUN_DIR}/rest/wa-planogram-list.response.json" 2>/dev/null | head -n1)"
PG_REV="$(jq -r --arg id "$PG_ID" '(.items // [])[] | select(.id==$id) | .revision' "${E2E_RUN_DIR}/rest/wa-planogram-list.response.json" 2>/dev/null | head -n1)"
if [[ -z "$PG_ID" ]]; then
  PG_ID="$(jq -r '(.items // [])[0].id // empty' "${E2E_RUN_DIR}/rest/wa-planogram-list.response.json" 2>/dev/null)"
  PG_REV="$(jq -r '(.items // [])[0].revision // 0' "${E2E_RUN_DIR}/rest/wa-planogram-list.response.json" 2>/dev/null)"
fi
PG_REV="${PG_REV:-0}"
case "$PG_REV" in
  '' | *[!0-9]*) PG_REV=0 ;;
esac
if [[ -z "$PG_ID" ]]; then
  wa_ev "planogram" "GET /v1/admin/planograms" "skip" "No planogram in org — publish/slot/inventory steps skipped. Seed a planogram in admin UI or DB." "{}"
  log_warn "No planogram found; skipping operator session, draft, publish, and stock adjustment."
  log_flow_design_issue "P2" "$FLOW_ID" "01_web_admin_setup.sh" "planogram-empty" "REST" "/v1/admin/planograms" "Org has no planogram row — cannot assign product to machine slot deterministically via API in this run" "Vending sale-catalog may be empty or inconsistent" "API or admin UX to clone default planogram; document minimum seed" "${E2E_RUN_DIR}/rest/wa-planogram-list.response.json"
  wa_ev "inventory" "—" "skip" "No planogram / slot pipeline" "{}"
  e2e_flow_review_scenario_complete "$FLOW_ID" "01_web_admin_setup.sh" "flow-review-early" "stopped_no_planogram_in_org"
  exit 0
fi
e2e_set_data planogramId "$PG_ID"
e2e_set_data planogramRevision "$PG_REV"
wa_ev "planogram-list" "GET /v1/admin/planograms?${Q_ORG}" "pass" "Selected planogram" "$(jq -nc --arg p "$PG_ID" --argjson r "$PG_REV" '{planogramId:$p,planogramRevision:$r}')"

OP_BODY="$(jq -nc '{force_admin_takeover:true, auth_method:"oidc"}')"
code="$(e2e_http_post_json "wa-operator-login" "/v1/machines/${MACHINE_ID}/operator-sessions/login" "$OP_BODY")"
if ! e2e_http_assert_status "wa-operator-login" "200" "$code"; then
  wa_ev "operator-login" "POST /v1/.../operator-sessions/login" "skip" "HTTP $code — cannot run planogram draft without ACTIVE operator session (assignment/admin?)" "{}"
  wa_ev "planogram-draft" "—" "skip" "No operator session" "{}"
  wa_ev "planogram-publish" "—" "skip" "No operator session" "{}"
  wa_ev "inventory" "—" "skip" "No operator session" "{}"
  log_flow_design_issue "P2" "$FLOW_ID" "01_web_admin_setup.sh" "operator-session" "REST" "POST /v1/machines/{id}/operator-sessions/login" "Operator session required for planogram draft but automation cannot obtain it" "Blocks deterministic slot/planogram assignment" "Support machine-admin or service credentials for CI; document session preconditions" "${E2E_RUN_DIR}/rest/wa-operator-login.meta.json"
  e2e_flow_review_scenario_complete "$FLOW_ID" "01_web_admin_setup.sh" "flow-review-early" "stopped_no_operator_session"
  exit 0
fi
OP_SID="$(e2e_jq_resp wa-operator-login -r '.session.id // empty')"
e2e_set_data operatorSessionId "$OP_SID"
wa_ev "operator-login" "POST /v1/machines/${MACHINE_ID}/operator-sessions/login" "pass" "Operator session" "$(jq -nc --arg s "$OP_SID" '{operatorSessionId:$s}')"

DRAFT_JSON="$(jq -nc \
  --arg sid "$OP_SID" \
  --arg pid "$PG_ID" \
  --argjson prev "$PG_REV" \
  --arg cab "$CABINET" \
  --arg sc "$SLOT" \
  --arg prid "$PRODUCT_ID" \
  --argjson price "$PRICE_MINOR" \
  --argjson qty "$INV_QTY" \
  '{
    operator_session_id:$sid,
    planogramId:$pid,
    planogramRevision:$prev,
    syncLegacyReadModel:true,
    items:[{
      cabinetCode:$cab,
      slotCode:$sc,
      slotIndex:1,
      productId:$prid,
      maxQuantity:$qty,
      priceMinor:($price | tonumber),
      layoutKey:"grid-4x6",
      layoutRevision:1,
      legacySlotIndex:1,
      metadata:{}
    }]
  }')"
path="/v1/admin/machines/${MACHINE_ID}/planograms/draft?${Q_ORG}"
code="$(e2e_http_put_json "wa-planogram-draft" "$path" "$DRAFT_JSON")"
if ! e2e_http_assert_status "wa-planogram-draft" "204" "$code"; then
  wa_ev "planogram-draft" "$path" "skip" "Draft save HTTP $code — inspect rest/wa-planogram-draft.response.json" "{}"
  wa_ev "planogram-publish" "—" "skip" "Draft failed" "{}"
  wa_ev "inventory" "—" "skip" "Draft failed" "{}"
  log_api_contract_issue "P2" "$FLOW_ID" "01_web_admin_setup.sh" "planogram-draft" "REST" "$path" "Planogram draft rejected — product-to-slot assignment not persisted" "Automated catalog incomplete" "Align OpenAPI with required fields; return structured validation errors" "${E2E_RUN_DIR}/rest/wa-planogram-draft.response.json"
  e2e_flow_review_scenario_complete "$FLOW_ID" "01_web_admin_setup.sh" "flow-review-early" "stopped_planogram_draft_failed"
  exit 0
fi
wa_ev "planogram-draft" "$path" "pass" "Saved draft slots" "$(jq -nc --arg p "$PG_ID" '{planogramId:$p}')"

PUB_JSON="$DRAFT_JSON"
path="/v1/admin/machines/${MACHINE_ID}/planograms/publish?${Q_ORG}"
code="$(e2e_http_post_json_idem "wa-planogram-publish" "$path" "$PUB_JSON" "e2e-pub-${SUFFIX}")"
if ! e2e_http_assert_status "wa-planogram-publish" "200" "$code"; then
  wa_ev "planogram-publish" "$path" "skip" "Publish HTTP $code" "{}"
  wa_ev "inventory" "—" "skip" "Publish failed" "{}"
  log_api_contract_issue "P2" "$FLOW_ID" "01_web_admin_setup.sh" "planogram-publish" "REST" "$path" "Planogram publish failed — version/ack of catalog on machine unclear" "Devices may not see product assignment" "Expose catalog_version and publish error details; document rollback" "${E2E_RUN_DIR}/rest/wa-planogram-publish.response.json"
  e2e_flow_review_scenario_complete "$FLOW_ID" "01_web_admin_setup.sh" "flow-review-early" "stopped_planogram_publish_failed"
  exit 0
fi
wa_ev "planogram-publish" "$path" "pass" "Published planogram" "{}"

e2e_http_get "wa-slots-after" "/v1/admin/machines/${MACHINE_ID}/slots?${Q_ORG}"
Q_BEFORE="$(jq -r --arg sc "$SLOT" '(.slots // [])[] | select(.slotCode==$sc) | .currentQuantity' "${E2E_RUN_DIR}/rest/wa-slots-after.response.json" 2>/dev/null | head -n1)"
[[ -n "$Q_BEFORE" ]] || Q_BEFORE="0"
Q_AFTER="$INV_QTY"
STOCK_JSON="$(jq -nc \
  --arg sid "$OP_SID" \
  --arg pid "$PG_ID" \
  --arg prid "$PRODUCT_ID" \
  --arg cab "$CABINET" \
  --arg sc "$SLOT" \
  --argjson qb "$Q_BEFORE" \
  --argjson qa "$Q_AFTER" \
  '{
    operator_session_id:$sid,
    reason:"restock",
    items:[{
      cabinetCode:$cab,
      slotCode:$sc,
      slotIndex:1,
      productId:$prid,
      planogramId:$pid,
      quantityBefore:$qb,
      quantityAfter:$qa
    }]
  }')"
path="/v1/admin/machines/${MACHINE_ID}/stock-adjustments?${Q_ORG}"
code="$(e2e_http_post_json_idem "wa-stock" "$path" "$STOCK_JSON" "e2e-stock-${SUFFIX}")"
if e2e_http_assert_status "wa-stock" "200" "$code"; then
  e2e_set_data inventoryQuantityAfter "$Q_AFTER"
  wa_ev "inventory" "$path" "pass" "Stock adjustment" "$(jq -nc --arg sc "$SLOT" --argjson q "$Q_AFTER" '{slotCode:$sc,inventoryQuantity:$q}')"
else
  wa_ev "inventory" "$path" "skip" "Stock adjustment HTTP $code (quantity_before mismatch or slot not ready)" "{}"
  log_api_contract_issue "P2" "$FLOW_ID" "01_web_admin_setup.sh" "stock-adjustment" "REST" "$path" "Stock adjustment did not apply — inventory quantity may not match expected E2E state" "Sale/inventory tests may flake" "Deterministic quantity_before rules; clearer conflict responses" "${E2E_RUN_DIR}/rest/wa-stock.response.json"
fi

wa_ev "done" "—" "pass" "Web admin setup completed" "$(jq -nc --arg o "$ORG_ID" --arg s "$SITE_ID" --arg m "$MACHINE_ID" --arg p "$PRODUCT_ID" --arg sl "$SLOT" '{organizationId:$o,siteId:$s,machineId:$m,productId:$p,slotCode:$sl}')"

log_unnecessary_complexity_issue "P3" "$FLOW_ID" "01_web_admin_setup.sh" "setup-call-depth" "mixed" "WA-SETUP-01" "Scratch setup requires many sequential admin REST calls before machine is vending-ready" "Slower CI; fragile ordering" "Provide bundled onboarding or fixture APIs; document minimal call graph" "${E2E_RUN_DIR}/test-data.json"
log_response_shape_issue "P3" "$FLOW_ID" "01_web_admin_setup.sh" "machine-create-shape" "REST" "POST /v1/admin/organizations/{org}/machines" "Harness tolerates both .id and .machineId in create-machine response" "Clients may parse inconsistently" "Single canonical resource id field in OpenAPI" "${E2E_RUN_DIR}/rest/wa-machine-create.response.json"
log_data_setup_issue "P3" "$FLOW_ID" "01_web_admin_setup.sh" "reuse-safety" "docs" "reuse vs --fresh-data" "Duplicate handling depends on idempotency keys and manual cleanup; no unified archive strategy asserted" "Shared-org QA collisions" "Document cleanup playbook; add delete/retire endpoints for E2E resources" "${E2E_RUN_DIR}/test-data.json"
e2e_flow_review_scenario_complete "$FLOW_ID" "01_web_admin_setup.sh" "flow-review-complete" "wa_setup_completed_with_documented_debt"

exit 0
