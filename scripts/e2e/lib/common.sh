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
  if command -v python3 >/dev/null 2>&1; then
    echo python3
    return 0
  fi
  if command -v py >/dev/null 2>&1; then
    echo "py -3"
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

e2e_admin_token() {
  if [[ -n "${ADMIN_TOKEN:-}" ]]; then
    printf '%s' "${ADMIN_TOKEN}"
    return 0
  fi
  if [[ -z "${ADMIN_EMAIL:-}" || -z "${ADMIN_PASSWORD:-}" ]]; then
    return 1
  fi
  local out="${E2E_RUN_DIR}/raw/admin-login.json"
  local code
  code="$(curl -sS -o "$out" -w '%{http_code}' -X POST \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    --connect-timeout 8 --max-time 20 \
    -d "$(jq -nc --arg e "${ADMIN_EMAIL}" --arg p "${ADMIN_PASSWORD}" '{email:$e,password:$p}')" \
    "${BASE_URL%/}/v1/auth/login")"
  [[ "$code" == "200" ]] || return 1
  jq -r '.accessToken // .access_token // .tokens.accessToken // empty' "$out"
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
  local start end rc
  start="$(e2e_py -c 'import time; print(int(time.time()*1000))')"
  set +e
  grpcurl "${args[@]}" "$target" "$method" <"$req" >"$resp" 2>"$log"
  rc=$?
  set -e
  end="$(e2e_py -c 'import time; print(int(time.time()*1000))')"
  local lat=$((end - start))
  E2E_GRPC_LAST_RC=$rc
  E2E_GRPC_LAST_LAT=$lat
  return "$rc"
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
