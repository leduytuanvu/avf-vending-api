#!/usr/bin/env bash
# Enterprise GitHub governance verification (branch protection, production environment)
# using the GitHub REST API. Implementation uses `gh api` (via tools/verify_github_governance.py).
#
# Modes:
#   CHECK_MODE=offline — no API calls; self-test only (gh/python3 presence + checklist).
#   (default) — requires gh CLI, GH_TOKEN or GITHUB_TOKEN, and GITHUB_REPOSITORY (or REPOSITORY).
#
# CI:
#   Set GOVERNANCE_PR_IS_FORK=true to skip (fork pull_request — no base-repo governance gate in CI).
#   Set ENFORCE_GITHUB_GOVERNANCE=true to fail in CI if token is missing when enforcement is on.
#
# See: docs/operations/github-governance.md, docs/runbooks/github-governance.md
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"
readonly PY_VERIFIER="${ROOT}/tools/verify_github_governance.py"

fail() {
  echo "verify_github_governance.sh: $*" >&2
  echo "GOVERNANCE_CHECK: FAIL" >&2
  exit 1
}

print_offline_planned_checks() {
  cat <<'TEXT'
The following are validated in live mode (only after CHECK_MODE is unset and credentials are set):

  - Branch `main` (classic /protection or ruleset notes in verifier output on 404)
    - Protected; required status checks; strict (up to date) when ENFORCE is on
    - Required pull request reviews (>=1) on main
    - Force pushes blocked; branch deletion blocked when API exposes it
  - Branch `develop` — PR/reviews and/or required checks; force push and deletion blocked
  - GitHub Environment `production` — exists; required reviewers > 0; deployment branches limited
    to `main` (or protected-branches mode with main protected)

Production deploy (Deploy Production workflow) is not enterprise-ready if these GitHub settings are
missing: configure them only in the repository Settings UI; this script only reads the API.
TEXT
}

if [[ "${CHECK_MODE:-}" == "offline" ]]; then
  echo "GOVERNANCE_CHECK: PASS (offline self-test — no GitHub API calls)"
  echo ""
  if command -v gh >/dev/null 2>&1; then
    echo "  gh CLI:  found ($(command -v gh))"
  else
    echo "  gh CLI:  NOT FOUND (install for live checks: https://cli.github.com/)"
  fi
  for c in python3 python; do
    if command -v "${c}" >/dev/null 2>&1 && "${c}" -c "import sys" 2>/dev/null; then
      echo "  python:  found ($("${c}" -V 2>&1 | head -1), $(command -v "${c}"))"
      break
    fi
  done
  if [[ ! -f "${PY_VERIFIER}" ]]; then
    echo "  error: missing ${PY_VERIFIER}" >&2
    echo "GOVERNANCE_CHECK: FAIL" >&2
    exit 1
  fi
  echo "  verifier: present (${PY_VERIFIER})"
  echo ""
  print_offline_planned_checks
  exit 0
fi

if ! command -v gh >/dev/null 2>&1; then
  fail "gh CLI is required for live checks (set CHECK_MODE=offline for a dry list). Install: https://cli.github.com/"
fi

if [[ -n "${GOVERNANCE_PR_IS_FORK:-}" ]] && { [[ "${GOVERNANCE_PR_IS_FORK}" == "1" ]] || [[ "${GOVERNANCE_PR_IS_FORK}" == "true" ]]; }; then
  echo "GOVERNANCE_CHECK: SKIPPED (pull_request from a fork — governance API not run; run on develop/main after merge, or use workflow_dispatch)"
  exit 0
fi

if [[ -n "${REPOSITORY:-}" ]]; then
  export GITHUB_REPOSITORY="${REPOSITORY//[[:space:]]/}"
fi

token=""
if [[ -n "${GH_TOKEN:-}" ]]; then
  token="${GH_TOKEN}"
elif [[ -n "${GITHUB_TOKEN:-}" ]]; then
  token="${GITHUB_TOKEN}"
fi
if [[ -z "${token}" ]]; then
  if [[ "${ENFORCE_GITHUB_GOVERNANCE:-}" == "true" ]] && [[ "${GITHUB_ACTIONS:-}" == "true" || "${CI:-}" == "true" ]]; then
    echo "verify_github_governance.sh: ENFORCE_GITHUB_GOVERNANCE is set in CI but no GH_TOKEN or GITHUB_TOKEN" >&2
    echo "GOVERNANCE_CHECK: FAIL" >&2
    exit 2
  fi
  echo "GOVERNANCE_CHECK: SKIPPED (no GH_TOKEN or GITHUB_TOKEN; export one with repo read access for live checks)"
  exit 0
fi

if [[ -z "${GITHUB_REPOSITORY:-}" ]]; then
  fail "set GITHUB_REPOSITORY=owner/repo or REPOSITORY=owner/repo"
fi
if [[ "${GITHUB_REPOSITORY}" != */* ]]; then
  fail "GITHUB_REPOSITORY must be owner/name (got: ${GITHUB_REPOSITORY})"
fi

# Smoke: token can read repository metadata
repo_api="repos/${GITHUB_REPOSITORY}"
if ! GH_TOKEN="${token}" GITHUB_TOKEN="${token}" gh api -H "Accept: application/vnd.github+json" "repos/${GITHUB_REPOSITORY}" --jq '."full_name"' >/dev/null 2>&1; then
  if [[ "${GOVERNANCE_SKIP_ON_API_AUTH_FAIL:-}" == "true" ]]; then
    echo "GOVERNANCE_CHECK: SKIPPED (GitHub API not readable with current token; see docs/operations/github-governance.md)"
    exit 0
  fi
  fail "gh api could not read repository ${GITHUB_REPOSITORY} (check token scopes: repo or contents read)"
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
[[ -n "${python_exec}" ]] || fail "python3 (or python) is required to run ${PY_VERIFIER}"
[[ -f "${PY_VERIFIER}" ]] || fail "missing ${PY_VERIFIER}"

exec "${python_exec}" "${PY_VERIFIER}" "$@"
