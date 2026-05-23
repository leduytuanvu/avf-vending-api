#!/usr/bin/env bash
# Static contract checks for the production E2E governance automation window.
# No GitHub API calls — ensures scripts/docs/workflow exist and do not permanently weaken protection.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() {
  echo "verify_governance_protection_window.sh: error: $*" >&2
  exit 1
}

ENABLE="${ROOT}/scripts/governance/enable-production-e2e-automation-window.sh"
RESTORE="${ROOT}/scripts/governance/restore-production-protections.sh"
WORKFLOW="${ROOT}/.github/workflows/production-e2e-automation-window.yml"
RUNBOOK="${ROOT}/docs/runbooks/production-e2e-automation-window.md"
PY="${ROOT}/tools/production_e2e_governance_window.py"
LEGACY_DEPLOY="${ROOT}/.github/workflows/deploy-production.yml"

for f in "${ENABLE}" "${RESTORE}" "${WORKFLOW}" "${RUNBOOK}" "${PY}"; do
  [[ -f "${f}" ]] || fail "missing required file: ${f}"
done

# Enable script must require explicit confirmation (delegated to Python; shell passes through).
grep -qF "AVF_E2E_AUTOMATION_WINDOW_CONFIRM" "${ENABLE}" || \
  fail "enable script must document AVF_E2E_AUTOMATION_WINDOW_CONFIRM"
grep -qF "I_ACCEPT_TEMPORARY_PRODUCTION_AUTOMATION_RISK" "${PY}" || \
  fail "Python tool must require I_ACCEPT_TEMPORARY_PRODUCTION_AUTOMATION_RISK"

# Bypass actor must be explicit — never silently weaken all protection.
grep -qF "AVF_E2E_AUTOMATION_BYPASS_ACTOR_ID" "${PY}" || \
  fail "governance tool must require AVF_E2E_AUTOMATION_BYPASS_ACTOR_ID"
grep -qF "Will NOT remove all protection" "${PY}" || \
  fail "enable path must fail when bypass actors are unsupported (no silent protection removal)"

# Must not permanently delete branch protection / rulesets.
if grep -qE '(DELETE|delete).*(branch.*protection|rulesets/)' "${PY}"; then
  fail "governance tool must not DELETE branch protection or rulesets wholesale"
fi

# Restore must be documented, referenced, and support status/dry-run.
grep -qF "restore-production-protections.sh" "${RUNBOOK}" || \
  fail "runbook must reference restore-production-protections.sh"
grep -qF "restore-production-protections.sh" "${WORKFLOW}" || \
  fail "workflow must reference restore-production-protections.sh"
grep -qF -e "--status" "${RESTORE}" || fail "restore script must support --status"
grep -qF -e "--dry-run" "${RESTORE}" || fail "restore script must support --dry-run"

# Workflow: workflow_dispatch only — no push/pull_request/workflow_run triggers.
if grep -qE '^[[:space:]]{2}(push|pull_request|workflow_run|schedule):' "${WORKFLOW}"; then
  fail "production-e2e-automation-window.yml must not auto-run on push/PR/workflow_run/schedule"
fi
grep -qE '^[[:space:]]{2}workflow_dispatch:' "${WORKFLOW}" || \
  fail "production-e2e-automation-window.yml must declare workflow_dispatch"

# deploy-prod.yml remains canonical; deploy-production.yml is notice-only (no real deploy).
grep -q '^name: Deploy Production$' "${ROOT}/.github/workflows/deploy-prod.yml" || \
  fail "deploy-prod.yml must remain the canonical Deploy Production workflow"
[[ -f "${LEGACY_DEPLOY}" ]] || fail "deploy-production.yml pointer workflow must exist"
grep -qF "NO REAL DEPLOY" "${LEGACY_DEPLOY}" || \
  fail "deploy-production.yml must remain notice-only (NO REAL DEPLOY banner)"
if grep -vE '^[[:space:]]*#' "${LEGACY_DEPLOY}" | grep -qE '^[[:space:]]+environment:[[:space:]]+production[[:space:]]*$'; then
  fail "deploy-production.yml must not use environment: production (pointer only)"
fi

# Snapshot directory under gitignored .e2e-runs/
grep -qE '^\.e2e-runs/' "${ROOT}/.gitignore" || \
  fail ".gitignore must ignore .e2e-runs/ (governance snapshots live there)"

# Governance scripts must not embed production credentials.
for secret_pat in \
  'E2E_PROD_ADMIN_PASSWORD=' \
  'COMMERCE_PAYMENT_WEBHOOK_SECRET=' \
  'DATABASE_URL=postgres' \
  'BEGIN OPENSSH PRIVATE KEY' \
  'Bearer eyJ'; do
  if grep -qF "${secret_pat}" "${ENABLE}" "${RESTORE}" "${PY}" 2>/dev/null; then
    fail "governance scripts must not contain hardcoded secret pattern: ${secret_pat}"
  fi
done

# Python syntax (GitHub-hosted Linux + Windows dev shells).
python_exec=""
for c in python3 python; do
  if command -v "${c}" >/dev/null 2>&1 && "${c}" -c "import sys" 2>/dev/null; then
    python_exec="${c}"
    break
  fi
done
if [[ -z "${python_exec}" ]] && command -v py >/dev/null 2>&1 && py -3 -c "import sys" 2>/dev/null; then
  python_exec="py -3"
fi
[[ -n "${python_exec}" ]] || fail "python3, python, or py -3 required for syntax check"
${python_exec} -m py_compile "${PY}"

echo "GOVERNANCE_PROTECTION_WINDOW: PASS"
