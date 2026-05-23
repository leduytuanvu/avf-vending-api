#!/usr/bin/env bash
# CI: regenerate Postman from e2e-manifest.yaml, enforce git cleanliness + shell parity.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

PY=""
if command -v python3 >/dev/null 2>&1 && python3 -c "import sys" 2>/dev/null; then
  PY=python3
elif command -v py >/dev/null 2>&1 && py -3 -c "import sys" 2>/dev/null; then
  PY="py -3"
else
  echo "ERROR: python3 or py -3 required" >&2
  exit 1
fi

echo "== production Postman parity =="
${PY} postman/production/generate_postman_from_manifest.py

echo "== git diff postman/production/ (must be clean after regen) =="
if ! git diff --ignore-cr-at-eol --exit-code -- postman/production/; then
  echo "POSTMAN_GIT_DIFF_FAIL: commit regenerated postman/production/* or fix generator" >&2
  exit 1
fi

echo "== shell ↔ Postman structural parity =="
${PY} tests/e2e/production/scripts/validate_postman_shell_parity.py

if [[ "${RUN_PRODUCTION_NEWMAN:-}" == "1" ]]; then
  if ! command -v newman >/dev/null 2>&1; then
    echo "NEWMAN_SKIP: newman not installed" >&2
    exit 1
  fi
  [[ -n "${E2E_PROD_ADMIN_EMAIL:-}" && -n "${E2E_PROD_ADMIN_PASSWORD:-}" ]] || {
    echo "NEWMAN_SKIP: E2E_PROD_ADMIN_* not set" >&2
    exit 0
  }
  RUN_ID="ci-$(date -u +%Y%m%dT%H%M%SZ)"
  RUN_DIR="${ROOT}/.e2e-runs/production/${RUN_ID}"
  mkdir -p "${RUN_DIR}/postman"
  export PROD_E2E_RUN_ID="${RUN_ID}"
  export PROD_E2E_PREFIX="E2E-PROD-${RUN_ID}"
  export PROD_E2E_STATE_JSON="${RUN_DIR}/state.json"
  export BASE_URL="${E2E_PROD_BASE_URL:-https://api.ldtv.dev}"
  export ADMIN_EMAIL="${E2E_PROD_ADMIN_EMAIL}"
  export ADMIN_PASSWORD="${E2E_PROD_ADMIN_PASSWORD}"
  export E2E_PRODUCTION_WRITE_CONFIRMATION="${E2E_PRODUCTION_WRITE_CONFIRMATION:-I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION}"
  # shellcheck source=/dev/null
  source "${ROOT}/tests/e2e/production/lib/postman_env.sh" 2>/dev/null || true
  if declare -F prod_e2e_sync_postman_env >/dev/null 2>&1; then
    export PROD_E2E_REPO_ROOT="${ROOT}"
    export PROD_E2E_RUN_DIR="${RUN_DIR}"
    prod_e2e_sync_postman_env
  else
    cp postman/production/avf-production-e2e.postman_environment.json \
      "${RUN_DIR}/postman/runtime.postman_environment.json"
  fi
  newman run postman/production/avf-production-e2e.postman_collection.json \
    -e "${RUN_DIR}/postman/runtime.postman_environment.json" \
    --reporters cli,json \
    --reporter-json-export "${RUN_DIR}/postman/newman-report.json" \
    --bail
  echo "NEWMAN_PRODUCTION_OK run_dir=${RUN_DIR}"
else
  echo "NEWMAN_SKIP: set RUN_PRODUCTION_NEWMAN=1 and E2E_PROD_* to run live Newman"
fi

echo "POSTMAN_PARITY_CI_OK"
