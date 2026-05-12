#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# Non-interactive Git Bash on Windows sometimes surfaces the WindowsApps "python" stub first; prefer real installs.
if [[ -x "/c/Python314/python" ]]; then
  PY_0="/c/Python314/python"
  PY_1=""
elif [[ -x "/c/Python312/python" ]]; then
  PY_0="/c/Python312/python"
  PY_1=""
else
  pick_py() {
    if command -v python3 >/dev/null 2>&1; then
      echo "python3"
    elif command -v python >/dev/null 2>&1; then
      echo "python"
    elif command -v py >/dev/null 2>&1; then
      echo "py|-3"
    else
      echo "run-full-backend-test-audit: python3/py/python not found" >&2
      exit 1
    fi
  }

  PY_SPEC="$(pick_py)"
  if [[ "$PY_SPEC" == py\|-3 ]]; then
    PY_0="py"
    PY_1="-3"
  else
    PY_0="$PY_SPEC"
    PY_1=""
  fi
fi

REPORT_DIR="$ROOT/reports/test"
mkdir -p "$REPORT_DIR/e2e-evidence"

py_run_script() {
  if [[ -n "$PY_1" ]]; then
    "$PY_0" "$PY_1" "$@"
  else
    "$PY_0" "$@"
  fi
}

py_run_stdin() {
  if [[ -n "$PY_1" ]]; then
    "$PY_0" "$PY_1" -
  else
    "$PY_0" -
  fi
}

AUDIT_ROWS="$REPORT_DIR/_audit_cmds.json"
echo "[]" >"$AUDIT_ROWS"

overall=0

append_audit_row() {
  export LABEL="$1" CMD="$2" ROOTP="$ROOT" EC="$3" DUR_MS="$4" AUDIT_ROWS="$AUDIT_ROWS"
  py_run_stdin <<'PY'
import json
import os
from pathlib import Path

path = Path(os.environ["AUDIT_ROWS"])
rows = json.loads(path.read_text(encoding="utf-8"))
rows.append(
    {
        "label": os.environ["LABEL"],
        "command": os.environ["CMD"],
        "cwd": os.environ["ROOTP"],
        "exit_code": int(os.environ["EC"]),
        "duration_ms": int(os.environ["DUR_MS"]),
    }
)
path.write_text(json.dumps(rows, indent=2), encoding="utf-8")
PY
}

run_one() {
  local label="$1"
  local cmd_desc="$2"
  shift 2

  SECONDS=0 || true
  set +e
  "$@"
  local ec=$?
  set -e

  local dur=$SECONDS
  if [[ "$ec" -ne 0 ]]; then
    overall=1
  fi
  append_audit_row "$label" "$cmd_desc" "$ec" "$dur"
}

run_py_one() {
  local label="$1"
  local cmd_desc="$2"
  shift 2

  SECONDS=0 || true
  set +e
  py_run_script "$@"
  local ec=$?
  set -e

  local dur=$SECONDS
  if [[ "$ec" -ne 0 ]]; then
    overall=1
  fi
  append_audit_row "$label" "$cmd_desc" "$ec" "$dur"
}

run_one go_test_audit "go test ./..." go test ./... || true
run_one gofmt_audit 'gofmt -l "." (must be empty)' bash -c 'cd "'"$ROOT"'" && test -z "$(gofmt -l .)"' || true

run_one go_vet_audit "go vet ./..." go vet ./... || true

REST_URL="${BASE_URL:-http://127.0.0.1:8080}"
run_py_one rest_inventory "python scripts/test/rest_openapi_coverage.py --base-url ${REST_URL}" \
  "${ROOT}/scripts/test/rest_openapi_coverage.py" --base-url "${REST_URL}" || true

run_py_one rest_full_live "python scripts/test/rest_full_live_coverage.py --mode local --base-url ${REST_URL}" \
  "${ROOT}/scripts/test/rest_full_live_coverage.py" --mode local --base-url "${REST_URL}" || true

run_py_one grpc_inventory "python scripts/test/grpc_inventory_coverage.py" \
  "${ROOT}/scripts/test/grpc_inventory_coverage.py" || true

run_one grpc_full "bash scripts/test/run-grpc-full-coverage.sh" \
  bash "${ROOT}/scripts/test/run-grpc-full-coverage.sh" || true

run_py_one mqtt_inventory "python scripts/test/mqtt_inventory_coverage.py" \
  "${ROOT}/scripts/test/mqtt_inventory_coverage.py" || true

run_one mqtt_full "bash scripts/test/run-mqtt-full-coverage.sh" \
  bash "${ROOT}/scripts/test/run-mqtt-full-coverage.sh" || true

run_py_one e2e_flow_inventory "python scripts/test/e2e_flow_inventory.py" \
  "${ROOT}/scripts/test/e2e_flow_inventory.py" || true

run_py_one chk_api "python scripts/test/check-api-coverage.py" \
  "${ROOT}/scripts/test/check-api-coverage.py" || true

run_py_one chk_flow "python scripts/test/check-flow-coverage.py" \
  "${ROOT}/scripts/test/check-flow-coverage.py" || true

[[ -f "$AUDIT_ROWS" ]] || echo "[]" >"$AUDIT_ROWS"

run_py_one report_gen "python scripts/test/generate-test-report.py" \
  "${ROOT}/scripts/test/generate-test-report.py" || true

export AUDIT_ROWS_FILE="$AUDIT_ROWS" REPORT_DIR="$REPORT_DIR"
py_run_script "${ROOT}/scripts/test/merge_audit_commands.py"
unset AUDIT_ROWS_FILE

py_run_script "${ROOT}/scripts/test/generate-test-report.py" || true

run_py_one full_backend_reports "python scripts/test/generate-full-backend-reports.py" \
  "${ROOT}/scripts/test/generate-full-backend-reports.py" || true

rm -f "$AUDIT_ROWS"

echo "run-full-backend-test-audit: exit ${overall}"
exit "$overall"
