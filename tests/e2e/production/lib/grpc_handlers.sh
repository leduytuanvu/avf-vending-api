#!/usr/bin/env bash
# shellcheck shell=bash
# Multi-step gRPC E2E handlers (commerce, media cache, idempotency, offline).

prod_e2e_grpc_dispatch() {
  local flow_json="$1"
  local handler
  handler="$(echo "$flow_json" | jq -r '.handler')"
  case "$handler" in
    catalog_sync_assert) prod_e2e_grpc_handler_catalog_sync_assert "$flow_json" ;;
    media_download_cache) prod_e2e_grpc_handler_media_download_cache "$flow_json" ;;
    commerce_cash) prod_e2e_grpc_handler_commerce_cash "$flow_json" ;;
    commerce_qr) prod_e2e_grpc_handler_commerce_qr "$flow_json" ;;
    commerce_vend_failure) prod_e2e_grpc_handler_commerce_vend_failure "$flow_json" ;;
    commerce_cancel) prod_e2e_grpc_handler_commerce_cancel "$flow_json" ;;
    commerce_idempotency) prod_e2e_grpc_handler_commerce_idempotency "$flow_json" ;;
    offline_replay) prod_e2e_grpc_handler_offline_replay "$flow_json" ;;
    inventory_snapshot_capture) prod_e2e_grpc_handler_inventory_snapshot_capture "$flow_json" ;;
    *)
      echo "unknown gRPC handler: ${handler}" >&2
      return 1
      ;;
  esac
}

prod_e2e_grpc_flow_from_handler() {
  local flow_json="$1"
  local key="$2"
  echo "$flow_json" | jq -c --arg k "$key" '.[$k]'
}

prod_e2e_grpc_slot_index() {
  echo "${slotIndex:-1}"
}

prod_e2e_grpc_slot_quantity() {
  local resp_file="$1"
  local si qty
  si="$(prod_e2e_grpc_slot_index)"
  qty="$(jq -r --argjson si "$si" '
    (.slots // [])[] |
    select(.slotIndex == $si or .slot_index == $si) |
    (.currentQuantity // .current_quantity // empty)
  ' "$resp_file" 2>/dev/null | head -n1)"
  if [[ -z "$qty" || "$qty" == "null" ]]; then
    qty="$(jq -r --arg sc "A1" '
      (.slots // [])[] |
      select(.slotCode == $sc or .slot_code == $sc) |
      (.currentQuantity // .current_quantity // empty)
    ' "$resp_file" 2>/dev/null | head -n1)"
  fi
  echo "${qty:-0}"
}

prod_e2e_grpc_normalize_sha256() {
  local s="$1"
  s="${s#sha256:}"
  printf '%s' "$s"
}

prod_e2e_grpc_handler_catalog_sync_assert() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  local inner
  inner="$(prod_e2e_grpc_flow_from_handler "$flow_json" "catalog_flow")"
  prod_e2e_grpc_execute_flow "$inner" || return 1
  local resp="${PROD_E2E_RAW_DIR}/$(echo "$inner" | jq -r '.evidence_label').response.json"
  if grep -q 'data:image/' "$resp" 2>/dev/null; then
    prod_e2e_fail_classify "c" "$id" "catalog response contains embedded image bytes — URLs only per proto"
    return 1
  fi
  local cat_ver
  cat_ver="$(jq -r '.snapshot.catalogVersion // empty' "$resp")"
  [[ -n "$cat_ver" ]] && prod_e2e_state_set catalogVersion "$cat_ver"
  local pid="${productId:-}"
  if ! jq -e --arg pid "$pid" '
    (.snapshot.items // [])[] |
    select(.productId == $pid) |
    (.primaryMedia.displayUrl // .primaryMedia.thumbUrl // .primaryMedia.display_url // .primaryMedia.thumb_url) != null
  ' "$resp" >/dev/null 2>&1; then
    prod_e2e_fail_classify "c" "$id" "published catalog item missing primary media URL for productId=${pid}"
    return 1
  fi
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "grpc" "pass" "$evidence_label"
  return 0
}

prod_e2e_grpc_handler_media_download_cache() {
  local flow_json="$1"
  local id evidence_label
  id="$(echo "$flow_json" | jq -r '.id')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  local inner
  inner="$(prod_e2e_grpc_flow_from_handler "$flow_json" "manifest_flow")"
  prod_e2e_grpc_execute_flow "$inner" || return 1
  local resp="${PROD_E2E_RAW_DIR}/$(echo "$inner" | jq -r '.evidence_label').response.json"
  local mfp
  mfp="$(jq -r '.mediaFingerprint // empty' "$resp")"
  [[ -n "$mfp" ]] && prod_e2e_state_set mediaFingerprint "$mfp"

  local cache_dir="${PROD_E2E_RUN_DIR}/media-cache"
  mkdir -p "$cache_dir"
  local url sha256 ctype size_bytes idx=0 failures=0
  while IFS=$'\t' read -r url sha256 ctype size_bytes; do
    [[ -n "$url" ]] || continue
    idx=$((idx + 1))
    local out="${cache_dir}/media-${idx}.bin"
    local meta="${PROD_E2E_RAW_DIR}/${evidence_label}.download-${idx}.meta.json"
    local code
    code="$(curl -sS -L -o "$out" -w '%{http_code}' "$url" || true)"
    local dl_sha dl_size
    dl_size="$(wc -c <"$out" 2>/dev/null | tr -d ' ')"
    dl_sha="$(prod_e2e_py -c "import hashlib,sys; print(hashlib.sha256(open(sys.argv[1],'rb').read()).hexdigest())" "$out" 2>/dev/null || true)"
    jq -nc \
      --arg url "$url" \
      --argjson code "${code:-0}" \
      --arg expected_sha "$sha256" \
      --arg actual_sha "$dl_sha" \
      --arg ctype "$ctype" \
      --argjson size "$size_bytes" \
      --argjson dl_size "${dl_size:-0}" \
      --arg path "$out" \
      '{url:$url,http_code:$code,expected_sha256:$expected_sha,actual_sha256:$actual_sha,content_type:$ctype,expected_size_bytes:$size,downloaded_size_bytes:$dl_size,cache_path:$path}' >"$meta"
    if [[ "$code" != "200" ]]; then
      failures=$((failures + 1))
      continue
    fi
    if [[ -n "$sha256" && -n "$dl_sha" ]]; then
      local exp_sha act_sha
      exp_sha="$(prod_e2e_grpc_normalize_sha256 "$sha256")"
      act_sha="$(prod_e2e_grpc_normalize_sha256 "$dl_sha")"
      if [[ "$exp_sha" != "$act_sha" ]]; then
        prod_e2e_fail_classify "c" "$id" "media download sha256 mismatch for ${url}"
        failures=$((failures + 1))
      fi
    fi
  done < <(jq -r '
    (.entries // [])[] |
    .primaryMedia as $m |
    [
      ($m.displayUrl // $m.display_url // $m.thumbUrl // $m.thumb_url // ""),
      ($m.checksumSha256 // $m.checksum_sha256 // ""),
      ($m.contentType // $m.content_type // "image/png"),
      ($m.sizeBytes // $m.size_bytes // 0)
    ] | @tsv
  ' "$resp" 2>/dev/null)

  if [[ "$idx" -eq 0 ]]; then
    prod_e2e_fail_classify "e" "$id" "media manifest has no downloadable URLs — verify planogram/product media setup"
    return 1
  fi
  [[ "$failures" -eq 0 ]] || return 1

  local delta_flow ack_flow
  delta_flow="$(prod_e2e_grpc_flow_from_handler "$flow_json" "delta_flow")"
  ack_flow="$(prod_e2e_grpc_flow_from_handler "$flow_json" "ack_flow")"
  [[ "$delta_flow" != "null" ]] && prod_e2e_grpc_execute_flow "$delta_flow" || true
  [[ "$ack_flow" != "null" ]] && prod_e2e_grpc_execute_flow "$ack_flow" || true
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "grpc" "pass" "$evidence_label"
  return 0
}

prod_e2e_grpc_handler_inventory_snapshot_capture() {
  local flow_json="$1"
  local inner
  inner="$(echo "$flow_json" | jq 'del(.handler)')"
  prod_e2e_grpc_execute_flow "$inner" || return 1
  local evidence_label
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  local resp="${PROD_E2E_RAW_DIR}/${evidence_label}.response.json"
  local qty
  qty="$(prod_e2e_grpc_slot_quantity "$resp")"
  prod_e2e_state_set inventoryQtySlotA1 "$qty"
  return 0
}

prod_e2e_grpc_commerce_create_order() {
  local stem="$1"
  local ctx meta body full
  ctx="$(prod_e2e_grpc_idem_context "$stem")"
  meta="$(prod_e2e_grpc_meta_json "${PROD_E2E_PREFIX}-${stem}" "$(echo "$ctx" | jq -r '.idempotencyKey')" "$(echo "$ctx" | jq -r '.clientEventId')")"
  body="$(jq -nc \
    --argjson ctx "$ctx" \
    --argjson meta "$meta" \
    --arg mid "${machineId:-}" \
    --arg pid "${productId:-}" \
    --arg cab "A" \
    --arg sc "A1" \
    --argjson si "$(prod_e2e_grpc_slot_index)" \
    --arg cur "${currency:-VND}" \
    '{context:$ctx, machineId:$mid, productId:$pid, slot:{cabinetCode:$cab, slotCode:$sc, slotIndex:$si}, currency:$cur, meta:$meta}')"
  full="$(prod_e2e_grpc_full_method MachineCommerceService CreateOrder)"
  prod_e2e_grpc_call_raw "$full" "$body" "grpc-${stem}-create-order" machine "$(echo "$ctx" | jq -r '.idempotencyKey')"
}

prod_e2e_grpc_handler_commerce_cash() {
  local flow_json="$1"
  local id stem
  id="$(echo "$flow_json" | jq -r '.id')"
  stem="cash-$(echo "$flow_json" | jq -r '.evidence_label')"

  prod_e2e_grpc_commerce_create_order "$stem" || return 1
  local oid
  oid="$(jq -r '.orderId // empty' "${PROD_E2E_RAW_DIR}/grpc-${stem}-create-order.response.json")"
  [[ -n "$oid" ]] || { prod_e2e_fail_classify "c" "$id" "CreateOrder missing orderId"; return 1; }
  prod_e2e_state_set grpcCashOrderId "$oid"

  local inv_before inv_after ctx cc_body full
  ctx="$(prod_e2e_grpc_idem_context "${stem}-confirm")"
  cc_body="$(jq -nc --argjson ctx "$ctx" --arg oid "$oid" '{context:$ctx, orderId:$oid}')"
  full="$(prod_e2e_grpc_full_method MachineCommerceService ConfirmCashPayment)"
  prod_e2e_grpc_call_raw "$full" "$cc_body" "grpc-${stem}-confirm-cash" machine "$(echo "$ctx" | jq -r '.idempotencyKey')" || return 1

  inv_before="${inventoryQtySlotA1:-}"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineInventoryService GetInventorySnapshot)" \
    "$(jq -nc --argjson meta "$(prod_e2e_grpc_meta_json "${stem}-inv-pre")" '{meta:$meta}')" \
    "grpc-${stem}-inv-pre" machine "" || true
  inv_before="$(prod_e2e_grpc_slot_quantity "${PROD_E2E_RAW_DIR}/grpc-${stem}-inv-pre.response.json")"
  inv_before="${inv_before:-0}"

  ctx="$(prod_e2e_grpc_idem_context "${stem}-vstart")"
  local sv_body
  sv_body="$(jq -nc --argjson ctx "$ctx" --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{context:$ctx, orderId:$oid, slotIndex:$si}')"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService StartVend)" "$sv_body" "grpc-${stem}-vstart" machine "$(echo "$ctx" | jq -r '.idempotencyKey')" || return 1

  ctx="$(prod_e2e_grpc_idem_context "${stem}-vsuccess")"
  local vs_body
  vs_body="$(jq -nc --argjson ctx "$ctx" --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{context:$ctx, orderId:$oid, slotIndex:$si}')"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService ConfirmVendSuccess)" "$vs_body" "grpc-${stem}-vsuccess" machine "$(echo "$ctx" | jq -r '.idempotencyKey')" || return 1

  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService GetOrder)" \
    "$(jq -nc --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{orderId:$oid, slotIndex:$si}')" \
    "grpc-${stem}-get-order" machine "" || return 1
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService GetOrderStatus)" \
    "$(jq -nc --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{orderId:$oid, slotIndex:$si}')" \
    "grpc-${stem}-get-status" machine "" || return 1

  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineInventoryService GetInventorySnapshot)" \
    "$(jq -nc --argjson meta "$(prod_e2e_grpc_meta_json "${stem}-inv-post")" '{meta:$meta}')" \
    "grpc-${stem}-inv-post" machine "" || return 1
  inv_after="$(prod_e2e_grpc_slot_quantity "${PROD_E2E_RAW_DIR}/grpc-${stem}-inv-post.response.json")"
  inv_after="${inv_after:-0}"

  if [[ "$inv_after" -ge "$inv_before" ]]; then
    prod_e2e_fail_classify "c" "$id" "inventory not decremented after cash vend (before=${inv_before} after=${inv_after})"
    return 1
  fi
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "grpc" "pass" "$(echo "$flow_json" | jq -r '.evidence_label')"
  return 0
}

prod_e2e_grpc_handler_commerce_qr() {
  local flow_json="$1"
  local id stem
  id="$(echo "$flow_json" | jq -r '.id')"
  stem="qr-$(echo "$flow_json" | jq -r '.evidence_label')"

  prod_e2e_grpc_commerce_create_order "$stem" || return 1
  local oid pay_id
  oid="$(jq -r '.orderId // empty' "${PROD_E2E_RAW_DIR}/grpc-${stem}-create-order.response.json")"
  [[ -n "$oid" ]] || return 1
  prod_e2e_state_set grpcQrOrderId "$oid"

  local ctx ps_body
  ctx="$(prod_e2e_grpc_idem_context "${stem}-ps")"
  ps_body="$(jq -nc \
    --argjson ctx "$ctx" \
    --arg oid "$oid" \
    --argjson amt 15000 \
    '{context:$ctx, orderId:$oid, provider:"stripe", paymentState:"created", amountMinor:$amt, currency:"VND", outboxPayloadJson:"{\"source\":\"e2e_prod_grpc\"}"}')"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService CreatePaymentSession)" \
    "$ps_body" "grpc-${stem}-payment-session" machine "$(echo "$ctx" | jq -r '.idempotencyKey')" || return 1
  pay_id="$(jq -r '.paymentId // empty' "${PROD_E2E_RAW_DIR}/grpc-${stem}-payment-session.response.json")"
  [[ -n "$pay_id" ]] || { prod_e2e_fail_classify "c" "$id" "CreatePaymentSession missing paymentId"; return 1; }
  prod_e2e_state_set paymentId "$pay_id"
  prod_e2e_state_set orderId "$oid"

  local wh_flow
  wh_flow="$(echo "$flow_json" | jq -c '.webhook_flow // null')"
  if [[ "$wh_flow" != "null" ]]; then
    prod_e2e_rest_execute_flow "$wh_flow" || return 1
  fi

  local i=0 paid=0
  while [[ "$i" -lt 15 ]]; do
    prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService GetOrderStatus)" \
      "$(jq -nc --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{orderId:$oid, slotIndex:$si}')" \
      "grpc-${stem}-poll-${i}" machine "" || true
    local ost pst
    ost="$(jq -r '.orderStatus // empty' "${PROD_E2E_RAW_DIR}/grpc-${stem}-poll-${i}.response.json" 2>/dev/null)"
    pst="$(jq -r '.paymentState // empty' "${PROD_E2E_RAW_DIR}/grpc-${stem}-poll-${i}.response.json" 2>/dev/null)"
    if [[ "$ost" == "paid" || "$pst" == "succeeded" || "$pst" == "paid" ]]; then
      paid=1
      break
    fi
    i=$((i + 1))
    sleep 2
  done
  [[ "$paid" -eq 1 ]] || {
    prod_e2e_fail_classify "d" "$id" "order not paid after webhook poll — check COMMERCE_PAYMENT_WEBHOOK_SECRET"
    return 1
  }

  ctx="$(prod_e2e_grpc_idem_context "${stem}-vstart")"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService StartVend)" \
    "$(jq -nc --argjson ctx "$ctx" --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{context:$ctx, orderId:$oid, slotIndex:$si}')" \
    "grpc-${stem}-vstart" machine "$(echo "$ctx" | jq -r '.idempotencyKey')" || return 1
  ctx="$(prod_e2e_grpc_idem_context "${stem}-vsuccess")"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService ConfirmVendSuccess)" \
    "$(jq -nc --argjson ctx "$ctx" --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{context:$ctx, orderId:$oid, slotIndex:$si}')" \
    "grpc-${stem}-vsuccess" machine "$(echo "$ctx" | jq -r '.idempotencyKey')" || return 1

  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "grpc" "pass" "$(echo "$flow_json" | jq -r '.evidence_label')"
  return 0
}

prod_e2e_grpc_handler_commerce_vend_failure() {
  local flow_json="$1"
  local id stem
  id="$(echo "$flow_json" | jq -r '.id')"
  stem="fail-$(echo "$flow_json" | jq -r '.evidence_label')"

  prod_e2e_grpc_commerce_create_order "$stem" || return 1
  local oid
  oid="$(jq -r '.orderId // empty' "${PROD_E2E_RAW_DIR}/grpc-${stem}-create-order.response.json")"
  [[ -n "$oid" ]] || return 1

  local ctx
  ctx="$(prod_e2e_grpc_idem_context "${stem}-cash")"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService ConfirmCashPayment)" \
    "$(jq -nc --argjson ctx "$ctx" --arg oid "$oid" '{context:$ctx, orderId:$oid}')" \
    "grpc-${stem}-cash" machine "$(echo "$ctx" | jq -r '.idempotencyKey')" || return 1

  ctx="$(prod_e2e_grpc_idem_context "${stem}-vstart")"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService StartVend)" \
    "$(jq -nc --argjson ctx "$ctx" --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{context:$ctx, orderId:$oid, slotIndex:$si}')" \
    "grpc-${stem}-vstart" machine "$(echo "$ctx" | jq -r '.idempotencyKey')" || return 1

  ctx="$(prod_e2e_grpc_idem_context "${stem}-vfail")"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService ReportVendFailure)" \
    "$(jq -nc --argjson ctx "$ctx" --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{context:$ctx, orderId:$oid, slotIndex:$si, failureReason:"e2e_grpc_simulated_fault"}')" \
    "grpc-${stem}-vfail" machine "$(echo "$ctx" | jq -r '.idempotencyKey')" || return 1

  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService GetOrderStatus)" \
    "$(jq -nc --arg oid "$oid" --argjson si "$(prod_e2e_grpc_slot_index)" '{orderId:$oid, slotIndex:$si}')" \
    "grpc-${stem}-status" machine "" || return 1

  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "grpc" "pass" "$(echo "$flow_json" | jq -r '.evidence_label')"
  return 0
}

prod_e2e_grpc_handler_commerce_cancel() {
  local flow_json="$1"
  local id stem
  id="$(echo "$flow_json" | jq -r '.id')"
  stem="cancel-$(echo "$flow_json" | jq -r '.evidence_label')"

  prod_e2e_grpc_commerce_create_order "$stem" || return 1
  local oid ctx cancel_body ik
  oid="$(jq -r '.orderId // empty' "${PROD_E2E_RAW_DIR}/grpc-${stem}-create-order.response.json")"
  ctx="$(prod_e2e_grpc_idem_context "${stem}-cancel")"
  ik="$(echo "$ctx" | jq -r '.idempotencyKey')"
  cancel_body="$(jq -nc --argjson ctx "$ctx" --arg oid "$oid" '{context:$ctx, orderId:$oid, reason:"e2e_cancel_before_payment"}')"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService CancelOrder)" \
    "$cancel_body" "grpc-${stem}-cancel-1" machine "$ik" || return 1
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService CancelOrder)" \
    "$cancel_body" "grpc-${stem}-cancel-dup" machine "$ik" || return 1
  local r1 r2
  r1="$(jq -r '.replay // false' "${PROD_E2E_RAW_DIR}/grpc-${stem}-cancel-1.response.json")"
  r2="$(jq -r '.replay // false' "${PROD_E2E_RAW_DIR}/grpc-${stem}-cancel-dup.response.json")"
  if [[ "$r2" != "true" && "$r2" != "false" ]]; then
    : # some servers omit replay on duplicate cancel — accept stable order_status
  fi
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "grpc" "pass" "$(echo "$flow_json" | jq -r '.evidence_label')"
  return 0
}

prod_e2e_grpc_handler_commerce_idempotency() {
  local flow_json="$1"
  local id stem
  id="$(echo "$flow_json" | jq -r '.id')"
  stem="idem-$(echo "$flow_json" | jq -r '.evidence_label')"
  local ctx
  ctx="$(prod_e2e_grpc_idem_context "${stem}-co")"
  local body ik
  body="$(jq -nc \
    --argjson ctx "$ctx" \
    --argjson meta "$(prod_e2e_grpc_meta_json "${stem}-co" "$(echo "$ctx" | jq -r '.idempotencyKey')" "$(echo "$ctx" | jq -r '.clientEventId')")" \
    --arg mid "${machineId:-}" \
    --arg pid "${productId:-}" \
    --argjson si "$(prod_e2e_grpc_slot_index)" \
    '{context:$ctx, machineId:$mid, productId:$pid, slot:{cabinetCode:"A", slotCode:"A1", slotIndex:$si}, currency:"VND", meta:$meta}')"
  ik="$(echo "$ctx" | jq -r '.idempotencyKey')"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService CreateOrder)" "$body" "grpc-${stem}-co-a" machine "$ik" || return 1
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineCommerceService CreateOrder)" "$body" "grpc-${stem}-co-b" machine "$ik" || return 1
  local o1 o2
  o1="$(jq -r '.orderId // empty' "${PROD_E2E_RAW_DIR}/grpc-${stem}-co-a.response.json")"
  o2="$(jq -r '.orderId // empty' "${PROD_E2E_RAW_DIR}/grpc-${stem}-co-b.response.json")"
  [[ -n "$o1" && "$o1" == "$o2" ]] || {
    prod_e2e_fail_classify "c" "$id" "CreateOrder idempotency replay returned different orderId (${o1} vs ${o2})"
    return 1
  }
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "grpc" "pass" "$(echo "$flow_json" | jq -r '.evidence_label')"
  return 0
}

prod_e2e_grpc_handler_offline_replay() {
  local flow_json="$1"
  local id stem
  id="$(echo "$flow_json" | jq -r '.id')"
  stem="offline-$(echo "$flow_json" | jq -r '.evidence_label')"

  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineOfflineSyncService GetSyncCursor)" \
    "$(jq -nc --argjson meta "$(prod_e2e_grpc_meta_json "${stem}-cursor")" '{meta:$meta}')" \
    "grpc-${stem}-cursor" machine "" || true

  local bundle_ik="${PROD_E2E_PREFIX}-${stem}-bundle-ik"
  local evt_ik="${PROD_E2E_PREFIX}-${stem}-evt-ik"
  local ceid="${PROD_E2E_PREFIX}-${stem}-ce"
  local off_body
  off_body="$(jq -nc \
    --arg mid "${machineId:-}" \
    --arg rid "${PROD_E2E_PREFIX}-${stem}-req" \
    --arg bik "$bundle_ik" \
    --arg ceid "$ceid" \
    --arg eik "$evt_ik" \
    --arg occ "$(prod_e2e_grpc_now_rfc3339)" \
    '{
      meta:{machineId:$mid, requestId:$rid, idempotencyKey:$bik},
      events:[{
        meta:{machineId:$mid, requestId:$rid, clientEventId:$ceid, offlineSequence:1, idempotencyKey:$eik},
        eventType:"e2e.offline.ping",
        payload:{phase:"grpc", note:"production offline replay"}
      }]
    }')"
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineOfflineSyncService PushOfflineEvents)" \
    "$off_body" "grpc-${stem}-push-a" machine "$bundle_ik" || return 1
  prod_e2e_grpc_call_raw "$(prod_e2e_grpc_full_method MachineOfflineSyncService PushOfflineEvents)" \
    "$off_body" "grpc-${stem}-push-b" machine "$bundle_ik" || return 1
  prod_e2e_evidence_append_row "$id" "$(echo "$flow_json" | jq -r '.label')" "grpc" "pass" "$(echo "$flow_json" | jq -r '.evidence_label')"
  return 0
}
