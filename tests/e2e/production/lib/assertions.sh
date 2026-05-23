#!/usr/bin/env bash
# shellcheck shell=bash
# Response assertions for production E2E flows.

prod_e2e_assert_http_status() {
  local expected="$1"
  local actual="$2"
  local label="$3"
  if echo "$expected" | jq -e 'type == "array"' >/dev/null 2>&1; then
    echo "$expected" | jq -e --argjson a "$actual" 'index($a) != null' >/dev/null 2>&1 && return 0
    echo "ASSERT FAIL ${label}: expected HTTP one of $(echo "$expected" | jq -c .), got ${actual}" >&2
    if declare -F prod_e2e_fail_classify >/dev/null 2>&1; then
      prod_e2e_fail_classify "c" "$label" "expected HTTP $(echo "$expected" | jq -c .) got ${actual}"
    fi
    return 1
  fi
  if [[ "$actual" != "$expected" ]]; then
    echo "ASSERT FAIL ${label}: expected HTTP ${expected}, got ${actual}" >&2
    if declare -F prod_e2e_fail_classify >/dev/null 2>&1; then
      prod_e2e_fail_classify "c" "$label" "expected HTTP ${expected} got ${actual} — verify OpenAPI/manifest alignment"
    fi
    return 1
  fi
  return 0
}

prod_e2e_jq_path() {
  local p="$1"
  [[ "$p" == .* ]] || p=".${p}"
  printf '%s' "$p"
}

prod_e2e_assert_body_file() {
  local body_file="$1"
  shift
  local label="$1"
  shift
  [[ -f "$body_file" ]] || { echo "ASSERT FAIL ${label}: missing body ${body_file}" >&2; return 1; }
  [[ -s "$body_file" ]] || { echo "ASSERT FAIL ${label}: empty body" >&2; return 1; }
  if ! command -v jq >/dev/null 2>&1; then
    return 0
  fi
  local assertion type path value
  for assertion in "$@"; do
    type="$(echo "$assertion" | jq -r '.type // empty')"
    path="$(echo "$assertion" | jq -r '.path // empty')"
    value="$(echo "$assertion" | jq -r '.value // empty')"
    case "$type" in
      body_not_empty) ;;
      json_path_exists)
        jq -e "$(prod_e2e_jq_path "$path") != null" "$body_file" >/dev/null || {
          echo "ASSERT FAIL ${label}: json path missing ${path}" >&2
          return 1
        }
        ;;
      json_path_equals)
        local got jpath
        jpath="$(prod_e2e_jq_path "$path")"
        got="$(jq -r "${jpath} // empty" "$body_file")"
        if [[ "$got" != "$value" ]]; then
          echo "ASSERT FAIL ${label}: ${path} expected ${value}, got ${got}" >&2
          return 1
        fi
        ;;
      *)
        echo "ASSERT WARN ${label}: unknown assertion type ${type}" >&2
        ;;
    esac
  done
  return 0
}

prod_e2e_capture_from_body() {
  local body_file="$1"
  local capture_json="$2"
  [[ -n "$capture_json" && "$capture_json" != "null" ]] || return 0
  local keys key jqpath val capture_tmp
  keys="$(echo "$capture_json" | jq -r 'keys[]?' 2>/dev/null || true)"
  [[ -n "$keys" ]] || return 0
  capture_tmp="${PROD_E2E_RUN_DIR:-/tmp}/.capture.tmp"
  while IFS= read -r key; do
    key="${key//$'\r'/}"
    [[ -n "$key" ]] || continue
    jqpath="$(echo "$capture_json" | jq -r --arg k "$key" '.[$k]' 2>/dev/null || true)"
    jqpath="${jqpath//$'\r'/}"
    [[ -n "$jqpath" ]] || continue
    jqpath="$(prod_e2e_jq_path "$jqpath")"
    val=""
    local cap_py="${PROD_E2E_PRODUCTION_DIR:-${PROD_E2E_REPO_ROOT}/tests/e2e/production}/scripts/capture_json_path.py"
    if [[ -f "$cap_py" ]]; then
      if declare -F prod_e2e_py >/dev/null 2>&1; then
        val="$(prod_e2e_py "$cap_py" "$body_file" "$jqpath" 2>/dev/null || true)"
      elif command -v python3 >/dev/null 2>&1; then
        val="$(python3 "$cap_py" "$body_file" "$jqpath" 2>/dev/null || true)"
      elif command -v py >/dev/null 2>&1; then
        val="$(py -3 "$cap_py" "$body_file" "$jqpath" 2>/dev/null || true)"
      fi
    elif command -v jq >/dev/null 2>&1; then
      jq -r "${jqpath} // empty" "$body_file" >"$capture_tmp" 2>/dev/null || : >"$capture_tmp"
      val="$(tr -d '\r\n' <"$capture_tmp")"
      rm -f "$capture_tmp"
    fi
    val="${val//$'\r'/}"
    [[ -n "$val" ]] || continue
    prod_e2e_state_set "$key" "$val"
  done <<<"$keys"
}
