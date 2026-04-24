#!/usr/bin/env bash
# Run scripts/verify_enterprise_release.sh from repo root and write a JSON result for build_release_evidence_pack.sh.
# Usage: bash deployments/prod/scripts/emit_verify_enterprise_result_json.sh /path/to/verify-result.json
set -euo pipefail

OUT="${1:?usage: emit_verify_enterprise_result_json.sh /path/to/verify-result.json}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${ROOT}"

if ! bash scripts/verify_enterprise_release.sh; then
	echo "emit_verify_enterprise_result_json: verify-enterprise-release failed" >&2
	exit 1
fi

COMPLETED="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
python3 - "${OUT}" "${COMPLETED}" <<'PY'
import json
import sys
from pathlib import Path

out, completed = sys.argv[1], sys.argv[2]
doc = {
    "schema_version": 1,
    "final_result": "pass",
    "completed_at_utc": completed,
    "tool": "verify-enterprise-release",
    "command": "bash scripts/verify_enterprise_release.sh",
}
Path(out).parent.mkdir(parents=True, exist_ok=True)
Path(out).write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
print("wrote", out)
PY
