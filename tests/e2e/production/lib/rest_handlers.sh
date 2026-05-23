#!/usr/bin/env bash
# shellcheck shell=bash
# REST special handlers (media presigned upload, planogram prerequisites).

prod_e2e_rest_dispatch() {
  local flow_json="$1"
  local handler
  handler="$(echo "$flow_json" | jq -r '.handler // "standard"')"
  case "$handler" in
    standard) prod_e2e_rest_execute_flow "$flow_json" ;;
    media_presigned_upload) prod_e2e_rest_media_presigned_upload "$flow_json" ;;
    media_cloudinary_upload) prod_e2e_rest_media_cloudinary_upload "$flow_json" ;;
    planogram_list) prod_e2e_rest_planogram_list "$flow_json" ;;
    slots_verify) prod_e2e_rest_slots_verify "$flow_json" ;;
    *) echo "unknown REST handler: ${handler} for $(echo "$flow_json" | jq -r .id)" >&2; return 1 ;;
  esac
}

prod_e2e_rest_planogram_list() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  prod_e2e_rest_execute_flow "$flow_json" || return 1
  local resp="${PROD_E2E_RAW_DIR}/rest-planogram-list.response.json"
  [[ -f "$resp" ]] || return 1
  local pg_id pg_rev
  pg_id="$(jq -r '(.items // [])[] | select(.status=="published") | .id' "$resp" 2>/dev/null | head -n1)"
  pg_rev="$(jq -r --arg id "$pg_id" '(.items // [])[] | select(.id==$id) | .revision' "$resp" 2>/dev/null | head -n1)"
  if [[ -z "$pg_id" ]]; then
    pg_id="$(jq -r '(.items // [])[0].id // empty' "$resp" 2>/dev/null)"
    pg_rev="$(jq -r '(.items // [])[0].revision // 0' "$resp" 2>/dev/null)"
  fi
  [[ -n "$pg_id" ]] || {
    prod_e2e_fail_classify "e" "REST-PLANO-000" "no planogram in org — seed planogram or fix data setup"
    return 1
  }
  pg_rev="${pg_rev:-0}"
  prod_e2e_state_set planogramId "$pg_id"
  prod_e2e_state_set planogramRevision "$pg_rev"
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "rest" "pass" "$evidence_label"
  return 0
}

prod_e2e_rest_media_presigned_upload() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"

  local init_flow
  init_flow="$(echo "$flow_json" | jq -c '.init_flow')"
  prod_e2e_rest_execute_flow "$init_flow" || return 1

  local init_resp="${PROD_E2E_RAW_DIR}/rest-media-init.response.json"
  local media_id upload_url
  media_id="$(jq -r '.mediaId // empty' "$init_resp")"
  upload_url="$(jq -r '.uploadUrl // .upload_url // empty' "$init_resp")"
  [[ -n "$media_id" && -n "$upload_url" ]] || {
    prod_e2e_fail_classify "c" "$id" "media init response missing mediaId/uploadUrl — API contract drift"
    return 1
  }
  prod_e2e_state_set mediaId "$media_id"

  local png="${PROD_E2E_PRODUCTION_DIR}/fixtures/test-product.png"
  local put_meta="${PROD_E2E_RAW_DIR}/${evidence_label}.put.meta.json"
  local put_code
  put_code="$(curl -sS -o "${PROD_E2E_RAW_DIR}/${evidence_label}.put.body" -w '%{http_code}' -X PUT \
    -H "Content-Type: image/png" --data-binary "@${png}" "$upload_url" || true)"
  jq -nc --arg url "$upload_url" --argjson code "${put_code:-0}" '{method:"PUT",upload_url:$url,http_code:$code}' >"$put_meta"

  if [[ "$put_code" != "200" && "$put_code" != "204" ]]; then
    prod_e2e_fail_classify "d" "$id" "presigned PUT failed HTTP ${put_code} — production object storage config"
    return 1
  fi

  local complete_flow
  complete_flow="$(echo "$flow_json" | jq -c '.complete_flow')"
  prod_e2e_rest_execute_flow "$complete_flow" || return 1
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "rest" "pass" "$evidence_label"
  return 0
}

prod_e2e_rest_media_cloudinary_upload() {
  local flow_json="$1"
  local id evidence_label upload_flow
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  upload_flow="$(echo "$flow_json" | jq -c '.upload_flow // .init_flow')"

  local path url method flow_id evidence_sub
  path="$(echo "$upload_flow" | jq -r '.path')"
  method="$(echo "$upload_flow" | jq -r '.method // "POST"')"
  flow_id="$(echo "$upload_flow" | jq -r '.id // "REST-MEDIA-UPLOAD"')"
  evidence_sub="$(echo "$upload_flow" | jq -r '.evidence_label // "rest-media-cloudinary"')"
  path="$(prod_e2e_render_template_string "$path")"
  url="${BASE_URL%/}${path}"

  if declare -F prod_e2e_state_reload_key >/dev/null 2>&1; then
    prod_e2e_state_reload_key accessToken || true
    prod_e2e_state_reload_key ADMIN_TOKEN || true
  fi
  local tok="${ADMIN_TOKEN:-${accessToken:-}}"
  [[ -n "$tok" ]] || {
    prod_e2e_fail_classify "d" "$id" "admin token missing for Cloudinary media upload"
    return 1
  }

  local png="${PROD_E2E_PRODUCTION_DIR}/fixtures/test-product.png"
  if [[ ! -f "$png" ]]; then
    png="${PROD_E2E_REPO_ROOT}/tests/e2e/production/fixtures/test-product.png"
  fi
  if command -v cygpath >/dev/null 2>&1; then
    png="$(cygpath -u "$png" 2>/dev/null || echo "$png")"
  fi
  [[ -f "$png" ]] || {
    prod_e2e_fail_classify "a" "$id" "missing fixture test-product.png"
    return 1
  }

  local resp_file="${PROD_E2E_RAW_DIR}/${evidence_sub}.response.json"
  local hdr_file="${PROD_E2E_RAW_DIR}/${evidence_sub}.response.headers.txt"
  local req_file="${PROD_E2E_RAW_DIR}/${evidence_sub}.request.json"
  local meta_file="${PROD_E2E_RAW_DIR}/${evidence_sub}.meta.json"
  jq -nc --arg method "$method" --arg path "$path" --arg url "$url" \
    '{method:$method,path:$path,url:$url,auth:"bearer_admin",body:"multipart file=product_image"}' >"$req_file"
  prod_e2e_redact_file "$req_file" "${req_file}.redacted"
  mv "${req_file}.redacted" "$req_file"

  local code idem_key
  idem_key="${PROD_E2E_PREFIX}-${flow_id}"
  code="$(prod_e2e_py "${PROD_E2E_PRODUCTION_DIR}/scripts/upload_multipart.py" \
    "$url" "$tok" "$idem_key" "$png" "$hdr_file" "$resp_file" 2>/dev/null || echo 0)"
  code="${code//$'\r'/}"
  [[ "$code" =~ ^[0-9]+$ ]] || code=0

  prod_e2e_redact_file "$resp_file" "${resp_file}.redacted" 2>/dev/null || true
  [[ -f "${resp_file}.redacted" ]] && mv "${resp_file}.redacted" "$resp_file"

  local expected
  expected="$(echo "$upload_flow" | jq -r '.expected_status // 200')"
  jq -nc --argjson code "${code:-0}" --arg expected "$expected" \
    '{http_code:$code,expected_status:$expected}' >"$meta_file"

  if [[ "$code" != "$expected" ]]; then
    prod_e2e_fail_classify "c" "$id" "Cloudinary media upload expected HTTP ${expected} got ${code}"
    prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "rest" "fail" "$evidence_label"
    return 1
  fi

  local media_id sha
  media_id="$(jq -r '.mediaId // empty' "$resp_file" 2>/dev/null | tr -d '\r')"
  sha="$(jq -r '.checksum // .sha256 // empty' "$resp_file" 2>/dev/null | tr -d '\r')"
  [[ -n "$media_id" ]] || {
    prod_e2e_fail_classify "c" "$id" "Cloudinary upload response missing mediaId"
    prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "rest" "fail" "$evidence_label"
    return 1
  }
  prod_e2e_state_set mediaId "$media_id"
  [[ -n "$sha" ]] && prod_e2e_state_set mediaSha256 "$sha"

  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "rest" "pass" "$evidence_label"
  return 0
}

prod_e2e_rest_slots_verify() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  prod_e2e_rest_execute_flow "$flow_json" || return 1
  local resp="${PROD_E2E_RAW_DIR}/${evidence_label}.response.json"
  [[ -f "$resp" ]] || return 1
  if ! jq -e --arg sc "A1" --arg pid "${productId:-}" '
    (.slots // [])[] | select(.slotCode == $sc) |
    (.productId == $pid) and (.priceMinor != null) and
    ((.maxQuantity // .capacity) != null) and ((.currentQuantity // .quantity) != null)
  ' "$resp" >/dev/null 2>&1; then
    prod_e2e_fail_classify "c" "$id" "slot A1 missing productId, priceMinor, capacity, or quantity — API contract drift"
    prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "rest" "fail" "$evidence_label"
    return 1
  fi
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "rest" "pass" "$evidence_label"
  return 0
}
