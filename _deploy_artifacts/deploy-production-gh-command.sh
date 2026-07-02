#!/usr/bin/env bash
# =============================================================================
# WARNING: Review production-deploy-inputs.json before running.
# Security Release never auto-deploys production.
# =============================================================================
set -Eeuo pipefail
_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_JSON="${_HERE}/production-deploy-inputs.json"
python3 - "${_HERE}" <<'PY'
import json, pathlib, sys

TODO = "TODO_STAGING_EVIDENCE_RUN_ID"
here = pathlib.Path(sys.argv[1])

def check(name: str) -> None:
    path = here / name
    if not path.is_file():
        return
    raw = path.read_text(encoding="utf-8")
    if TODO in raw:
        print(
            "error: %s contains %s — replace with a successful Staging Deployment Contract run id before deploy."
            % (name, TODO),
            file=sys.stderr,
        )
        sys.exit(1)
    data = json.loads(raw)
    sid = (data.get("staging_evidence_id") or "").strip()
    allow = data.get("allow_missing_staging_evidence") is True
    reason = (data.get("missing_staging_evidence_reason") or "").strip()
    if sid == TODO:
        print("error: staging_evidence_id is still the TODO literal in %s." % name, file=sys.stderr)
        sys.exit(1)
    if not sid and not (allow and reason):
        print(
            "error: staging_evidence_id empty in %s without allow_missing_staging_evidence=true "
            "and a non-empty missing_staging_evidence_reason." % name,
            file=sys.stderr,
        )
        sys.exit(1)

for fn in ("production-deploy-inputs.json", "production-deploy-request.json"):
    check(fn)
PY
cd "${REPO_ROOT:?export REPO_ROOT to your avf-vending-api clone}"
gh workflow run "Deploy Production" --ref main --json < "${_JSON}"
