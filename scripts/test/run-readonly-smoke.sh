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
APP_ENV_VAL="${APP_ENV:-}"
EXPECT_PUBLIC_METRICS="${SMOKE_EXPECT_PUBLIC_METRICS:-}"
EXPECT_OPENAPI_JSON="${SMOKE_EXPECT_OPENAPI_JSON:-}"
OPS_BASE_URL="${OPS_BASE_URL:-}"
SCRAPE_TOKEN="${METRICS_SCRAPE_TOKEN:-}"

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

append_probe_row() {
  local path="$1"
  local url="$2"
  local status="$3"
  local latency_ms="$4"
  local snippet="$5"
  local probe_class="$6"
  local expectation_note="$7"
  local outcome="$8"
  jq -nc \
    --arg path "${path}" \
    --arg url "${url}" \
    --arg status "${status}" \
    --argjson latency_ms "${latency_ms}" \
    --arg snippet "${snippet}" \
    --arg probe_class "${probe_class}" \
    --arg expectation_note "${expectation_note}" \
    --arg outcome "${outcome}" \
    '{path:$path,url:$url,method:"GET",status:($status|tonumber? // 0),latency_ms:$latency_ms,body_snippet:$snippet,probe_class:$probe_class,expectation_note:$expectation_note,outcome:$outcome}' \
    >>"${tmp}"
}

http_get_probe() {
  local url="$1"
  local accept="$2"
  local auth_header="${3:-}"
  local body_file="${4}"
  local start_ms end_ms status ec
  start_ms="$(python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
)"
  set +e
  if [[ -n "${auth_header}" ]]; then
    status="$(curl -fsS -o "${body_file}" -w '%{http_code}' -X GET \
      -H "Accept: ${accept}" \
      -H "${auth_header}" \
      --connect-timeout 5 \
      --max-time 15 \
      "${url}" 2>/dev/null)"
  else
    status="$(curl -fsS -o "${body_file}" -w '%{http_code}' -X GET \
      -H "Accept: ${accept}" \
      --connect-timeout 5 \
      --max-time 15 \
      "${url}" 2>/dev/null)"
  fi
  ec=$?
  set -e
  end_ms="$(python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
)"
  local latency_ms=$((end_ms - start_ms))
  if [[ "${ec}" -ne 0 ]]; then
    status="000"
    : >"${body_file}"
  fi
  echo "${status}" "${latency_ms}"
}

snippet_from_body() {
  python3 - "$1" <<'PY'
import re
import sys
from pathlib import Path

text = Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace")[:500]
text = re.sub(r'(?i)("?(password|token|secret|authorization)"?\s*[:=]\s*)"?[^",\s}]+"?', r'\1"***REDACTED***"', text)
text = text.replace("\n", " ").replace("\r", " ")
print(text, end="")
PY
}

for path in "${paths[@]}"; do
  if [[ "${path}" != /* ]]; then
    echo "run-readonly-smoke: path must start with /: ${path}" >&2
    overall=2
    continue
  fi
  url="${BASE_URL%/}${path}"
  bfile="$(mktemp)"
  read -r status latency_ms < <(http_get_probe "${url}" 'application/json' "" "${bfile}")
  snippet="$(snippet_from_body "${bfile}")"
  rm -f "${bfile}"
  ec_match=0
  if [[ "${status}" == "000" ]]; then
    overall=1
    ec_match=1
  elif [[ "${path}" == "/health/live" || "${path}" == "/health/ready" || "${path}" == "/version" ]]; then
    if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
      overall=1
    fi
  fi
  outcome="pass"
  if [[ "${ec_match}" -eq 1 || "${status}" -lt 200 || "${status}" -ge 300 ]]; then
    outcome="fail"
  fi
  note="required core probe"
  jq -nc \
    --arg path "${path}" \
    --arg url "${url}" \
    --arg status "${status}" \
    --argjson latency_ms "${latency_ms}" \
    --arg snippet "${snippet}" \
    --arg probe_class "core" \
    --arg expectation_note "${note}" \
    --arg outcome "${outcome}" \
    '{path:$path,url:$url,method:"GET",status:($status|tonumber? // 0),latency_ms:$latency_ms,body_snippet:$snippet,probe_class:$probe_class,expectation_note:$expectation_note,outcome:$outcome}' \
    >>"${tmp}"
done

if [[ "${overall}" -eq 2 ]]; then
  overall=1
fi

# --- Public GET /metrics (never print scrape tokens) ---
pm_url="${BASE_URL%/}/metrics"
pm_body="$(mktemp)"
read -r pm_status pm_lat < <(http_get_probe "${pm_url}" 'text/plain' "" "${pm_body}")
pm_snip="$(snippet_from_body "${pm_body}")"
rm -f "${pm_body}"
pm_outcome="pass"
pm_note=""
if [[ "${EXPECT_PUBLIC_METRICS}" == "true" ]]; then
  pm_note="SMOKE_EXPECT_PUBLIC_METRICS=true: public /metrics must return 200 (not recommended for production)"
  if [[ "${pm_status}" != "200" ]]; then
    pm_outcome="fail"
    overall=1
  fi
else
  if [[ "${APP_ENV_VAL}" == "production" || "${EXPECT_PUBLIC_METRICS}" == "false" ]]; then
    pm_note="production or explicit off: public /metrics may 404 (expected-safe)"
    if [[ "${pm_status}" == "404" ]]; then
      pm_outcome="pass"
    elif [[ "${pm_status}" == "200" ]]; then
      pm_note="${pm_note}; observed 200"
    elif [[ "${pm_status}" == "000" ]]; then
      pm_outcome="fail"
      overall=1
    else
      pm_outcome="fail"
      overall=1
    fi
  else
    pm_note="non-production: public /metrics may be 200 or 404"
    if [[ "${pm_status}" == "000" ]]; then
      pm_outcome="fail"
      overall=1
    elif [[ "${pm_status}" != "200" && "${pm_status}" != "404" ]]; then
      pm_outcome="fail"
      overall=1
    fi
  fi
fi
append_probe_row "/metrics" "${pm_url}" "${pm_status}" "${pm_lat}" "${pm_snip}" "public_metrics" "${pm_note}" "${pm_outcome}"

# --- Public GET /swagger/doc.json ---
doc_url="${BASE_URL%/}/swagger/doc.json"
doc_body_file="$(mktemp)"
read -r doc_status doc_lat < <(http_get_probe "${doc_url}" 'application/json' "" "${doc_body_file}")
doc_snip="$(snippet_from_body "${doc_body_file}")"
doc_outcome="pass"
doc_note=""
if [[ "${EXPECT_OPENAPI_JSON}" == "true" ]]; then
  doc_note="SMOKE_EXPECT_OPENAPI_JSON=true: require 200 JSON with openapi + paths"
  if [[ "${doc_status}" != "200" ]]; then
    doc_outcome="fail"
    overall=1
  else
    if ! python3 - "${doc_body_file}" <<'PY'
import json
import sys
from pathlib import Path
p = Path(sys.argv[1])
raw = p.read_text(encoding="utf-8", errors="replace")
d = json.loads(raw)
if "paths" not in d:
    sys.exit(2)
if "openapi" not in d and "swagger" not in d:
    sys.exit(2)
PY
    then
      doc_outcome="fail"
      overall=1
    fi
  fi
else
  doc_note="OpenAPI JSON not required; 404 is acceptable"
  if [[ "${doc_status}" == "404" ]]; then
    doc_outcome="pass"
  elif [[ "${doc_status}" == "200" ]]; then
    doc_note="${doc_note}; observed 200"
  elif [[ "${doc_status}" == "000" ]]; then
    doc_outcome="fail"
    overall=1
  else
    doc_outcome="fail"
    overall=1
  fi
fi
rm -f "${doc_body_file}"
append_probe_row "/swagger/doc.json" "${doc_url}" "${doc_status}" "${doc_lat}" "${doc_snip}" "openapi_json" "${doc_note}" "${doc_outcome}"

# --- Ops listener (optional) ---
if [[ -n "${OPS_BASE_URL}" ]]; then
  ob="${OPS_BASE_URL%/}"
  for op_path in /health/live /health/ready; do
    o_url="${ob}${op_path}"
    obody="$(mktemp)"
    read -r o_status o_lat < <(http_get_probe "${o_url}" 'text/plain' "" "${obody}")
    o_snip="$(snippet_from_body "${obody}")"
    rm -f "${obody}"
    o_out="pass"
    o_note="ops listener core"
    if [[ "${o_status}" -lt 200 || "${o_status}" -ge 300 || "${o_status}" == "000" ]]; then
      o_out="fail"
      overall=1
    fi
    append_probe_row "${op_path}" "${o_url}" "${o_status}" "${o_lat}" "${o_snip}" "ops" "${o_note}" "${o_out}"
  done
  m_url="${ob}/metrics"
  auth_h=""
  if [[ -n "${SCRAPE_TOKEN}" ]]; then
    auth_h="Authorization: Bearer ${SCRAPE_TOKEN}"
  fi
  om_body="$(mktemp)"
  read -r om_status om_lat < <(http_get_probe "${m_url}" 'text/plain' "${auth_h}" "${om_body}")
  om_snip="$(snippet_from_body "${om_body}")"
  rm -f "${om_body}"
  om_out="pass"
  om_note="ops /metrics"
  if [[ -n "${SCRAPE_TOKEN}" ]]; then
    om_note="ops /metrics with Authorization: Bearer ***REDACTED***"
  fi
  if [[ "${om_status}" != "200" || "${om_status}" == "000" ]]; then
    om_out="fail"
    overall=1
  fi
  append_probe_row "/metrics" "${m_url}" "${om_status}" "${om_lat}" "${om_snip}" "ops_metrics" "${om_note}" "${om_out}"
fi

jq -s \
  --arg base_url "${BASE_URL}" \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson exit_code "${overall}" \
  --arg app_env "${APP_ENV_VAL}" \
  --arg smoke_expect_public_metrics "${EXPECT_PUBLIC_METRICS}" \
  --arg smoke_expect_openapi_json "${EXPECT_OPENAPI_JSON}" \
  --arg ops_base_url "${OPS_BASE_URL}" \
  --argjson metrics_scrape_token_configured "$(if [[ -n "${SCRAPE_TOKEN}" ]]; then echo true; else echo false; fi)" \
  '{
    generated_at:$generated_at,
    base_url:$base_url,
    mode:"readonly",
    exit_code:$exit_code,
    expectations:{
      app_env:(if ($app_env|length) > 0 then $app_env else null end),
      smoke_expect_public_metrics:(if ($smoke_expect_public_metrics|length) > 0 then $smoke_expect_public_metrics else null end),
      smoke_expect_openapi_json:(if ($smoke_expect_openapi_json|length) > 0 then $smoke_expect_openapi_json else null end),
      ops_base_url:(if ($ops_base_url|length) > 0 then $ops_base_url else null end),
      metrics_scrape_token_configured:$metrics_scrape_token_configured
    },
    probes:.
  }' \
  "${tmp}" >"${OUT_JSON}"

{
  echo "# Read-only Smoke Report"
  echo
  echo "- Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "- Base URL: \`${BASE_URL}\`"
  echo "- Exit code: \`${overall}\`"
  if [[ -n "${APP_ENV_VAL}" ]]; then
    echo "- APP_ENV: \`${APP_ENV_VAL}\`"
  fi
  if [[ -n "${EXPECT_PUBLIC_METRICS}" ]]; then
    echo "- SMOKE_EXPECT_PUBLIC_METRICS: \`${EXPECT_PUBLIC_METRICS}\`"
  fi
  if [[ -n "${EXPECT_OPENAPI_JSON}" ]]; then
    echo "- SMOKE_EXPECT_OPENAPI_JSON: \`${EXPECT_OPENAPI_JSON}\`"
  fi
  if [[ -n "${OPS_BASE_URL}" ]]; then
    echo "- OPS_BASE_URL: \`${OPS_BASE_URL}\`"
  fi
  echo
  echo "| Class | Path | Method | Status | Latency ms | Outcome | Note | Body snippet |"
  echo "|---|---|:---:|---:|---:|---|---:|---|"
  jq -r '.probes[] | "| \(.probe_class) | `\(.path)` | GET | \(.status) | \(.latency_ms) | \(.outcome) | \(.expectation_note) | `\(.body_snippet | gsub("`"; "\\`") | gsub("\n"; " ") )` |"' "${OUT_JSON}"
} >"${OUT_MD}"

exit "${overall}"
