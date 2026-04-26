#!/usr/bin/env bash
# Optional GitHub settings verification (branch protection, environments) via REST API.
# Requires GH_TOKEN or GITHUB_TOKEN and GITHUB_REPOSITORY=owner/repo for real checks.
#
# Production: verifies environment `production` exists; checks deployment_branch_policy and
# required_reviewers protection_rules when the API returns them. When ENFORCE_GITHUB_GOVERNANCE=true,
# also fails on clearly missing main-branch policy (e.g. strict required checks, missing approval count)
# as implemented in tools/verify_github_governance.py. Undetectable or ruleset-only policy → warnings
# and/or printed **manual UI checklist** (code cannot set branch/environment protection from this repo).
#
# Local: missing token → warning + manual checklist to stdout, exit 0.
# CI: ENFORCE_GITHUB_GOVERNANCE=true and missing token → exit 2 + manual checklist to stderr.
#
# Optional: GITHUB_GOVERNANCE_WARN_ONLY=true — if the API omits environment fields, treat as warnings (not for CI gating).
#
# Usage:
#   bash scripts/ci/verify_github_governance.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

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
  echo "verify_github_governance.sh: error: python3 (or python) is required" >&2
  exit 1
fi

exec "${python_exec}" "${ROOT}/tools/verify_github_governance.py" "$@"
