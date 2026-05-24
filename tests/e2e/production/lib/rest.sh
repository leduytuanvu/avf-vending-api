#!/usr/bin/env bash
# shellcheck shell=bash
# REST flow executor — saves raw artifacts under .e2e-runs/production/<runId>/raw/

prod_e2e_rest_expected_includes_status() {
  local expected="$1"
  local status="$2"
  if echo "$expected" | jq -e 'type == "array"' >/dev/null 2>&1; then
    echo "$expected" | jq -e --argjson s "$status" 'index($s) != null' >/dev/null 2>&1
    return $?
  fi
  [[ "$expected" == "$status" ]]
}

prod_e2e_rest_curl_once() {
  local method="$1"
  local -n _opts="$2"
  local url="$3"
  local body="$4"
  case "$method" in
    GET) curl "${_opts[@]}" "$url" || true ;;
    POST|PUT|PATCH) curl "${_opts[@]}" -X "$method" -d "$body" "$url" || true ;;
    DELETE) curl "${_opts[@]}" -X DELETE "$url" || true ;;
    *) echo "000" ;;
  esac
}

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
    GET|POST|PUT|PATCH|DELETE)
      code="$(prod_e2e_rest_curl_once "$method" curl_opts "$url" "$body")"
      ;;
    *)
      echo "REST unsupported method ${method} for ${id}" >&2
      return 1
      ;;
  esac

  if [[ "${PROD_E2E_RETRY_ON_AUTH_401:-1}" == "1" && "$code" == "401" && "$auth" == "bearer_admin" ]] \
    && ! prod_e2e_rest_expected_includes_status "$expected_status" "401"; then
    if prod_e2e_refresh_admin_token "${PROD_E2E_REST_COV_INDEX:-0}"; then
      prod_e2e_state_reload_key accessToken || true
      local -a retry_opts=()
      local opt
      for opt in "${curl_opts[@]}"; do
        [[ "$opt" == Authorization:* ]] && continue
        retry_opts+=("$opt")
      done
      if [[ -n "${accessToken:-}" ]]; then
        retry_opts+=(-H "Authorization: Bearer ${accessToken}")
        code="$(prod_e2e_rest_curl_once "$method" retry_opts "$url" "$body")"
      fi
    fi
  fi

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
prod_e2e_admin_token_age_sec() {
  local issued="${accessTokenIssuedAt:-0}"
  [[ "$issued" =~ ^[0-9]+$ ]] || { echo "0"; return 0; }
  echo "$(( $(date +%s) - issued ))"
}

prod_e2e_refresh_admin_token() {
  local flow_index="${1:-0}"
  [[ "${PROD_E2E_DRY_RUN:-}" == "1" ]] && return 0
  prod_e2e_state_reload_key refreshToken || true
  prod_e2e_state_reload_key accessToken || true
  prod_e2e_state_reload_key accessTokenIssuedAt || true

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
      local at rt now
      at="$(jq -r '.tokens.accessToken // .accessToken // empty' "$resp_file")"
      rt="$(jq -r '.tokens.refreshToken // .refreshToken // empty' "$resp_file")"
      now="$(date +%s)"
      [[ -n "$at" ]] && prod_e2e_state_set accessToken "$at"
      [[ -n "$rt" ]] && prod_e2e_state_set refreshToken "$rt"
      prod_e2e_state_set accessTokenIssuedAt "$now"
      echo "AUTH_REFRESH=OK flow_index=${flow_index} token_age_sec=0"
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
      local at rt now
      at="$(jq -r '.tokens.accessToken // empty' "$login_resp")"
      rt="$(jq -r '.tokens.refreshToken // empty' "$login_resp")"
      now="$(date +%s)"
      [[ -n "$at" ]] && prod_e2e_state_set accessToken "$at"
      [[ -n "$rt" ]] && prod_e2e_state_set refreshToken "$rt"
      prod_e2e_state_set accessTokenIssuedAt "$now"
      echo "AUTH_REFRESH=OK flow_index=${flow_index} token_age_sec=0"
      return 0
    fi
  fi

  echo "WARN: could not refresh admin token (flow_index=${flow_index} token_age_sec=$(prod_e2e_admin_token_age_sec))" >&2
  return 1
}

prod_e2e_init_report_window() {
  local to from
  to="$(date -u +%Y-%m-%dT23:59:59Z 2>/dev/null || true)"
  from="$(date -u -d '90 days ago' +%Y-%m-%dT00:00:00Z 2>/dev/null || date -u -v-90d +%Y-%m-%dT00:00:00Z 2>/dev/null || true)"
  [[ -z "$from" ]] && from="2026-03-01T00:00:00Z"
  [[ -z "$to" ]] && to="2026-05-23T23:59:59Z"
  export PROD_E2E_REPORT_FROM="$from"
  export PROD_E2E_REPORT_TO="$to"
  prod_e2e_state_set reportFrom "$from"
  prod_e2e_state_set reportTo "$to"
}

prod_e2e_seed_rest_coverage_context() {
  prod_e2e_init_report_window
  if [[ -z "${commandId:-}" && -n "${ADMIN_TOKEN:-}" && -n "${PROD_E2E_BASE_URL:-}" ]]; then
    local resp code cid
    resp="$(curl -sS -L \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H "Content-Type: application/json" \
      "${PROD_E2E_BASE_URL}/v1/admin/commands?limit=1" \
      -w $'\nHTTP %{http_code}' 2>/dev/null || true)"
    code="$(printf '%s' "$resp" | tail -n1 | sed -n 's/.*HTTP \([0-9][0-9][0-9]\).*/\1/p')"
    if [[ "$code" == "200" ]]; then
      cid="$(printf '%s' "$resp" | sed '$d' | jq -r '.items[0].commandId // empty' 2>/dev/null || true)"
      [[ -n "$cid" ]] && prod_e2e_state_set commandId "$cid"
    fi
  fi
  if [[ -z "${orderId:-}" && -n "${grpcCashOrderId:-}" ]]; then
    prod_e2e_state_set orderId "$grpcCashOrderId"
  fi
}

prod_e2e_rest_run_flow() {
  local flow_json="$1"
  local skip_if skip_if_empty id label evidence_label
  skip_if="$(echo "$flow_json" | jq -r '.skip_if_env // empty')"
  if [[ -n "$skip_if" && -n "${!skip_if:-}" ]]; then
    id="$(echo "$flow_json" | jq -r '.id // ""')"
    label="$(echo "$flow_json" | jq -r '.label // ""')"
    evidence_label="$(echo "$flow_json" | jq -r '.evidence_label // ""')"
    prod_e2e_evidence_append_row "$id" "$label" "rest" "skipped" "$evidence_label"
    return 0
  fi
  skip_if_empty="$(echo "$flow_json" | jq -r '.skip_if_empty // empty')"
  if [[ -n "$skip_if_empty" && -z "${!skip_if_empty:-}" ]]; then
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
