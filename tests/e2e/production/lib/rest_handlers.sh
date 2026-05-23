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
