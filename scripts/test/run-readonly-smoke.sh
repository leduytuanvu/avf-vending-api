#!/usr/bin/env bash
# shellcheck shell=bash
# Read-only smoke probe for staging/production. Never calls mutating routes.

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT_DIR="${ROOT}/reports/test"
mkdir -p "${REPORT_DIR}"

BASE_URL="${BASE_URL:-}"
ALLOW_LOCAL_SMOKE="${ALLOW_LOCAL_SMOKE:-false}"
OUT_JSON="${REPORT_DIR}/readonly-smoke.json"
OUT_MD="${REPORT_DIR}/readonly-smoke.md"

if [[ -z "${BASE_URL}" ]]; then
  echo "run-readonly-smoke: BASE_URL is required" >&2
  exit 2
fi

case "${BASE_URL}" in
  http://127.0.0.1:*|http://localhost:*|https://127.0.0.1:*|https://localhost:*)
    if [[ "${ALLOW_LOCAL_SMOKE}" != "true" ]]; then
      echo "run-readonly-smoke: refusing localhost BASE_URL unless ALLOW_LOCAL_SMOKE=true" >&2
      exit 2
    fi
    ;;
esac

paths=(/health/live /health/ready /version)
if [[ -n "${SMOKE_READONLY_PATHS:-}" ]]; then
  # shellcheck disable=SC2206
  extra_paths=(${SMOKE_READONLY_PATHS})
  paths+=("${extra_paths[@]}")
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
: >"${tmp}"

overall=0
for path in "${paths[@]}"; do
  if [[ "${path}" != /* ]]; then
    echo "run-readonly-smoke: path must start with /: ${path}" >&2
    overall=2
    continue
  fi
  url="${BASE_URL%/}${path}"
  body_file="$(mktemp)"
  start_ms="$(python - <<'PY'
import time
print(int(time.time() * 1000))
PY
)"
  set +e
  status="$(curl -fsS -o "${body_file}" -w '%{http_code}' -X GET \
    -H 'Accept: application/json' \
    --connect-timeout 5 \
    --max-time 15 \
    "${url}" 2>/dev/null)"
  ec=$?
  set -e
  end_ms="$(python - <<'PY'
import time
print(int(time.time() * 1000))
PY
)"
  latency_ms=$((end_ms - start_ms))
  snippet="$(python - "${body_file}" <<'PY'
import json
import re
import sys
from pathlib import Path

text = Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace")[:500]
text = re.sub(r'(?i)("?(password|token|secret|authorization)"?\s*[:=]\s*)"?[^",\s}]+"?', r'\1"***REDACTED***"', text)
print(text)
PY
)"
  rm -f "${body_file}"
  if [[ "${ec}" -ne 0 ]]; then
    status="000"
    overall=1
  elif [[ "${path}" == "/health/live" || "${path}" == "/health/ready" ]]; then
    if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
      overall=1
    fi
  fi
  jq -nc \
    --arg path "${path}" \
    --arg status "${status}" \
    --argjson latency_ms "${latency_ms}" \
    --arg snippet "${snippet}" \
    '{path:$path,method:"GET",status:($status|tonumber? // 0),latency_ms:$latency_ms,body_snippet:$snippet}' \
    >>"${tmp}"
done

jq -s \
  --arg base_url "${BASE_URL}" \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson exit_code "${overall}" \
  '{generated_at:$generated_at,base_url:$base_url,mode:"readonly",exit_code:$exit_code,probes:.}' \
  "${tmp}" >"${OUT_JSON}"

{
  echo "# Read-only Smoke Report"
  echo
  echo "- Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "- Base URL: \`${BASE_URL}\`"
  echo "- Exit code: \`${overall}\`"
  echo
  echo "| Method | Path | Status | Latency ms | Body snippet |"
  echo "|---|---|---:|---:|---|"
  jq -r '.probes[] | "| GET | `\(.path)` | \(.status) | \(.latency_ms) | `\(.body_snippet | gsub("`"; "\\`") | gsub("\n"; " ") )` |"' "${OUT_JSON}"
} >"${OUT_MD}"

exit "${overall}"
