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

# From `on:` up to the next `concurrency:` (top level). Works for workflows where `concurrency` follows
# the on block (possibly with # comments, e.g. deploy-prod).
on_until_concurrency() {
  awk '/^on:/{p=1;next} /^concurrency:/{exit} p' "$1"
}

# Staging contract: on block is immediately followed (after inputs) by a top-level # comment; stop there
# so we do not include `permissions:` in the "on" slice.
on_until_column0_hash() {
  awk '/^on:/{p=1;next} p && /^#/{exit} p' "$1"
}

# deploy-prod: legacy extractor — same end condition as for deploy-prod in earlier revisions.
deploy_prod_on_block() {
  awk '/^on:/{p=1;next} p && /^[a-zA-Z#]/ {exit} p' "${WF}/deploy-prod.yml"
}

echo "Checking workflow contracts under ${WF}"

# Prefer an interpreter that actually runs (skip broken Windows "python3" app-install stubs when possible).
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
  fail "a working python3 (or python) is required for workflow heredoc checks"
fi

# --- build-push: CI workflow_run completed + dispatch; only develop/main push paths in job if ---
on_until_concurrency "${WF}/build-push.yml" | grep -qE '^[[:space:]]*workflow_run:' || fail "build-push.yml must use workflow_run trigger"
on_until_concurrency "${WF}/build-push.yml" | grep -qE '^[[:space:]]*workflow_dispatch:' || fail "build-push.yml must use workflow_dispatch trigger"
on_until_concurrency "${WF}/build-push.yml" | grep -qE '[[:space:]]*-[[:space:]]*CI' || fail "build-push.yml workflow_run must list CI as upstream"
on_until_concurrency "${WF}/build-push.yml" | grep -q 'completed' || fail "build-push.yml workflow_run must use types completed (or types: completed)"
grep -q "head_branch == 'develop'" "${WF}/build-push.yml" || fail "build-push.yml must restrict workflow_run builds to develop (with main)"
grep -q "head_branch == 'main'" "${WF}/build-push.yml" || fail "build-push.yml must restrict workflow_run builds to main (with develop)"
grep -q "github.event.inputs.build_target_branch == 'develop'" "${WF}/build-push.yml" || fail "build-push.yml workflow_dispatch must allow develop target branch"
grep -q "github.event.inputs.build_target_branch == 'main'" "${WF}/build-push.yml" || fail "build-push.yml workflow_dispatch must allow main target branch"

# --- security-release: after Build and Push completed ---
sr_on_block="$(on_until_concurrency "${WF}/security-release.yml")"
printf '%s\n' "${sr_on_block}" | grep -q 'Build and Push Images' || fail "security-release.yml must trigger on Build and Push Images"
printf '%s\n' "${sr_on_block}" | grep -q 'completed' || fail "security-release.yml workflow_run must use completed"
printf '%s\n' "${sr_on_block}" | grep -qE 'workflow_run:|workflow_dispatch' || fail "security-release.yml must declare workflow_run and may declare workflow_dispatch"
grep -q 'name: security-verdict' "${WF}/security-release.yml" || fail "security-release.yml must upload a security-verdict artifact (name: security-verdict)"
grep -qE 'uses: actions/upload-artifact@|actions/upload-artifact' "${WF}/security-release.yml" || fail "security-release.yml must use actions/upload-artifact to publish the verdict"
for key in source_build_run_id source_sha source_branch release_gate_verdict; do
  grep -q "${key}" "${WF}/security-release.yml" || fail "security-release.yml must reference security-verdict field in workflow code: ${key}"
done
# published_images embed app / goose
grep -qE "app_image_ref|published_images" "${WF}/security-release.yml" || fail "security-release.yml must reference app_image_ref (or published_images) in workflow code"
grep -qE "goose_image_ref|published_images" "${WF}/security-release.yml" || fail "security-release.yml must reference goose_image_ref (or published_images) in workflow code"

# --- deploy-develop: after Security Release; security-verdict + source_build_run_id; staging can be disabled ---
dd_on_block="$(on_until_column0_hash "${WF}/deploy-develop.yml")"
printf '%s\n' "${dd_on_block}" | grep -qE '^[[:space:]]*workflow_run:' || fail "deploy-develop.yml must use workflow_run trigger"
printf '%s\n' "${dd_on_block}" | grep -q 'Security Release' || fail "deploy-develop.yml must trigger on Security Release"
printf '%s\n' "${dd_on_block}" | grep -q 'completed' || fail "deploy-develop.yml must require workflow_run completed (types)"
# --- deploy-develop must not be triggered by repo-level "Security" (only Security Release) ---
if grep -E '^[[:space:]]*-[[:space:]]+Security[[:space:]]*$' "${WF}/deploy-develop.yml" >/dev/null 2>&1; then
  fail "deploy-develop.yml must not list a workflow_run trigger named only 'Security' (use Security Release)."
fi
grep -q "github.event.workflow_run.conclusion == 'success'" "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must require workflow_run.conclusion == success (avoid racing failed upstream)"
grep -q 'ENABLE_REAL_STAGING_DEPLOY' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must reference ENABLE_REAL_STAGING_DEPLOY for real vs no-op staging"
grep -q 'source_build_run_id' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must reference source_build_run_id (from security-verdict / candidate)"
grep -q 'security-verdict' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must consume the security-verdict artifact"
# Do not use GitHub workflow runs list API to pick Build by yml (must use source_build_run_id from verdict).
if grep -qE 'actions/workflows/[^"[:space:]]+\.(yml|yaml)/runs' "${WF}/deploy-develop.yml"; then
  fail "deploy-develop.yml must not list workflow runs by workflow yaml path; resolve build via security-verdict source_build_run_id"
fi
# Do not use head_sha in gh api (race / wrong-candidate class of bugs).
if grep 'gh api' "${WF}/deploy-develop.yml" | grep -q 'head_sha' 2>/dev/null; then
  fail "deploy-develop.yml: gh api must not use head_sha to resolve a build; use security-verdict first"
fi
# Contract documentation line (reordered resolver regression)
grep -q 'loads Build artifacts' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml should document that Build artifacts load by run id from security-verdict (see candidate resolver summary)"

# --- deploy-prod: no automatic triggers; no pull_request; manual / main-only ---
dp_on_for_triggers="$(on_until_concurrency "${WF}/deploy-prod.yml")"
if printf '%s\n' "${dp_on_for_triggers}" | grep -qE '^[[:space:]]*pull_request[[:space:]]*:'; then
  fail "deploy-prod.yml must not declare on.pull_request (enterprise policy)"
fi
if printf '%s\n' "${dp_on_for_triggers}" | grep -qE '^[[:space:]]*push[[:space:]]*:'; then
  fail "deploy-prod.yml must not declare on.push (production is manual or tightly gated, not on push)"
fi
if printf '%s\n' "${dp_on_for_triggers}" | grep -qE '^[[:space:]]*schedule[[:space:]]*:'; then
  fail "deploy-prod.yml must not declare on.schedule (no scheduled production)"
fi

if grep -E '^[[:space:]]*-[[:space:]]+Security[[:space:]]*$' "${WF}/deploy-prod.yml" >/dev/null 2>&1; then
  fail "deploy-prod.yml must not list a workflow_run trigger named only 'Security' (use Security Release)."
fi
grep -q 'Security Release' "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must reference Security Release for production evidence"
dp_on_legacy="$(deploy_prod_on_block)"
if echo "${dp_on_legacy}" | grep -qE '^[[:space:]]*workflow_run:'; then
  fail "deploy-prod.yml must not declare on.workflow_run (enterprise policy: production is workflow_dispatch only)"
fi
grep -q 'workflow_dispatch:' "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must be triggered with workflow_dispatch"
grep -qE 'source_branch must be main|artifact_source_branch must be main for production' "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must validate security-verdict source_branch is main"
grep -q 'DEPLOY_PRODUCTION' "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must require DEPLOY_PRODUCTION for manual deploy confirmation"
grep -q 'deploy_production_confirmation' "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must define deploy_production_confirmation input"
grep -q 'security_release_run_id' "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must accept security_release_run_id (Security Release run for deploy mode)"
grep -q "github.ref == 'refs/heads/main'" "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must gate production jobs to the main branch ref"

# --- Required verdict-style fields referenced in production workflow code ---
for key in source_build_run_id source_sha source_branch app_image_ref goose_image_ref release_gate_verdict; do
  grep -q "${key}" "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must reference field ${key} in workflow (security/artifact contract)"
done

# --- Deployable image refs: digest-pinned in deploy paths ---
for f in deploy-prod.yml deploy-develop.yml _reusable-deploy.yml; do
  grep -qE '@sha256:|digest-pinned' "${WF}/${f}" || fail "${f} must keep digest-pinned image policy (@sha256: or explicit digest-pinned checks)"
done

# --- Branch filters: staging/develop (additional) ---
grep -q "head_branch == 'develop'" "${WF}/deploy-develop.yml" || fail "deploy-develop.yml should gate on develop (head_branch)"
grep -q "branches:" "${WF}/deploy-develop.yml" || fail "deploy-develop.yml should declare workflow_run branches"
grep -A6 'workflow_run:' "${WF}/deploy-develop.yml" | grep -q 'develop' || fail "deploy-develop.yml workflow_run should filter develop"
grep -qE 'source_branch must be develop|staging candidate source_branch must be develop' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must validate security-verdict source_branch is develop"
grep -q 'automatic staging requires source_event push' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must require security-verdict source_event push for automatic staging"
grep -q 'verify_workflow_contracts.sh' .github/workflows/ci.yml || fail "ci.yml must run scripts/ci/verify_workflow_contracts.sh"

# --- build-push: must chain only from CI by name, push, develop/main (downstream security-verdict expects Build and Push run id) ---
grep -qE "github.event.workflow_run.name == 'CI'|github.event.workflow_run.name == \"CI\"" "${WF}/build-push.yml" || fail "build-push.yml must gate on workflow_run.name == 'CI'"

# --- Reusable deploy exposes artifact_source_event ---
grep -q 'artifact_source_event:' "${WF}/_reusable-deploy.yml" || fail "_reusable-deploy.yml must expose artifact_source_event output"

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
if grep -q 'build-push.yml/runs' "${WF}/deploy-develop.yml"; then
  fail "deploy-develop.yml must not query build-push.yml/runs for candidate resolution (use security-verdict)."
fi
if grep -q 'build-push.yml/runs' "${WF}/deploy-prod.yml"; then
  fail "deploy-prod.yml must not query build-push.yml/runs for automatic candidate resolution (use security-verdict)."
fi

# --- Anti-regression: dangerous jq that only allows push on build resolver (without workflow_run) in deploy workflows ---
# Allow lines that include workflow_run in the same select expression.
for f in "${WF}/deploy-develop.yml" "${WF}/deploy-prod.yml"; do
  if grep -E '\.event // ""\) == "push"' "${f}" >/dev/null 2>&1; then
    if ! grep -E 'workflow_run' "${f}" >/dev/null 2>&1; then
      fail "${f}: build event filter mentions only push; ensure workflow_run chain / artifact validation is present."
    fi
  fi
done

# --- Bash heredoc / multiline: reject unquoted <<WORD (allowlisted exceptions) ---
"${python_exec}" - <<'PY'
import re
import sys
from pathlib import Path

def load_allow():
  p = Path("scripts/ci/workflow_heredoc_allowlist.txt")
  out = []
  if not p.is_file():
    return out
  for line in p.read_text(encoding="utf-8", errors="replace").splitlines():
    s = line.strip()
    if not s or s.startswith("#"):
      continue
    out.append(re.compile(s))
  return out


def line_has_unsafe_heredoc_token(line: str) -> bool:
  """True if a bash-style unquoted heredoc (<<WORD) is present; excludes <<<, <<', <<", <<- , <<: , <<GHA*."""
  if "<<" not in line:
    return False
  i = 0
  n = len(line)
  while i < n - 1:
    if line[i : i + 3] == "<<<":
      i += 3
      continue
    if line[i : i + 2] != "<<":
      i += 1
      continue
    rest = line[i + 2 :]
    if not rest:
      i += 2
      continue
    t = rest.lstrip()
    if t.startswith(("<", "'", '"', "!", "-", ":", "#")):
      i += 2
      continue
    if t[:3] == "GHA" or t.startswith("GHA"):
      i += 2
      continue
    if re.match(r"^[A-Za-z_][A-Za-z0-9_]*", t):
      return True
    i += 2
  return False


allow_res = load_allow()
bad = []
wdir = Path(".github/workflows")
for yml in sorted(wdir.glob("*.yml")):
  text = yml.read_text(encoding="utf-8", errors="replace")
  for lineno, line in enumerate(text.splitlines(), 1):
    if not line_has_unsafe_heredoc_token(line):
      continue
    if any(r.search(line) for r in allow_res):
      continue
    bad.append(f"{yml}:{lineno}:{line!r}")
if bad:
  print(
    "verify_workflow_contracts.sh: error: unquoted or fragile bash heredoc (<<WORD) in workflow line(s). "
    "Use <<'LIT', <<\"LIT\", <<- , <<: , GitHub *<<GHA* multiline, or add a regex to scripts/ci/workflow_heredoc_allowlist.txt",
    file=sys.stderr,
  )
  for b in bad:
    print(f"  {b}", file=sys.stderr)
  sys.exit(1)
PY

echo "Workflow contract checks passed."
