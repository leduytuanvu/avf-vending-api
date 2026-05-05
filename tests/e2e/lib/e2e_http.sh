#!/usr/bin/env bash
# shellcheck shell=bash
# REST helpers. Requires e2e_common.sh, BASE_URL, E2E_RUN_DIR.

e2e_http_log_dir() {
  echo "${E2E_RUN_DIR}/rest"
}

_e2e_http_write_meta() {
  local step="$1"
  local code="$2"
  local elapsed_ms="$3"
  local dir
  dir="$(e2e_http_log_dir)"
  jq -nc \
    --arg step "$step" \
    --argjson code "$code" \
    --argjson elapsed_ms "$elapsed_ms" \
    '{step:$step,httpStatus:$code,elapsedMs:$elapsed_ms}' >"${dir}/${step}.meta.json"
}

e2e_http_request_json() {
  local method="$1"
  local step="$2"
  local path="$3"
  local body_file="${4:-}"
  local dir
  dir="$(e2e_http_log_dir)"
  mkdir -p "$dir"

  local url="${BASE_URL%/}${path}"
  local req="${dir}/${step}.request.json"
  local tmp_hdr
  tmp_hdr="$(mktemp)"

  local -a curl_opts=(-sS -D "${tmp_hdr}" -o "${dir}/${step}.response.json" -w '%{http_code}|%{time_total}')
  curl_opts+=(-X "$method" -H "Content-Type: application/json")

  if [[ -n "${ADMIN_TOKEN:-}" ]]; then
    curl_opts+=(-H "Authorization: Bearer ${ADMIN_TOKEN}")
  fi

  if [[ -n "$body_file" ]]; then
    curl_opts+=(-d @"${body_file}")
  fi

  jq -nc \
    --arg method "$method" \
    --arg url "$url" \
    --arg tokenSet "$( [[ -n "${ADMIN_TOKEN:-}" ]] && echo true || echo false)" \
    '{method:$method,url:$url,headers:{"Content-Type":"application/json",Authorization:(if $tokenSet=="true" then "Bearer ***" else null end)}}' >"${req}.tmp"
  mv "${req}.tmp" "$req"

  local out
  out="$(curl "${curl_opts[@]}" "$url" 2>/dev/null || true)"
  local code body_time
  code="${out%%|*}"
  body_time="${out##*|}"
  local elapsed_ms=0
  if [[ "$body_time" =~ ^[0-9.]+$ ]]; then
    elapsed_ms="$(python3 -c "print(int(round(float('${body_time}')*1000)))" 2>/dev/null || echo 0)"
  fi

  # Fallback if curl failed completely
  [[ -n "$code" ]] || code=0

  _e2e_http_write_meta "$step" "$code" "$elapsed_ms"
  rm -f "${tmp_hdr}"
  echo "$code"
}

e2e_http_get() {
  local step="$1"
  local path="$2"
  e2e_http_request_json "GET" "$step" "$path" ""
}

e2e_http_delete() {
  local step="$1"
  local path="$2"
  e2e_http_request_json "DELETE" "$step" "$path" ""
}

e2e_http_post_json() {
  local step="$1"
  local path="$2"
  local json_body="$3"
  local dir
  dir="$(e2e_http_log_dir)"
  local body_file="${dir}/${step}.body.json"
  echo "$json_body" >"$body_file"
  e2e_http_request_json "POST" "$step" "$path" "$body_file"
}

e2e_http_put_json() {
  local step="$1"
  local path="$2"
  local json_body="$3"
  local dir
  dir="$(e2e_http_log_dir)"
  local body_file="${dir}/${step}.body.json"
  echo "$json_body" >"$body_file"
  e2e_http_request_json "PUT" "$step" "$path" "$body_file"
}

e2e_http_patch_json() {
  local step="$1"
  local path="$2"
  local json_body="$3"
  local dir
  dir="$(e2e_http_log_dir)"
  local body_file="${dir}/${step}.body.json"
  echo "$json_body" >"$body_file"
  e2e_http_request_json "PATCH" "$step" "$path" "$body_file"
}

e2e_http_assert_status() {
  local step="$1"
  local expected="$2"
  local actual="$3"
  if [[ "$actual" != "$expected" ]]; then
    fail_step "HTTP ${step}: expected status ${expected}, got ${actual}"
    return 1
  fi
  return 0
}

e2e_jq_resp() {
  local step="$1"
  local jqexpr="$2"
  local dir
  dir="$(e2e_http_log_dir)"
  jq "$jqexpr" "${dir}/${step}.response.json"
}
