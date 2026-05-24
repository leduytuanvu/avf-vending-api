#!/usr/bin/env bash
# shellcheck shell=bash
# REST flow executor — saves raw artifacts under .e2e-runs/production/<runId>/raw/

prod_e2e_rest_execute_flow() {
  local flow_json="$1"
  local id label method path auth evidence_label expected_status optional
  id="$(echo "$flow_json" | jq -r '.id')"
  label="$(echo "$flow_json" | jq -r '.label')"
  method="$(echo "$flow_json" | jq -r '.method')"
  path="$(echo "$flow_json" | jq -r '.path')"
  auth="$(echo "$flow_json" | jq -r '.auth // "none"')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label')"
  expected_status="$(echo "$flow_json" | jq -c '.expected_status // 200')"
  optional="$(echo "$flow_json" | jq -r '.optional // false')"

  local skip_if
  skip_if="$(echo "$flow_json" | jq -r '.skip_if_env // empty')"
  if [[ -n "$skip_if" && -n "${!skip_if:-}" ]]; then
    prod_e2e_evidence_append_row "$id" "$label" "rest" "skipped" "$evidence_label"
    return 0
  fi

  path="$(prod_e2e_render_template_string "$path")"
  local url="${BASE_URL%/}${path}"
  local req_file="${PROD_E2E_RAW_DIR}/${evidence_label}.request.json"
  local resp_file="${PROD_E2E_RAW_DIR}/${evidence_label}.response.json"
  local meta_file="${PROD_E2E_RAW_DIR}/${evidence_label}.meta.json"
  local hdr_file="${PROD_E2E_RAW_DIR}/${evidence_label}.response.headers.txt"

  local body=""
  local tpl_file
  tpl_file="$(echo "$flow_json" | jq -r '.request_template_file // empty')"
  if [[ -n "$tpl_file" ]]; then
    local fixture="${PROD_E2E_PRODUCTION_DIR}/fixtures/${tpl_file#fixtures/}"
    body="$(prod_e2e_render_json_template "$fixture")"
  else
    body="$(echo "$flow_json" | jq -c '.request_template // {}')"
    body="$(prod_e2e_coerce_json_body "$(prod_e2e_render_template_string "$body")")"
  fi

  jq -nc \
    --arg method "$method" \
    --arg path "$path" \
    --arg url "$url" \
    --arg auth "$auth" \
    --argjson body "$body" \
    '{method:$method,path:$path,url:$url,auth:$auth,body:$body}' >"$req_file" 2>/dev/null || {
    printf '{"method":"%s","path":"%s","url":"%s","body":%s}\n' "$method" "$path" "$url" "$body" >"$req_file"
  }
  prod_e2e_redact_file "$req_file" "${req_file}.redacted"
  mv "${req_file}.redacted" "$req_file"

  if [[ "${PROD_E2E_DRY_RUN:-}" == "1" ]]; then
    prod_e2e_evidence_append_row "$id" "$label" "rest" "dry-run" "$evidence_label"
    return 0
  fi

  local -a curl_opts=(--globoff -sS -L -D "$hdr_file" -o "$resp_file" -w '%{http_code}')
  case "$auth" in
    bearer_admin)
      if declare -F prod_e2e_state_reload_key >/dev/null 2>&1; then
        prod_e2e_state_reload_key accessToken || true
        prod_e2e_state_reload_key ADMIN_TOKEN || true
      fi
      if [[ -n "${accessToken:-}" ]]; then
        curl_opts+=(-H "Authorization: Bearer ${accessToken}")
      elif [[ -n "${ADMIN_TOKEN:-}" ]]; then
        curl_opts+=(-H "Authorization: Bearer ${ADMIN_TOKEN}")
      fi
      ;;
    bearer_machine)
      curl_opts+=(-H "Authorization: Bearer ${machineToken:-}")
      ;;
    webhook_hmac)
      local ts sig
      ts="$(date +%s)"
      sig="$(prod_e2e_webhook_signature "$body" "$ts" "${COMMERCE_PAYMENT_WEBHOOK_SECRET:-}")"
      curl_opts+=(-H "X-AVF-Webhook-Timestamp: ${ts}" -H "X-AVF-Webhook-Signature: ${sig}")
      ;;
    webhook_hmac_stale)
      local ts sig
      ts="$(( $(date +%s) - 86400 ))"
      sig="$(prod_e2e_webhook_signature "$body" "$ts" "${COMMERCE_PAYMENT_WEBHOOK_SECRET:-}")"
      curl_opts+=(-H "X-AVF-Webhook-Timestamp: ${ts}" -H "X-AVF-Webhook-Signature: ${sig}")
      ;;
    webhook_hmac_invalid)
      curl_opts+=(-H "X-AVF-Webhook-Timestamp: 0" -H "X-AVF-Webhook-Signature: invalid")
      ;;
  esac

  if [[ "$(echo "$flow_json" | jq -r '.idempotency // false')" == "true" ]]; then
    curl_opts+=(-H "Idempotency-Key: ${PROD_E2E_PREFIX}-${id}")
  fi
  curl_opts+=(-H "Content-Type: application/json")

  local code=""
  case "$method" in
    GET)
      code="$(curl "${curl_opts[@]}" "$url" || true)"
      ;;
    POST|PUT|PATCH)
      code="$(curl "${curl_opts[@]}" -X "$method" -d "$body" "$url" || true)"
      ;;
    DELETE)
      code="$(curl "${curl_opts[@]}" -X DELETE "$url" || true)"
      ;;
    *)
      echo "REST unsupported method ${method} for ${id}" >&2
      return 1
      ;;
  esac

  jq -nc --arg method "$method" --arg path "$path" --argjson code "${code:-0}" \
    '{method:$method,path:$path,http_code:$code}' >"$meta_file"

  local status="pass"
  if ! prod_e2e_assert_http_status "$expected_status" "$code" "$id"; then
    status="fail"
    if [[ "$optional" == "true" ]]; then
      status="optional-fail"
    else
      prod_e2e_evidence_append_row "$id" "$label" "rest" "$status" "$evidence_label"
      prod_e2e_evidence_append_section "$id" "$evidence_label" "${method} ${path}" "$code" "$req_file" "$resp_file"
      return 1
    fi
  fi

  local assertions
  assertions="$(echo "$flow_json" | jq -c '.assertions // []')"
  if [[ "$assertions" != "[]" && "$assertions" != "null" ]]; then
    while IFS= read -r a; do
      [[ -n "$a" ]] || continue
      prod_e2e_assert_body_file "$resp_file" "$id" "$a" || status="fail"
    done < <(echo "$assertions" | jq -c '.[]')
  fi

  local capture
  capture="$(echo "$flow_json" | jq -c '.capture // null')"
  prod_e2e_capture_from_body "$resp_file" "$capture"

  prod_e2e_evidence_append_row "$id" "$label" "rest" "$status" "$evidence_label"
  prod_e2e_evidence_append_section "$id" "$evidence_label" "${method} ${path}" "$code" "$req_file" "$resp_file"
  [[ "$status" == "fail" ]] && return 1
  return 0
}

# Re-authenticate admin before long REST-COV phase (JWT may expire during main manifest).
prod_e2e_refresh_admin_token() {
  [[ "${PROD_E2E_DRY_RUN:-}" == "1" ]] && return 0
  prod_e2e_state_reload_key refreshToken || true
  prod_e2e_state_reload_key accessToken || true
  prod_e2e_state_reload_key ADMIN_TOKEN || true

  if [[ -n "${refreshToken:-}" ]]; then
    local body resp_file code
    body="$(jq -nc --arg rt "$refreshToken" '{refreshToken:$rt}')"
    resp_file="${PROD_E2E_RAW_DIR}/rest-auth-refresh-pre-coverage.response.json"
    mkdir -p "${PROD_E2E_RAW_DIR}"
    code="$(curl -sS -o "$resp_file" -w '%{http_code}' \
      -H "Content-Type: application/json" \
      -X POST "${BASE_URL%/}/v1/auth/refresh" \
      -d "$body" 2>/dev/null || echo "000")"
    if [[ "$code" == "200" ]] && jq -e '.tokens.accessToken // .accessToken' "$resp_file" >/dev/null 2>&1; then
      local at rt
      at="$(jq -r '.tokens.accessToken // .accessToken // empty' "$resp_file")"
      rt="$(jq -r '.tokens.refreshToken // .refreshToken // empty' "$resp_file")"
      [[ -n "$at" ]] && prod_e2e_state_set accessToken "$at"
      [[ -n "$rt" ]] && prod_e2e_state_set refreshToken "$rt"
      echo "ADMIN_TOKEN_REFRESH ok (pre REST-COV)"
      return 0
    fi
  fi

  if [[ -n "${ADMIN_EMAIL:-}" && -n "${ADMIN_PASSWORD:-}" ]]; then
    local login_body login_resp code
    login_body="$(jq -nc --arg e "$ADMIN_EMAIL" --arg p "$ADMIN_PASSWORD" '{email:$e,password:$p}')"
    login_resp="${PROD_E2E_RAW_DIR}/rest-auth-login-pre-coverage.response.json"
    code="$(curl -sS -o "$login_resp" -w '%{http_code}' \
      -H "Content-Type: application/json" \
      -X POST "${BASE_URL%/}/v1/auth/login" \
      -d "$login_body" 2>/dev/null || echo "000")"
    if [[ "$code" == "200" ]]; then
      local at rt
      at="$(jq -r '.tokens.accessToken // empty' "$login_resp")"
      rt="$(jq -r '.tokens.refreshToken // empty' "$login_resp")"
      [[ -n "$at" ]] && prod_e2e_state_set accessToken "$at"
      [[ -n "$rt" ]] && prod_e2e_state_set refreshToken "$rt"
      echo "ADMIN_TOKEN_RELOGIN ok (pre REST-COV)"
      return 0
    fi
  fi

  echo "WARN: could not refresh admin token before REST-COV" >&2
  return 1
}

prod_e2e_rest_run_flow() {
  local flow_json="$1"
  local skip_if id label evidence_label
  skip_if="$(echo "$flow_json" | jq -r '.skip_if_env // empty')"
  if [[ -n "$skip_if" && -n "${!skip_if:-}" ]]; then
    id="$(echo "$flow_json" | jq -r '.id // ""')"
    label="$(echo "$flow_json" | jq -r '.label // ""')"
    evidence_label="$(echo "$flow_json" | jq -r '.evidence_label // ""')"
    prod_e2e_evidence_append_row "$id" "$label" "rest" "skipped" "$evidence_label"
    return 0
  fi
  local handler
  handler="$(echo "$flow_json" | jq -r '.handler // empty')"
  if [[ -n "$handler" && "$handler" != "null" ]]; then
    prod_e2e_rest_dispatch "$flow_json"
  else
    prod_e2e_rest_execute_flow "$flow_json"
  fi
}

prod_e2e_webhook_signature() {
  local body="$1"
  local ts="$2"
  local secret="$3"
  prod_e2e_py -c '
import hmac, hashlib, sys, os
body, ts, secret = sys.argv[1], sys.argv[2], sys.argv[3]
if not secret:
    print("unsigned-local-dev")
else:
    dig = hmac.new(secret.encode(), (ts + ".").encode() + body.encode(), hashlib.sha256).hexdigest()
    print(dig)
' "$body" "$ts" "$secret"
}
