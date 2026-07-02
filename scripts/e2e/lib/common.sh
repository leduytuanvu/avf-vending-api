#!/usr/bin/env bash
# shellcheck shell=bash
# Shared helpers for scripts/e2e production harnesses.

set -Eeuo pipefail

e2e_repo_root() {
  local here
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
  printf '%s' "$here"
}

e2e_now_utc() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

e2e_python() {
  if [[ -n "${E2E_PYTHON:-}" ]]; then
    echo "${E2E_PYTHON}"
    return 0
  fi
  if command -v py >/dev/null 2>&1 && py -3 -c "import sys" >/dev/null 2>&1; then
    echo "py -3"
    return 0
  fi
  if command -v python3 >/dev/null 2>&1 && python3 -c "import sys" >/dev/null 2>&1; then
    echo python3
    return 0
  fi
  echo python3
}

e2e_py() {
  # shellcheck disable=SC2046
  $(e2e_python) "$@"
}

e2e_require_cmd() {
  local c
  for c in "$@"; do
    command -v "$c" >/dev/null 2>&1 || {
      echo "FATAL: required command not found: ${c}" >&2
      exit 127
    }
  done
}

e2e_init_run_dir() {
  local prefix="${1:-production-e2e}"
  E2E_RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
  E2E_RUN_DIR="${E2E_RUN_DIR:-${E2E_OUTPUT_DIR:-$(e2e_repo_root)/reports/e2e}/${prefix}/${E2E_RUN_TS}}"
  mkdir -p "${E2E_RUN_DIR}/raw" "${E2E_RUN_DIR}/logs"
  export E2E_RUN_DIR E2E_RUN_TS
}

e2e_record_probe() {
  local name="$1"
  local outcome="$2"
  local detail="${3:-}"
  local latency_ms="${4:-0}"
  printf '%s\n' "${name}|${outcome}|${latency_ms}|${detail}" >>"${E2E_RUN_DIR}/probes.tsv"
}

e2e_snippet() {
  local file="$1"
  e2e_py - "$file" <<'PY'
import re, sys
from pathlib import Path
text = Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace")[:400]
text = re.sub(r'(?i)("?(password|token|secret|authorization|refresh_token|access_token)"?\s*[:=]\s*)"?[^",\s}]+"?', r'\1"***REDACTED***"', text)
print(text.replace("\n", " ").replace("\r", " "), end="")
PY
}

e2e_curl_get() {
  local name="$1"
  local url="$2"
  local token="${3:-}"
  local out="${E2E_RUN_DIR}/raw/${name}.body"
  local meta="${E2E_RUN_DIR}/raw/${name}.meta"
  local -a hdr=( -H "Accept: application/json" -H "X-Request-ID: e2e-${E2E_RUN_TS}-${name}" )
  if [[ -n "$token" ]]; then
    hdr+=( -H "Authorization: Bearer ${token}" )
  fi
  local start end code
  start="$(e2e_py -c 'import time; print(int(time.time()*1000))')"
  set +e
  code="$(curl -sS -o "$out" -w '%{http_code}' --connect-timeout 8 --max-time 20 "${hdr[@]}" "$url")"
  local ec=$?
  set -e
  end="$(e2e_py -c 'import time; print(int(time.time()*1000))')"
  local lat=$((end - start))
  if [[ "$ec" -ne 0 ]]; then
    code="000"
  fi
  jq -nc --arg name "$name" --arg url "$url" --argjson code "${code:-0}" --argjson latency_ms "$lat" \
    '{name:$name,url:$url,http_code:$code,latency_ms:$latency_ms}' >"$meta"
  echo "$code" "$lat"
}

e2e_admin_token_trim() {
  local tok="${1:-}"
  tok="${tok#"${tok%%[![:space:]]*}"}"
  tok="${tok%"${tok##*[![:space:]]}"}"
  printf '%s' "$tok"
}

e2e_redact_tokens_in_json_file() {
  local f="${1:-}"
  [[ -f "$f" ]] || return 0
  if command -v sed >/dev/null 2>&1; then
    sed -i -E \
      -e 's/("accessToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      -e 's/("refreshToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      -e 's/("machineToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      "$f" 2>/dev/null || true
  fi
}

e2e_write_redacted_activation_code_create_summary() {
  local body_file="${1:-}"
  local http_code="${2:-0}"
  local out="${E2E_RUN_DIR}/raw/activation-code-create.json"
  local code_len=0
  if [[ -f "$body_file" ]]; then
    code_len="$(jq -r '(.activationCode // .code // "") | length' "$body_file" 2>/dev/null || echo 0)"
  fi
  jq -nc \
    --argjson http_code "${http_code:-0}" \
    --argjson activation_code_len "${code_len:-0}" \
    '{http_code: $http_code, activation_code_len: $activation_code_len, redacted: true}' >"$out"
}

e2e_write_redacted_activation_claim_request_summary() {
  local body_file="${1:-}"
  local out="${E2E_RUN_DIR}/raw/activation-claim.request.json"
  local code_len=0
  if [[ -f "$body_file" ]]; then
    code_len="$(jq -r '(.activationCode // .code // "") | length' "$body_file" 2>/dev/null || echo 0)"
  fi
  jq -nc \
    --argjson activation_code_len "${code_len:-0}" \
    '{activation_code_len: $activation_code_len, redacted: true}' >"$out"
}

e2e_admin_write_redacted_login_summary() {
  local code="${1:-0}"
  local body_file="${2:-}"
  local out="${E2E_RUN_DIR}/raw/admin-login.json"
  mkdir -p "${E2E_RUN_DIR}/raw"
  local token_len=0 roles_count=0 expires_in=0
  if [[ "$code" == "200" && -n "$body_file" && -f "$body_file" ]]; then
    token_len="$(jq -r '(.accessToken // .access_token // .tokens.accessToken // "") | length' "$body_file" 2>/dev/null || echo 0)"
    roles_count="$(jq -r '(.roles // .user.roles // []) | length' "$body_file" 2>/dev/null || echo 0)"
    expires_in="$(jq -r '.expiresIn // .expires_in // .tokens.expiresIn // 0' "$body_file" 2>/dev/null || echo 0)"
  fi
  jq -nc \
    --argjson http_code "${code:-0}" \
    --argjson token_len "${token_len:-0}" \
    --argjson roles_count "${roles_count:-0}" \
    --argjson expires_in "${expires_in:-0}" \
    '{
      ADMIN_AUTH_OK: ($http_code == 200),
      http_code: $http_code,
      token_len: $token_len,
      roles_count: $roles_count,
      expires_in: $expires_in,
      redacted: true
    }' >"$out"
}

e2e_admin_token() {
  local trimmed
  trimmed="$(e2e_admin_token_trim "${ADMIN_TOKEN:-}")"
  if [[ -n "$trimmed" ]]; then
    mkdir -p "${E2E_RUN_DIR}/raw"
    local tok_len="${#trimmed}"
    jq -nc \
      --argjson token_len "$tok_len" \
      '{
        ADMIN_AUTH_OK: true,
        http_code: 200,
        token_len: $token_len,
        roles_count: 0,
        expires_in: 0,
        redacted: true,
        source: "ADMIN_TOKEN_ENV"
      }' >"${E2E_RUN_DIR}/raw/admin-login.json"
    printf '%s' "$trimmed"
    return 0
  fi
  if [[ -z "${ADMIN_EMAIL:-}" || -z "${ADMIN_PASSWORD:-}" ]]; then
    E2E_ADMIN_AUTH_HTTP_CODE=""
    return 1
  fi
  mkdir -p "${E2E_RUN_DIR}/raw"
  local body_tmp
  body_tmp="$(mktemp)"
  local code
  code="$(curl -sS -o "$body_tmp" -w '%{http_code}' -X POST \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    --connect-timeout 8 --max-time 20 \
    -d "$(jq -nc --arg e "${ADMIN_EMAIL}" --arg p "${ADMIN_PASSWORD}" '{email:$e,password:$p}')" \
    "${BASE_URL%/}/v1/auth/login")"
  E2E_ADMIN_AUTH_HTTP_CODE="$code"
  local tok=""
  if [[ "$code" == "200" ]]; then
    tok="$(jq -r '.accessToken // .access_token // .tokens.accessToken // empty' "$body_tmp")"
  fi
  e2e_admin_write_redacted_login_summary "$code" "$body_tmp"
  rm -f "$body_tmp"
  [[ "$code" == "200" && -n "$tok" ]] || return 1
  printf '%s' "$tok"
}

e2e_grpc_proto_args() {
  GRPC_PROTO_ARGS=()
  local proto_root="${GRPC_PROTO_ROOT:-$(e2e_repo_root)/proto}"
  if [[ "${GRPC_USE_REFLECTION:-false}" == "true" ]]; then
    [[ "${GRPC_USE_PLAINTEXT:-false}" == "true" ]] && GRPC_PROTO_ARGS+=(-plaintext)
    return 0
  fi
  [[ -d "${proto_root}/avf/machine/v1" ]] || {
    echo "missing protos under ${proto_root}" >&2
    return 1
  }
  [[ "${GRPC_USE_PLAINTEXT:-false}" == "true" ]] && GRPC_PROTO_ARGS+=(-plaintext)
  GRPC_PROTO_ARGS+=(-import-path "$proto_root")
  local p
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    GRPC_PROTO_ARGS+=(-proto "${p#"${proto_root}"/}")
  done < <(find "$proto_root" -type f -name '*.proto' 2>/dev/null | LC_ALL=C sort)
}

e2e_grpc_call() {
  local method="$1"
  local body="$2"
  local evidence="$3"
  local auth="${4:-none}"
  local idem="${5:-}"
  local target="${GRPC_ADDR:-${GRPC_TARGET:-}}"
  [[ -n "$target" ]] || {
    echo "GRPC target unset (GRPC_ADDR / GRPC_TARGET)" >&2
    return 1
  }
  e2e_grpc_proto_args || return 1
  local req="${E2E_RUN_DIR}/raw/${evidence}.request.json"
  local resp="${E2E_RUN_DIR}/raw/${evidence}.response.json"
  local log="${E2E_RUN_DIR}/raw/${evidence}.grpc.log"
  printf '%s\n' "$body" >"$req"
  local -a args=("${GRPC_PROTO_ARGS[@]}")
  case "$auth" in
    machine)
      [[ -n "${MACHINE_ACCESS_TOKEN:-}" ]] || {
        echo "missing MACHINE_ACCESS_TOKEN" >&2
        return 1
      }
      args+=(-H "authorization: Bearer ${MACHINE_ACCESS_TOKEN}")
      [[ -n "${TEST_MACHINE_ID:-}" ]] && args+=(-H "x-machine-id: ${TEST_MACHINE_ID}")
      ;;
    none) ;;
    *) echo "unknown auth: ${auth}" >&2; return 1 ;;
  esac
  [[ -n "$idem" ]] && args+=(-H "idempotency-key: ${idem}")
  args+=(-d @ -max-time "${GRPC_MAX_TIME:-90}")
  local start end rc started_at ended_at grpc_code
  started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  start="$(e2e_py -c 'import time; print(int(time.time()*1000))')"
  set +e
  if command -v timeout >/dev/null 2>&1; then
    timeout --signal=TERM "$(( ${GRPC_MAX_TIME:-90} + 10 ))s" \
      grpcurl "${args[@]}" "$target" "$method" <"$req" >"$resp" 2>"$log"
    rc=$?
    if [[ "$rc" -eq 124 ]]; then
      echo "grpcurl timed out after ${GRPC_MAX_TIME:-90}s (timeout wrapper)" >>"$log"
    fi
  else
    grpcurl "${args[@]}" "$target" "$method" <"$req" >"$resp" 2>"$log"
    rc=$?
  fi
  set -e
  ended_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  end="$(e2e_py -c 'import time; print(int(time.time()*1000))')"
  local lat=$((end - start))
  E2E_GRPC_LAST_RC=$rc
  E2E_GRPC_LAST_LAT=$lat
  E2E_GRPC_LAST_METHOD="$method"
  E2E_GRPC_LAST_EVIDENCE="$evidence"
  grpc_code=""
  if [[ -f "$log" ]]; then
    grpc_code="$(grep -Eo 'Code: [A-Za-z]+' "$log" 2>/dev/null | tail -n1 | awk '{print $2}')"
    if [[ -z "$grpc_code" && "$rc" -eq 124 ]]; then
      grpc_code="DeadlineExceeded"
    fi
  fi
  local meta="${E2E_RUN_DIR}/raw/${evidence}.grpc-meta.json"
  jq -nc \
    --arg method "$method" \
    --arg evidence "$evidence" \
    --argjson rc "$rc" \
    --argjson latencyMs "$lat" \
    --arg maxTime "${GRPC_MAX_TIME:-90}" \
    --arg grpcCode "${grpc_code}" \
    --arg startedAt "$started_at" \
    --arg endedAt "$ended_at" \
    --arg respSize "$(wc -c <"$resp" 2>/dev/null | tr -d ' ')" \
    --arg logTail "$(tail -n 30 "$log" 2>/dev/null | sed 's/"/\\"/g' | tr '\n' ' ')" \
    '{method:$method, evidence:$evidence, rc:$rc, latencyMs:$latencyMs, maxTimeSec:$maxTime, grpc_code:$grpcCode, started_at:$startedAt, ended_at:$endedAt, responseBytes:($respSize|tonumber?), logTail:$logTail}' \
    >"$meta" 2>/dev/null || true
  if [[ "$method" == *"CreateOrder"* && "$rc" -ne 0 ]]; then
    e2e_write_create_order_failure_diag "$evidence" "$meta" || true
  fi
  return "$rc"
}

e2e_write_create_order_failure_diag() {
  local evidence="$1"
  local meta_path="$2"
  local out="${E2E_RUN_DIR}/CREATE_ORDER_FAILURE_DIAG.md"
  {
    echo "# CreateOrder failure diagnostic"
    echo ""
    echo "UTC: **$(date -u +%Y-%m-%dT%H:%M:%SZ)**"
    echo ""
    echo "Evidence: \`${evidence}\`"
    echo ""
    echo "See \`raw/${evidence}.grpc-meta.json\` and \`raw/${evidence}.grpc.log\`."
    echo ""
    echo "## Next server queries (read-only)"
    echo ""
    echo '```sql'
    echo "SELECT machine_id, operation, idempotency_key, status, last_seen_at"
    echo "FROM machine_idempotency_keys"
    echo "WHERE machine_id = '019e702c-11c6-7ab0-89c7-5eb32f0b12cb'"
    echo "  AND operation LIKE '%CreateOrder%'"
    echo "ORDER BY last_seen_at DESC LIMIT 20;"
    echo '```'
  } >"$out"
}

e2e_machine_is_canary() {
  local machine_id="$1"
  local admin_tok="$2"
  local out="${E2E_RUN_DIR}/raw/canary-machine-check.json"
  local code
  code="$(curl -sS -o "$out" -w '%{http_code}' \
    -H "Accept: application/json" \
    -H "Authorization: Bearer ${admin_tok}" \
    "${BASE_URL%/}/v1/admin/machines/${machine_id}")"
  [[ "$code" == "200" ]] || {
    echo "admin machine GET failed http=${code}"
    return 1
  }
  e2e_py - "$out" "$machine_id" <<'PY'
import json, os, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
mid = sys.argv[2].lower()
allow = os.environ.get("PRODUCTION_CANARY_MACHINE_ALLOWLIST", "")
if allow.strip():
    allowed = {x.strip().lower() for x in allow.split(",") if x.strip()}
    if mid in allowed:
        sys.exit(0)
name = (doc.get("name") or doc.get("Name") or "").lower()
code = (doc.get("code") or doc.get("Code") or "").lower()
serial = (doc.get("serialNumber") or doc.get("serial_number") or "").lower()
blob = " ".join([name, code, serial])
if "canary" in blob or "e2e-test" in blob or "e2e_test" in blob:
    sys.exit(0)
tags = doc.get("tags") or doc.get("Tags") or []
if isinstance(tags, list) and any("canary" in str(t).lower() for t in tags):
    sys.exit(0)
meta = doc.get("metadata") or doc.get("Metadata") or {}
if isinstance(meta, dict) and (meta.get("production_e2e_canary") is True or meta.get("e2e_canary") is True):
    sys.exit(0)
print("machine is not marked canary (name/code/serial must contain canary, or set PRODUCTION_CANARY_MACHINE_ALLOWLIST)", file=sys.stderr)
sys.exit(2)
PY
}

e2e_finalize_report() {
  local mode="$1"
  local smoke_verdict="$2"
  local exit_code="$3"
  local readiness_verdict="${4:-}"
  local strict_canary="${5:-}"
  local json="${E2E_RUN_DIR}/report.json"
  local md="${E2E_RUN_DIR}/REPORT.md"
  e2e_py - "$mode" "$smoke_verdict" "$exit_code" "$readiness_verdict" "$strict_canary" "$json" "$md" "${E2E_RUN_DIR}/probes.tsv" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path

mode, smoke_verdict, exit_code, readiness_verdict, strict_canary, json_path, md_path, tsv_path = sys.argv[1:9]
probes = []
p = Path(tsv_path)
if p.is_file():
    for line in p.read_text(encoding="utf-8").splitlines():
        parts = line.split("|", 3)
        if len(parts) >= 2:
            probes.append({
                "name": parts[0],
                "outcome": parts[1],
                "latency_ms": int(parts[2]) if len(parts) > 2 and parts[2].isdigit() else 0,
                "detail": parts[3] if len(parts) > 3 else "",
            })
payload = {
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "mode": mode,
    "smoke_verdict": smoke_verdict,
    "readiness_verdict": readiness_verdict or None,
    "strict_canary": strict_canary == "true",
    "verdict": smoke_verdict,
    "exit_code": int(exit_code),
    "probes": probes,
    "run_dir": str(Path(json_path).parent),
}
Path(json_path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
lines = [
    f"# Production E2E — {mode}",
    "",
    f"- **SMOKE_VERDICT:** {smoke_verdict}",
]
if readiness_verdict:
    lines.append(f"- **READINESS_VERDICT:** {readiness_verdict}")
if strict_canary:
    lines.append(f"- **Strict canary:** {strict_canary}")
lines.extend([
    f"- **Exit code:** {exit_code}",
    f"- **Generated:** {payload['generated_at']}",
    "",
    "| Probe | Outcome | Latency ms | Detail |",
    "|---|---|---:|---|",
])
for pr in probes:
    lines.append(f"| {pr['name']} | {pr['outcome']} | {pr['latency_ms']} | {pr['detail']} |")
Path(md_path).write_text("\n".join(lines) + "\n", encoding="utf-8")
print(smoke_verdict)
PY
  echo "Report: ${md}"
  echo "JSON:   ${json}"
  return "$exit_code"
}
