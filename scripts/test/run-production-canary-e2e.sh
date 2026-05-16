#!/usr/bin/env bash
# shellcheck shell=bash
# Guarded production canary E2E entrypoint. Destructive writes require explicit env confirmation.

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT_DIR="${ROOT}/reports/test"
mkdir -p "${REPORT_DIR}"

OUT_JSON="${REPORT_DIR}/production-canary-e2e.json"
OUT_MD="${REPORT_DIR}/production-canary-e2e.md"

required=(
  ALLOW_PROD_WRITES
  PROD_WRITE_CONFIRMATION
  CANARY_MACHINE_ID
  CANARY_MACHINE_TOKEN
  CANARY_SITE_ID
  CANARY_PRODUCT_ID
  CANARY_SLOT_INDEX
)

missing=()
for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    missing+=("${name}")
  fi
done
if [[ "${ALLOW_PROD_WRITES:-}" != "true" ]]; then
  missing+=("ALLOW_PROD_WRITES=true")
fi
if [[ "${PROD_WRITE_CONFIRMATION:-}" != "RUN_DESTRUCTIVE_PRODUCTION_TESTS" ]]; then
  missing+=("PROD_WRITE_CONFIRMATION=RUN_DESTRUCTIVE_PRODUCTION_TESTS")
fi

if [[ "${#missing[@]}" -gt 0 ]]; then
  python3 - "${OUT_JSON}" "${OUT_MD}" "${missing[*]}" <<'PY'
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

missing = sys.argv[3].split()
payload = {
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "production_canary_e2e": "BLOCKED",
    "reason": "required canary write confirmation/env is missing",
    "missing": missing,
    "required_next_action": "Configure canary-only IDs/tokens and explicit production write confirmation, then rerun.",
}
Path(sys.argv[1]).write_text(json.dumps(payload, indent=2), encoding="utf-8")
Path(sys.argv[2]).write_text(
    "# Production Canary E2E\n\n"
    "- Status: **BLOCKED**\n"
    "- Reason: required canary write confirmation/env is missing\n"
    "- Missing: `" + ", ".join(missing) + "`\n"
    "- Next action: configure canary-only IDs/tokens and explicit production write confirmation, then rerun.\n",
    encoding="utf-8",
)
PY
  echo "Production canary E2E BLOCKED: ${missing[*]}"
  exit 0
fi

if [[ -z "${BASE_URL:-}" ]]; then
  echo "run-production-canary-e2e: BASE_URL is required for production canary execution" >&2
  exit 2
fi

export E2E_TARGET=production
export MACHINE_TOKEN="${CANARY_MACHINE_TOKEN}"
export MACHINE_ID="${CANARY_MACHINE_ID}"

set +e
bash "${ROOT}/tests/e2e/run-all-local.sh" --reuse-data "${E2E_DATA_FILE:-/dev/null}"
ec=$?
set -e

python3 - "${OUT_JSON}" "${OUT_MD}" "${ec}" "${E2E_RUN_DIR:-}" <<'PY'
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

ec = int(sys.argv[3])
run_dir = sys.argv[4]
status = "PASS" if ec == 0 else "FAIL"
payload = {
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "production_canary_e2e": status,
    "exit_code": ec,
    "evidence_path": run_dir,
}
Path(sys.argv[1]).write_text(json.dumps(payload, indent=2), encoding="utf-8")
Path(sys.argv[2]).write_text(
    "# Production Canary E2E\n\n"
    f"- Status: **{status}**\n"
    f"- Exit code: `{ec}`\n"
    f"- Evidence: `{run_dir}`\n",
    encoding="utf-8",
)
PY

exit "${ec}"
