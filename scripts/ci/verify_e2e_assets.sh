#!/usr/bin/env bash
# Offline quality checks for tests/e2e (no API, no Docker).
# Optional: shellcheck, lychee/markdown-link-check for markdown (skipped if absent).
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

errs=0
warn() { echo "verify_e2e_assets: WARN: $*" >&2; }
err() { echo "verify_e2e_assets: ERROR: $*" >&2; errs=$((errs + 1)); }

python_exec=""
for c in python3 python; do
  if command -v "${c}" >/dev/null 2>&1 && "${c}" -c "import sys" 2>/dev/null; then
    python_exec="${c}"
    break
  fi
done
if [[ -z "${python_exec}" ]] && command -v py >/dev/null 2>&1 && py -3 -c "import sys" 2>/dev/null; then
  python_exec="py"
fi
if [[ -z "${python_exec}" ]]; then
  err "python3 (or py -3) is required for E2E tool syntax + JSON checks"
fi

py_compile_file() {
  local f="$1"
  if [[ "${python_exec}" == "py" ]]; then
    py -3 -m py_compile "${f}"
  else
    "${python_exec}" -m py_compile "${f}"
  fi
}

py_json_load() {
  local f="$1"
  if [[ "${python_exec}" == "py" ]]; then
    py -3 -c "import json; json.load(open(r'''${f}''', encoding='utf-8-sig'))"
  else
    "${python_exec}" -c "import json; json.load(open(r'''${f}''', encoding='utf-8-sig'))"
  fi
}

echo "verify_e2e_assets: repository root=${ROOT}"

# Required flow-review tooling (must exist; bash/python checks also run globally below)
for _req in tests/e2e/lib/e2e_flow_review.sh tests/e2e/tools/generate-improvement-summary.py tests/e2e/tools/generate-optimization-backlog.py; do
  [[ -f "${ROOT}/${_req}" ]] || err "missing required E2E asset: ${_req}"
done

# --- 1) bash -n ---
while IFS= read -r -d '' f; do
  if ! bash -n "${f}"; then
    err "bash -n failed: ${f}"
  fi
done < <(find tests/e2e -name '*.sh' -print0 | LC_ALL=C sort -z)

# Explicit syntax on flow-review library (also in find loop)
if ! bash -n tests/e2e/lib/e2e_flow_review.sh; then
  err "bash -n failed: tests/e2e/lib/e2e_flow_review.sh"
fi

# --- 2) shellcheck (optional) ---
if command -v shellcheck >/dev/null 2>&1; then
  sc_err=0
  while IFS= read -r -d '' f; do
    if ! shellcheck -x -e SC1091 "${f}"; then
      sc_err=1
    fi
  done < <(find tests/e2e -name '*.sh' -print0 | LC_ALL=C sort -z)
  if [[ "${sc_err}" -ne 0 ]]; then
    err "one or more shellcheck failures under tests/e2e/**/*.sh"
  fi
else
  warn "shellcheck not on PATH; skipping (install for stricter CI)"
fi

# --- 3) Python syntax (tests/e2e/tools/*.py) ---
if [[ -n "${python_exec}" ]]; then
  shopt -s nullglob
  for f in tests/e2e/tools/*.py; do
    if ! py_compile_file "${f}"; then
      err "py_compile failed: ${f}"
    fi
  done
  shopt -u nullglob
fi

# --- 4) JSON under tests/e2e/data/*.json ---
if command -v jq >/dev/null 2>&1; then
  shopt -s nullglob
  for f in tests/e2e/data/*.json; do
    if ! jq empty "${f}"; then
      err "invalid JSON: ${f}"
    fi
  done
  shopt -u nullglob
elif [[ -n "${python_exec}" ]]; then
  shopt -s nullglob
  for f in tests/e2e/data/*.json; do
    if ! py_json_load "${f}"; then
      err "invalid JSON: ${f}"
    fi
  done
  shopt -u nullglob
else
  warn "skipped JSON validation (no jq and no working python)"
fi

# Improvement finding example: required top-level keys
if [[ -n "${python_exec}" ]] && [[ -f tests/e2e/data/improvement-finding.example.json ]]; then
  _imp_ex="tests/e2e/data/improvement-finding.example.json"
  if [[ "${python_exec}" == "py" ]]; then
    py -3 -c "
import json
keys = {'ts','finding_id','severity','category','flow_id','scenario_id','step_name','protocol',
       'endpoint_or_rpc_or_topic','symptom','impact','recommendation','evidence_file','status'}
d = json.load(open(r'''${_imp_ex}''', encoding='utf-8'))
missing = keys - set(d.keys())
if missing:
    raise SystemExit('improvement-finding.example.json missing keys: ' + ', '.join(sorted(missing)))
if str(d.get('severity')) not in ('P0','P1','P2','P3'):
    raise SystemExit('improvement-finding.example.json severity must be P0–P3')
" || err "improvement-finding.example.json schema check failed"
  else
    "${python_exec}" -c "
import json
keys = {'ts','finding_id','severity','category','flow_id','scenario_id','step_name','protocol',
       'endpoint_or_rpc_or_topic','symptom','impact','recommendation','evidence_file','status'}
d = json.load(open(r'''${_imp_ex}''', encoding='utf-8'))
missing = keys - set(d.keys())
if missing:
    raise SystemExit('improvement-finding.example.json missing keys: ' + ', '.join(sorted(missing)))
if str(d.get('severity')) not in ('P0','P1','P2','P3'):
    raise SystemExit('improvement-finding.example.json severity must be P0–P3')
" || err "improvement-finding.example.json schema check failed"
  fi
fi

# --- 5) Markdown link sanity (optional) ---
MD_FILES=(tests/e2e/README.md docs/testing/e2e-local-test-guide.md)
md_checked=0
if command -v lychee >/dev/null 2>&1; then
  for m in "${MD_FILES[@]}"; do
    [[ -f "${m}" ]] || continue
    if ! lychee --offline --no-progress "${m}"; then
      err "lychee --offline reported issues: ${m}"
    fi
    md_checked=1
  done
elif command -v markdown-link-check >/dev/null 2>&1; then
  for m in "${MD_FILES[@]}"; do
    [[ -f "${m}" ]] || continue
    if ! markdown-link-check -q "${m}"; then
      err "markdown-link-check reported issues: ${m}"
    fi
    md_checked=1
  done
else
  warn "lychee and markdown-link-check absent; skipping markdown link checks"
fi
if [[ "${md_checked}" -eq 1 ]]; then
  echo "verify_e2e_assets: markdown link check completed for checked files"
fi

# --- 6) .gitignore: .e2e-runs ---
if ! grep -qE '^\.e2e-runs(/|\s|$)' .gitignore && ! grep -qE '(^|/)\.e2e-runs/' .gitignore && ! grep -qE '\.e2e-runs/' .gitignore; then
  err ".gitignore must ignore local E2E artifact dir (.e2e-runs)"
fi

# --- 7) Secret-like patterns in tracked tests/e2e + scripts/ci ---
# Exclude this checker script from PEM/Bearer hits in documentation strings.
filter_secret_hits() {
  grep -Ev '(^scripts/ci/verify_e2e_assets\.sh:|[/]README\.md:|\.md:|\.example\.json:|\.schema\.json:)' || true
}

if git grep -n 'BEGIN.*PRIVATE KEY' -- tests/e2e scripts/ci 2>/dev/null | filter_secret_hits | grep -q .; then
  err "possible private key material committed under tests/e2e or scripts/ci"
fi

if git grep -nE 'Authorization:[[:space:]]*Bearer[[:space:]]+[A-Za-z0-9._=-]{24,}' -- tests/e2e scripts/ci 2>/dev/null | filter_secret_hits | grep -q .; then
  git grep -nE 'Authorization:[[:space:]]*Bearer[[:space:]]+[A-Za-z0-9._=-]{24,}' -- tests/e2e scripts/ci 2>/dev/null | filter_secret_hits >&2
  err "hardcoded Authorization: Bearer token pattern in tracked E2E/CI files"
fi

if git grep -nE 'eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}' -- tests/e2e scripts/ci 2>/dev/null \
  | grep -Ev '(example|placeholder|YOUR_|REPLACE|\*\*\*|\.md:|\.example\.json)' | filter_secret_hits | grep -q .; then
  git grep -nE 'eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}' -- tests/e2e scripts/ci 2>/dev/null \
    | grep -Ev '(example|placeholder|YOUR_|REPLACE|\*\*\*|\.md:|\.example\.json)' | filter_secret_hits >&2
  err "possible committed JWT-shaped token under tests/e2e or scripts/ci"
fi

# --- 8) Scenario contracts ---
for f in tests/e2e/scenarios/*.sh; do
  [[ -f "${f}" ]] || continue
  base="$(basename "${f}")"

  has_id=0
  [[ "${base}" =~ ^00_ ]] && has_id=1
  if grep -qE '^[[:space:]]*(export[[:space:]]+)?(FLOW_ID|SID)=' "${f}"; then
    has_id=1
  fi
  if [[ "${has_id}" -eq 0 ]]; then
    err "scenario missing scenario id (FLOW_ID=, SID=, or 00_*.sh naming): ${f}"
  fi

  if ! grep -qE '(start_step[[:space:](]|append_event_jsonl[[:space:](]|e2e_append_test_event[[:space:](]|grpc_contract_try_unauth[[:space:]]|grpc_contract_skip[[:space:]]|grpc_contract_try[[:space:]])' "${f}"; then
    err "scenario must use start_step, append_event_jsonl, e2e_append_test_event, or grpc_contract_*: ${f}"
  fi

  if ! grep -qE '(end_step[[:space:]]+(passed|failed|skipped)|append_event_jsonl|e2e_append_test_event|grpc_contract_try_unauth|grpc_contract_skip|grpc_contract_try[[:space:]])' "${f}"; then
    err "scenario must record pass/fail/skip (end_step, append_event_jsonl, e2e_append_test_event, or grpc_contract_*): ${f}"
  fi
done

if [[ "${errs}" -gt 0 ]]; then
  echo "verify_e2e_assets: FAILED (${errs} error(s))" >&2
  exit 1
fi

echo "verify_e2e_assets: OK"
exit 0
