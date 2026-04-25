#!/usr/bin/env bash
# CI guard: enforce enterprise CI/CD release graph contracts for GitHub Actions.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WF="${ROOT}/.github/workflows"

cd "${ROOT}"

fail() {
  echo "verify_workflow_contracts.sh: error: $*" >&2
  exit 1
}

echo "Checking workflow contracts under ${WF}"

# --- deploy-develop must not be triggered by repo-level "Security" (only Security Release) ---
if grep -E '^[[:space:]]*-[[:space:]]+Security[[:space:]]*$' .github/workflows/deploy-develop.yml >/dev/null 2>&1; then
  fail "deploy-develop.yml must not list a workflow_run trigger named only 'Security' (use Security Release)."
fi
grep -q 'Security Release' .github/workflows/deploy-develop.yml || fail "deploy-develop.yml must trigger on Security Release"

# --- deploy-prod must not be triggered by "Security" ---
if grep -E '^[[:space:]]*-[[:space:]]+Security[[:space:]]*$' .github/workflows/deploy-prod.yml >/dev/null 2>&1; then
  fail "deploy-prod.yml must not list a workflow_run trigger named only 'Security' (use Security Release)."
fi
grep -q 'Security Release' .github/workflows/deploy-prod.yml || fail "deploy-prod.yml must trigger on Security Release"

# --- Branch filters: staging/develop vs production/main ---
grep -q "head_branch == 'develop'" .github/workflows/deploy-develop.yml || fail "deploy-develop.yml should gate on develop (head_branch)"
grep -q "branches:" .github/workflows/deploy-develop.yml || fail "deploy-develop.yml should declare workflow_run branches"
grep -A6 'workflow_run:' .github/workflows/deploy-develop.yml | grep -q 'develop' || fail "deploy-develop.yml workflow_run should filter develop"
grep -qE 'source_branch must be develop|staging candidate source_branch must be develop' .github/workflows/deploy-develop.yml || fail "deploy-develop.yml must validate security-verdict source_branch is develop"
grep -q 'automatic staging requires source_event push' .github/workflows/deploy-develop.yml || fail "deploy-develop.yml must require security-verdict source_event push for automatic staging"
grep -q 'verify_workflow_contracts.sh' .github/workflows/ci.yml || fail "ci.yml must run scripts/ci/verify_workflow_contracts.sh"

grep -q "head_branch == 'main'" .github/workflows/deploy-prod.yml || fail "deploy-prod.yml should gate auto path on main (head_branch)"
grep -A8 'workflow_run:' .github/workflows/deploy-prod.yml | grep -q 'main' || fail "deploy-prod.yml workflow_run should filter main"
grep -qE 'source_branch must be main|production candidate source_branch must be main' .github/workflows/deploy-prod.yml || fail "deploy-prod.yml must validate security-verdict source_branch is main"
grep -q 'DEPLOY_PRODUCTION' .github/workflows/deploy-prod.yml || fail "deploy-prod.yml must require DEPLOY_PRODUCTION for manual deploy confirmation"
grep -q 'deploy_production_confirmation' .github/workflows/deploy-prod.yml || fail "deploy-prod.yml must define deploy_production_confirmation input"

# --- Reusable deploy exposes artifact_source_event ---
grep -q 'artifact_source_event:' .github/workflows/_reusable-deploy.yml || fail "_reusable-deploy.yml must expose artifact_source_event output"

# --- Unique workflow names (top-level name: key) ---
dupes="$(
  grep -h '^name:' .github/workflows/*.yml 2>/dev/null | sed 's/^name:[[:space:]]*//' | tr -d '\r' | sort | uniq -d || true
)"
if [[ -n "${dupes}" ]]; then
  fail "duplicate workflow name: values found:\n${dupes}"
fi

# --- Only deploy-prod may combine workflow_run + production environment ---
while IFS= read -r -d '' f; do
  base="$(basename "${f}")"
  [[ "${base}" == "deploy-prod.yml" ]] && continue
  [[ "${base}" == "deploy-production.yml" ]] && continue
  if grep -q 'workflow_run:' "${f}" && grep -q 'environment:[[:space:]]*production' "${f}"; then
    fail "${base} must not use workflow_run together with environment: production (automatic production is only deploy-prod.yml)."
  fi
done < <(find .github/workflows -maxdepth 1 -name '*.yml' -print0)

# --- Auto path must not rely only on build-push API for candidate (artifact / security-verdict source of truth) ---
if grep -q 'build-push.yml/runs' .github/workflows/deploy-develop.yml; then
  fail "deploy-develop.yml must not query build-push.yml/runs for candidate resolution (use security-verdict)."
fi
if grep -q 'build-push.yml/runs' .github/workflows/deploy-prod.yml; then
  fail "deploy-prod.yml must not query build-push.yml/runs for automatic candidate resolution (use security-verdict)."
fi

# --- Anti-regression: dangerous jq that only allows push on build resolver (without workflow_run) in deploy workflows ---
# Allow lines that include workflow_run in the same select expression.
for f in .github/workflows/deploy-develop.yml .github/workflows/deploy-prod.yml; do
  if grep -E '\.event // ""\) == "push"' "${f}" >/dev/null 2>&1; then
    if ! grep -E 'workflow_run' "${f}" >/dev/null 2>&1; then
      fail "${f}: build event filter mentions only push; ensure workflow_run chain / artifact validation is present."
    fi
  fi
done

echo "Workflow contract checks passed."
