#!/usr/bin/env bash
# Staging: run canonical post-deploy smoke (tools/smoke_test.py), then write machine-readable evidence
# and a Markdown summary for GITHUB_STEP_SUMMARY. Invoked from deploy-develop.yml after deploy.
# On smoke failure, still emits evidence when smoke-test.json exists (smoke_test.py writes before exit).
#
# Requires: BASE_URL, ENVIRONMENT_NAME=staging (set by workflow). See scripts/deploy/smoke_test.sh.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"
export SMOKE_REPORT_PATH="${ROOT}/smoke-reports/smoke-test.json"

set +e
bash "${ROOT}/scripts/deploy/smoke_test.sh"
_s_rc=$?
set -e

if [[ -f "${SMOKE_REPORT_PATH}" ]]; then
  cp -f "${SMOKE_REPORT_PATH}" "${ROOT}/smoke-reports/staging-smoke-report.json"
fi

python_exec=""
for c in python3 python; do
  if command -v "${c}" >/dev/null 2>&1; then
    if "${c}" -c "import sys" 2>/dev/null; then
      python_exec="${c}"
      break
    fi
  fi
done
if [[ -z "${python_exec}" ]]; then
  echo "staging_smoke_evidence: error: python3 required" >&2
  exit 1
fi

if [[ -f "${SMOKE_REPORT_PATH}" ]]; then
  "${python_exec}" - <<'PY'
import json
import os
from pathlib import Path

report_path = Path(os.environ["SMOKE_REPORT_PATH"])
data = json.loads(report_path.read_text(encoding="utf-8"))
checks = data.get("checks") or []
overall = data.get("overall") or data.get("overall_status") or "unknown"
base = data.get("base_url") or ""
envn = data.get("environment_name") or ""

lines = [
    "## Staging smoke test evidence",
    "",
    f"- **environment**: `{envn}`",
    f"- **base_url**: `{base}`",
    f"- **overall**: `{overall}`",
    "",
    "| Check | Category | Status | HTTP | Detail |",
    "| --- | --- | --- | --- | --- |",
]
for c in checks:
    name = (c.get("name") or "").replace("|", "\\|")
    cat = c.get("category") or ""
    st = c.get("status") or ""
    http = c.get("http_status")
    http_s = "" if http is None else str(http)
    det = (c.get("detail") or "").replace("|", "\\|")[:500]
    lines.append(f"| {name} | `{cat}` | **{st}** | {http_s} | {det} |")

lines.append("")
lines.append("- Artifact: `staging-smoke-evidence` (JSON + `staging-smoke-summary.md`).")
lines.append(
    "- **Required** rows must be pass for a good deploy; **optional** `skip` means not configured in CI (not a pass). "
)
text = "\n".join(lines) + "\n"

summary_path = Path("smoke-reports/staging-smoke-summary.md")
summary_path.parent.mkdir(parents=True, exist_ok=True)
summary_path.write_text(text, encoding="utf-8")

gh = os.environ.get("GITHUB_STEP_SUMMARY")
if gh:
    Path(gh).write_text(text, encoding="utf-8")
PY
  echo "staging_smoke_evidence: wrote smoke-reports/staging-smoke-report.json and staging-smoke-summary.md"
else
  echo "staging_smoke_evidence: warning: no ${SMOKE_REPORT_PATH} — smoke may have failed before write" >&2
fi

exit "${_s_rc}"
