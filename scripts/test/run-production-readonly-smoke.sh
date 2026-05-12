#!/usr/bin/env bash
# shellcheck shell=bash
# Production/staging read-only smoke. Never calls mutating routes.

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT_DIR="${ROOT}/reports/test"
mkdir -p "${REPORT_DIR}"

OUT_JSON="${REPORT_DIR}/PRODUCTION_PROOF_REPORT.json"
OUT_MD="${REPORT_DIR}/PRODUCTION_PROOF_REPORT.md"
BASE_URL="${BASE_URL:-}"

if [[ -z "${BASE_URL}" ]]; then
  python3 - "${OUT_JSON}" "${OUT_MD}" <<'PY'
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

payload = {
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "production_readonly_smoke": "BLOCKED",
    "reason": "BASE_URL/STAGING_BASE_URL/PRODUCTION_BASE_URL not configured",
    "required_next_action": "Set BASE_URL to staging or production URL and rerun scripts/test/run-production-readonly-smoke.sh",
    "probes": [],
}
Path(sys.argv[1]).write_text(json.dumps(payload, indent=2), encoding="utf-8")
Path(sys.argv[2]).write_text(
    "# Production Proof Report\n\n"
    "- Production read-only smoke: **BLOCKED**\n"
    "- Reason: BASE_URL/STAGING_BASE_URL/PRODUCTION_BASE_URL not configured\n"
    "- Next action: set BASE_URL and rerun `scripts/test/run-production-readonly-smoke.sh`.\n",
    encoding="utf-8",
)
PY
  echo "Production read-only smoke BLOCKED: BASE_URL not configured"
  exit 0
fi

ALLOW_LOCAL_SMOKE="${ALLOW_LOCAL_SMOKE:-false}" BASE_URL="${BASE_URL}" bash "${ROOT}/scripts/test/run-readonly-smoke.sh"

python3 - "${REPORT_DIR}/readonly-smoke.json" "${OUT_JSON}" "${OUT_MD}" <<'PY'
import json
import sys
from pathlib import Path

src = Path(sys.argv[1])
payload = json.loads(src.read_text(encoding="utf-8"))
status = "PASS" if int(payload.get("exit_code", 1)) == 0 else "FAIL"
report = {
    "generated_at": payload.get("generated_at"),
    "production_readonly_smoke": status,
    "base_url": payload.get("base_url"),
    "probes": payload.get("probes", []),
    "evidence_path": str(src),
}
Path(sys.argv[2]).write_text(json.dumps(report, indent=2), encoding="utf-8")
with Path(sys.argv[3]).open("w", encoding="utf-8") as f:
    f.write("# Production Proof Report\n\n")
    f.write(f"- Production read-only smoke: **{status}**\n")
    f.write(f"- Base URL: `{payload.get('base_url')}`\n")
    f.write(f"- Evidence: `{src}`\n\n")
    f.write("| Method | Path | Status | Latency ms |\n")
    f.write("|---|---|---:|---:|\n")
    for probe in payload.get("probes", []):
        f.write(f"| GET | `{probe.get('path')}` | {probe.get('status')} | {probe.get('latency_ms')} |\n")
PY
