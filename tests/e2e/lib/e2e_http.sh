#!/usr/bin/env bash
# shellcheck shell=bash
# REST helpers. Requires e2e_common.sh, BASE_URL, E2E_RUN_DIR.

e2e_http_log_dir() {
  echo "${E2E_RUN_DIR}/rest"
}

_e2e_http_elapsed_ms() {
  local body_time="$1"
  if [[ "$body_time" =~ ^[0-9.]+$ ]]; then
    python3 -c "print(int(round(float('${body_time}')*1000)))" 2>/dev/null || echo 0
  else
    echo 0
  fi
}

_e2e_http_content_type_from_curl_d() {
  local hdr_file="$1"
  [[ -f "$hdr_file" ]] || { echo ""; return 0; }
  awk -F': ' 'tolower($1)=="content-type"{gsub(/\r/,"",$2); print $2; exit}' "$hdr_file"
}

# GET public or authed; logs body + raw headers + meta; appends events.jsonl.
# mode: required (default) | optional404 — 404 on optional is skipped, not failed.
# use_auth: true | false — send ADMIN_TOKEN when true and set.
# Returns 0 on success or optional skip; 1 on failure.
e2e_http_get_capture() {
  local step="$1"
  local path="$2"
  local mode="${3:-required}"
  local use_auth="${4:-false}"

  local dir
  dir="$(e2e_http_log_dir)"
  mkdir -p "$dir"

  local url="${BASE_URL%/}${path}"
  local hdr="${dir}/${step}.response.headers.txt"
  local body="${dir}/${step}.response.body"
  local req="${dir}/${step}.request.json"
  local meta="${dir}/${step}.meta.json"

  jq -nc \
    --arg method GET \
    --arg path "$path" \
    --arg url "$url" \
    --argjson auth "$( [[ "$use_auth" == "true" ]] && echo true || echo false)" \
    '{method:$method,path:$path,url:$url,auth:$auth}' >"${req}"

  local -a opts=(-sS -L --max-redirs 3 -D "$hdr" -o "$body" -w '%{http_code}|%{time_total}')
  if [[ "$use_auth" == "true" ]] && [[ -n "${ADMIN_TOKEN:-}" ]]; then
    opts+=(-H "Authorization: Bearer ${ADMIN_TOKEN}")
  fi

  local out
  out="$(curl "${opts[@]}" "$url" 2>/dev/null || true)"
  local code="${out%%|*}"
  local body_time="${out##*|}"
  [[ -n "$code" ]] || code=0
  local elapsed_ms
  elapsed_ms="$(_e2e_http_elapsed_ms "$body_time")"
  local ct
  ct="$(_e2e_http_content_type_from_curl_d "$hdr")"

  local result="passed"
  if [[ "$mode" == "optional404" ]] && [[ "$code" == "404" ]]; then
    result="skipped"
  elif [[ "$code" != "200" ]]; then
    result="failed"
  fi

  jq -nc \
    --arg step "$step" \
    --arg method GET \
    --arg path "$path" \
    --arg url "$url" \
    --argjson code "$code" \
    --argjson elapsed_ms "$elapsed_ms" \
    --arg mode "$mode" \
    --arg contentType "${ct:-}" \
    --arg headersFile "$(basename "$hdr")" \
    --arg bodyFile "$(basename "$body")" \
    --arg result "$result" \
    '{
      step:$step,
      method:$method,
      path:$path,
      url:$url,
      httpStatus:$code,
      elapsedMs:$elapsed_ms,
      mode:$mode,
      contentType:$contentType,
      headersFile:$headersFile,
      bodyFile:$bodyFile,
      result:$result
    }' >"${meta}"

  local ev_status="$result"
  local ev_msg="GET ${path} HTTP ${code} (${elapsed_ms}ms)"
  append_event_jsonl "http:${step}" "$ev_status" "$ev_msg"

  if [[ "$result" == "skipped" ]]; then
    return 0
  fi
  if [[ "$result" == "failed" ]]; then
    return 1
  fi
  return 0
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
  elapsed_ms="$(_e2e_http_elapsed_ms "$body_time")"

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
