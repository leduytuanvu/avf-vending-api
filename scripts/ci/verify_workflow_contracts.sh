#!/usr/bin/env bash
# CI guard: enforce enterprise CI/CD release graph contracts for GitHub Actions.
# Deterministic: reads only local .github/workflows/*.yml (no GitHub API, no network).
#
# Fails PR CI before merge on common regressions, including:
#   - deploy-prod: on.workflow_run / develop / pull_request (production is workflow_dispatch on main)
#   - security-release: non-empty verdict + security-verdict.json + CONTRACT_VERDICT_MODES + emit before each exit 0 in signal step
#   - security-release: workflow_run GHA guards (success + push|dispatch + develop|main); never chain event as promotion source
#   - security-release: no unprotected read/mapfile + < <( under set -e (or set +e/|| true); want_branch RESOLVED before BUILD_HEAD; smoke test
#   - build-push: release_candidate + push gate (no fake release artifacts on skip)
#   - deploy-develop: Security Release only (not Security), skipped verdict neutral exit, source_build_run_id
#   - canonical display names, duplicate "Security", heredoc safety (see also Python heredoc check below)
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

# `on:` block through first top-level `jobs:` (workflows with no `concurrency` before `jobs`, e.g. deploy-production pointer).
on_until_jobs() {
  awk '/^on:/{p=1;next} /^jobs:/{exit} p' "$1"
}

echo "Checking workflow contracts under ${WF}"

# --- Unique top-level workflow display names: Security vs Security Release ---
# Exactly one workflow must be named "Security" (security.yml) and one "Security Release" (security-release.yml).
sec_exact="$(grep -h '^name: Security$' "${WF}"/*.yml 2>/dev/null | wc -l | tr -d ' ')"
sec_rel_exact="$(grep -h '^name: Security Release$' "${WF}"/*.yml 2>/dev/null | wc -l | tr -d ' ')"
if [[ "${sec_exact}" -gt 1 ]]; then
  fail "more than one workflow is named \"Security\" (expected exactly one: security.yml)"
fi
if [[ "${sec_exact}" -ne 1 ]]; then
  fail "expected exactly one workflow named \"Security\" in security.yml; found ${sec_exact}"
fi
if [[ "${sec_rel_exact}" -ne 1 ]]; then
  fail "expected exactly one workflow named \"Security Release\" in security-release.yml; found ${sec_rel_exact}"
fi
if grep -q '^name: Security$' "${WF}/security-release.yml" 2>/dev/null; then
  fail "security-release.yml must be named \"Security Release\", not \"Security\""
fi

# --- Canonical workflow display names (required release-chain names) ---
grep -q '^name: CI$' "${WF}/ci.yml" || fail "ci.yml must be named \"CI\" (canonical CI workflow)"
grep -q '^name: Security$' "${WF}/security.yml" || fail "security.yml must be named \"Security\" (repo-level; distinct from Security Release)"
grep -q '^name: Build and Push Images$' "${WF}/build-push.yml" || fail "build-push.yml must be named \"Build and Push Images\""
grep -q '^name: Security Release$' "${WF}/security-release.yml" || fail "security-release.yml must be named \"Security Release\" (not \"Security\")"
grep -q '^name: Staging Deployment Contract$' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must be named \"Staging Deployment Contract\""
grep -q '^name: Deploy Production$' "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must be named \"Deploy Production\""

# --- No duplicate canonical display names (beyond Security vs Security Release, checked above) ---
# Note: duplicate top-level `name:` values are checked later in this script.

# --- deploy-prod.yml must not declare on.workflow_run (no automatic production from develop or upstream chains) ---
# Two-space indent matches keys directly under top-level `on:` in GitHub's workflow schema.
if grep -qE '^[[:space:]]{2}workflow_run[[:space:]]*:' "${WF}/deploy-prod.yml"; then
  fail "deploy-prod.yml must not declare on.workflow_run (Deploy Production is workflow_dispatch-only; no auto-deploy from develop)"
fi

# --- build-push: required release artifacts (downstream Security Release / staging consume these) ---
grep -qF "name: immutable-image-contract" "${WF}/build-push.yml" || fail "build-push.yml must upload the immutable-image-contract artifact (digest-pinned contract; missing artifact breaks Security Release / staging)"
grep -qF "name: promotion-manifest" "${WF}/build-push.yml" || fail "build-push.yml must upload the promotion-manifest artifact"
grep -qF "name: image-build-metadata" "${WF}/build-push.yml" || fail "build-push.yml must upload image-build-metadata (compatibility bundle)"
grep -qF "uses: ./.github/workflows/_reusable-build.yml" "${WF}/build-push.yml" || fail "build-push.yml must call _reusable-build.yml (publishes image-metadata artifact used by Security Release)"
grep -qF "name: image-metadata" "${WF}/_reusable-build.yml" || fail "_reusable-build.yml must upload the image-metadata artifact (Security Release / _reusable-deploy expect this name)"

# --- security-release: verdict must not be stoppable as empty; JSON comes from scripts/security/write_security_verdict.py ---
grep -qF "blocking security verdict is empty" "${WF}/security-release.yml" || fail "security-release.yml must fail closed when the resolved verdict is empty (expected pass, fail, or skipped in SECURITY_VERDICT / security-verdict.json)"
grep -qF "SECURITY_VERDICT" "${WF}/security-release.yml" || fail "security-release.yml must use SECURITY_VERDICT in the Enforce step (blocking empty string)"
grep -qF "security-reports/security-verdict.json" "${WF}/security-release.yml" || fail "security-release.yml must write security-reports/security-verdict.json (uploaded as security-verdict artifact)"
# Required writer modes are derived from scripts/security/write_security_verdict.py CONTRACT_VERDICT_MODES (checked in Python below; regression: empty verdict / missing skip path).
grep -qF "No releasable candidate. Security release gate skipped." "${WF}/security-release.yml" || fail "security-release.yml must document the neutral no-release-candidate skipped outcome (Build chain did not produce a releasable candidate)"
grep -qF "scripts/security/emit_security_verdict_outputs.py" "${WF}/security-release.yml" || fail "security-release.yml must call emit_security_verdict_outputs.py after each verdict write (GITHUB_OUTPUT contract)"
grep -qF "scripts/security/emit_security_verdict_summary.py" "${WF}/security-release.yml" || fail "security-release.yml must call emit_security_verdict_summary.py (job summary contract)"
grep -qF "export CANONICAL_SOURCE_SHA" "${WF}/security-release.yml" || fail "security-release.yml must export CANONICAL_SOURCE_SHA (artifact-first source coordinates for Security / verdict)"
grep -qF "scripts/security/emit_release_signal_debug_table.py" "${WF}/security-release.yml" || fail "security-release.yml must call emit_release_signal_debug_table.py (source coordinate debug summary)"
# Polling: must not use read < <(pipeline) on jq output (empty under set -e); require safe security_run_row + read <<< with || true
grep -qF "security_run_row" "${WF}/security-release.yml" || fail "security-release.yml must use security_run_row for repo Security (push) run polling (safe under set -e)"
# Release candidate identity: artifact-first (never prefer triggering workflow_run head over promotion-manifest)
grep -qF 'release_source_sha="${RESOLVED_SOURCE_SHA' "${WF}/security-release.yml" || fail "security-release.yml must set release_source_sha from RESOLVED_SOURCE_SHA first (artifact source_sha)"
grep -qF 'want_branch="${RESOLVED_SOURCE_BRANCH' "${WF}/security-release.yml" || fail "security-release.yml must set want_branch from RESOLVED_SOURCE_BRANCH before MANUAL_TARGET_BRANCH / GITHUB_REF_NAME (never BUILD_HEAD_BRANCH as candidate branch)"
if grep -E 'want_branch=.*(BUILD_HEAD_BRANCH|TRIGGERING_BUILD_HEAD_BRANCH)' "${WF}/security-release.yml" 2>/dev/null; then
  fail "security-release.yml: candidate want_branch must not be derived from BUILD_HEAD_BRANCH or TRIGGERING_BUILD_HEAD_BRANCH (use RESOLVED_SOURCE_BRANCH from promotion-manifest)"
fi
grep -qF "scripts/security/write_security_verdict.py ineligible-branch" "${WF}/security-release.yml" || fail "security-release.yml must call write_security_verdict.py ineligible-branch (canonical branch not develop/main; neutral skip)"
grep -qF "scripts/security/write_security_verdict.py unsupported-artifact-source-event" "${WF}/security-release.yml" || fail "security-release.yml must call write_security_verdict.py unsupported-artifact-source-event when ARTIFACT_SOURCE_EVENT is not an allowed semantic event"
if grep -qF "github.event.workflow_run.event == 'workflow_run'" "${WF}/security-release.yml" 2>/dev/null; then
  fail "security-release.yml must not treat a Build and Push Images run whose GHA event is workflow_run (chain) as a release candidate (only push and workflow_dispatch)"
fi
# Primary SHA: never workflow_run head_sha; WORKFLOW_SHA-only fallback is allowed when RESOLVED is empty
if grep -n "TRIGGER_WORKFLOW_RUN_SOURCE_SHA" "${WF}/security-release.yml" | grep -E 'release_source_sha=|source_sha=|CANONICAL_SOURCE' 2>/dev/null; then
  fail "security-release.yml: must not assign candidate SHA from TRIGGER_WORKFLOW_RUN_SOURCE_SHA (misleading in workflow_run chains; use RESOLVED_SOURCE_SHA or WORKFLOW_SHA for dispatch)"
fi

# --- security-release: automatic trigger must require success + push|dispatch + develop|main on workflow_run (defense against chain-only / wrong-branch builds) ---
_sr_succ="$(grep -cF "github.event.workflow_run.conclusion == 'success'" "${WF}/security-release.yml" 2>/dev/null | tr -d ' ' || echo 0)"
if [[ "${_sr_succ}" -lt 3 ]]; then
  fail "security-release.yml: expected at least 3 occurrences of github.event.workflow_run.conclusion == 'success' in job if: filters (found ${_sr_succ}); release gate must require a successful Build run"
fi
_sr_ev="$(grep -cF "github.event.workflow_run.event == 'push' || github.event.workflow_run.event == 'workflow_dispatch'" "${WF}/security-release.yml" 2>/dev/null | tr -d ' ' || echo 0)"
if [[ "${_sr_ev}" -lt 3 ]]; then
  fail "security-release.yml: expected at least 3 occurrences of (push || workflow_dispatch) on github.event.workflow_run.event (found ${_sr_ev}); workflow_run must not be treated as a valid Build source event for promotion"
fi
_sr_hb="$(grep -cF "(github.event.workflow_run.head_branch == 'develop' || github.event.workflow_run.head_branch == 'main')" "${WF}/security-release.yml" 2>/dev/null | tr -d ' ' || echo 0)"
if [[ "${_sr_hb}" -lt 3 ]]; then
  fail "security-release.yml: expected at least 3 occurrences of head_branch develop|main guards on workflow_run (found ${_sr_hb})"
fi
grep -qF "github.event.workflow_run.event != 'push' && github.event.workflow_run.event != 'workflow_dispatch'" "${WF}/security-release.yml" || fail "security-release.yml must document the skip-when-build-incomplete negation (Build GHA event must be push or workflow_dispatch, not workflow_run chain-only)"
grep -qF '[[ "${V}" == "fail" ]]' "${WF}/security-release.yml" || fail "security-release.yml Enforce step must block on verdict fail (full gate JSON with verdict=fail); missing fail branch allows pass with broken release gate"
grep -qF "python3 scripts/security/write_security_verdict.py full" "${WF}/security-release.yml" || fail "security-release.yml must call write_security_verdict.py full (pass and fail outcomes both emit JSON via write_full)"

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

# --- security-release: CONTRACT_VERDICT_MODES, signal-step emit before exit 0, read/mapfile + < <(, RESOLVED before BUILD_HEAD ---
"${python_exec}" - <<'PY' || fail "security-release.yml: static bash safety / CONTRACT_VERDICT_MODES / signal emit checks failed"
from __future__ import annotations

import ast
import re
import sys
from pathlib import Path

lines = Path(".github/workflows/security-release.yml").read_text(encoding="utf-8", errors="replace").splitlines()
wf_text = "\n".join(lines)


def code_only(line: str) -> str:
    return line.split("#", 1)[0]


def str_from_ast_elt(elt: ast.AST) -> str | None:
    if isinstance(elt, ast.Constant) and isinstance(elt.value, str):
        return elt.value
    if isinstance(elt, ast.Str):
        return elt.s
    return None


contract_modes: tuple[str, ...] | None = None
mod = ast.parse(Path("scripts/security/write_security_verdict.py").read_text(encoding="utf-8", errors="replace"))
for node in mod.body:
    if isinstance(node, ast.AnnAssign) and isinstance(node.target, ast.Name) and node.target.id == "CONTRACT_VERDICT_MODES":
        if isinstance(node.value, ast.Tuple):
            cm = tuple(x for elt in node.value.elts if (x := str_from_ast_elt(elt)))
            if cm:
                contract_modes = cm
        break
if not contract_modes:
    print(
        "verify_workflow_contracts: error: scripts/security/write_security_verdict.py must define "
        "CONTRACT_VERDICT_MODES = (\"skipped\", ...) for the workflow contract checker.",
        file=sys.stderr,
    )
    sys.exit(1)

for m in contract_modes:
    needle = "write_security_verdict.py %s" % m
    if needle not in wf_text:
        print(
            "verify_workflow_contracts: error: security-release.yml must invoke `python3 scripts/security/%s` "
            "(every CONTRACT_VERDICT_MODES entry must have a reachable write path so security-verdict.json is never missing on skip/fail)." % needle,
            file=sys.stderr,
        )
        sys.exit(1)

# Signal step: every `exit 0` must follow a prior emit_verdict_outputs_and_summary in-branch (regression: trap / empty verdict).
sig = None
for i, line in enumerate(lines):
    if "name: Write structured security release signal" in line:
        sig = i
        break
if sig is None:
    print("verify_workflow_contracts: error: security-release.yml missing step 'Write structured security release signal'", file=sys.stderr)
    sys.exit(1)
run_j = None
for j in range(sig, min(sig + 220, len(lines))):
    if re.match(r"^\s+run: \|\s*$", lines[j]):
        run_j = j + 1
        break
if run_j is None:
    print("verify_workflow_contracts: error: security-release.yml signal step has no run: | block (after env:)", file=sys.stderr)
    sys.exit(1)
end = None
for k in range(run_j, len(lines)):
    if lines[k].startswith("      - name:"):
        end = k
        break
if end is None:
    print("verify_workflow_contracts: error: could not find end of signal run script (next step)", file=sys.stderr)
    sys.exit(1)
body_lines = lines[run_j:end]
for i, line in enumerate(body_lines):
    if not re.match(r"^\s*exit 0(\s*;.*)?\s*$", line):
        continue
    chunk = body_lines[max(0, i - 55) : i + 1]
    if "emit_verdict_outputs_and_summary" not in "\n".join(chunk):
        print(
            "verify_workflow_contracts: error: security-release signal run: `exit 0` at offset line %d (~file line %d) "
            "has no prior emit_verdict_outputs_and_summary in the prior 55 lines (skip/no-candidate paths must emit JSON outputs before exit)."
            % (i + 1, run_j + i + 1),
            file=sys.stderr,
        )
        sys.exit(1)

# read/mapfile + process substitution < <(  must be safe under set -e: || true, or set +e before / set -e after (tight window).
for i, line in enumerate(lines):
    c0 = code_only(line)
    if re.match(r"^\s*#", line):
        continue
    if re.match(r"^\s*actions:\s*read", c0) or re.match(r"^\s*-\s*read\s*:", c0):
        continue
    if not re.search(r"\b(read|mapfile)\b", c0):
        continue
    j_end = min(i + 35, len(lines))
    block = [code_only(x) for x in lines[i:j_end]]
    joined = "\n".join(block)
    if "< <(" not in joined:
        continue
    if re.search(r"\|\|\s*true\b", joined):
        continue
    small_before = "\n".join([code_only(x) for x in lines[max(0, i - 8) : i]])
    small_after = "\n".join([code_only(x) for x in lines[i + 1 : min(i + 8, len(lines))]])
    if "set +e" in small_before and "set -e" in small_after:
        continue
    print(
        "verify_workflow_contracts: error: security-release.yml line %d: read/mapfile with < <( is not safe under set -e: "
        "add `|| true` on the same statement, or place the line between `set +e` (within 8 lines before) and `set -e` (within 8 lines after)."
        % (i + 1,),
        file=sys.stderr,
    )
    print(joined[:1200], file=sys.stderr)
    sys.exit(1)

# Candidate branch: RESOLVED_SOURCE_BRANCH must be listed before any BUILD*HEAD*BRANCH on the same line
res_token = "RESOLVED_SOURCE_BRANCH"
bad_tokens = ("BUILD_HEAD_BRANCH", "TRIGGERING_BUILD_HEAD_BRANCH")
for n, line in enumerate(lines, 1):
    if res_token not in line or not any(b in line for b in bad_tokens):
        continue
    if re.match(r"^\s*#", line):
        continue
    if not any(
        k in line
        for k in (
            "want_branch",
            "CANONICAL_SOURCE_BRANCH",
            "C_BR",
            "source_branch",
            "release_source",
        )
    ):
        continue
    ri = line.find(res_token)
    for bad in bad_tokens:
        bi = line.find(bad)
        if bi != -1 and ri != -1 and bi < ri:
            print(
                "verify_workflow_contracts: error: line %d: candidate resolution lists %s before %s" % (n, bad, res_token),
                file=sys.stderr,
            )
            print(line.rstrip()[:200], file=sys.stderr)
            sys.exit(1)
PY

# write_security_verdict.py smoke: always emits security-reports/security-verdict.json
"${python_exec}" "${ROOT}/scripts/security/tests/test_write_security_verdict_smoke.py" || fail "scripts/security/tests/test_write_security_verdict_smoke.py failed"

# --- build-push: CI workflow_run completed + dispatch; only develop/main push paths in job if ---
on_until_concurrency "${WF}/build-push.yml" | grep -qE '^[[:space:]]*workflow_run:' || fail "build-push.yml must use workflow_run trigger"
on_until_concurrency "${WF}/build-push.yml" | grep -qE '^[[:space:]]*workflow_dispatch:' || fail "build-push.yml must use workflow_dispatch trigger"
on_until_concurrency "${WF}/build-push.yml" | grep -qE '[[:space:]]*-[[:space:]]*CI' || fail "build-push.yml workflow_run must list CI as upstream"
on_until_concurrency "${WF}/build-push.yml" | grep -q 'completed' || fail "build-push.yml workflow_run must use types completed (or types: completed)"
build_push_wr_block="$(
  awk '/^  workflow_run:/{p=1;next} /^  workflow_dispatch:/{if(p) exit} p' "${WF}/build-push.yml"
)"
printf '%s\n' "${build_push_wr_block}" | grep -qE '^[[:space:]]*branches:' || fail "build-push.yml workflow_run must declare branches: [develop, main] so pull_request CI (head != develop/main) does not start this workflow"
printf '%s\n' "${build_push_wr_block}" | grep -q 'develop' && printf '%s\n' "${build_push_wr_block}" | grep -q 'main' || fail "build-push.yml workflow_run.branches must list develop and main"
grep -q "head_branch == 'develop'" "${WF}/build-push.yml" || fail "build-push.yml must restrict workflow_run builds to develop (with main)"
grep -q "head_branch == 'main'" "${WF}/build-push.yml" || fail "build-push.yml must restrict workflow_run builds to main (with develop)"
grep -q "github.event.inputs.build_target_branch == 'develop'" "${WF}/build-push.yml" || fail "build-push.yml workflow_dispatch must allow develop target branch"
grep -q "github.event.inputs.build_target_branch == 'main'" "${WF}/build-push.yml" || fail "build-push.yml workflow_dispatch must allow main target branch"
grep -qF "github.event.workflow_run.event == 'push'" "${WF}/build-push.yml" || fail "build-push.yml must require github.event.workflow_run.event == 'push' for workflow_run so non-push CI cannot complete Build and confuse downstream gates"
grep -qF "release_candidate" "${WF}/build-push.yml" || fail "build-push.yml must expose upstream-ci-release-gate.outputs.release_candidate so non-candidate runs skip build without a failed workflow"
grep -qF "Not a release candidate. No image was built or published." "${WF}/build-push.yml" || fail "build-push.yml must document the neutral not-a-release-candidate path (no GHCR images, no release manifest uploads)"
grep -qF "outputs.release_candidate" "${WF}/build-push.yml" || fail "build-push.yml must wire release_candidate into build-and-push when: (release artifacts only for real candidates)"
grep -qE 'upstream-ci-release-gate' "${WF}/build-push.yml" || fail "build-push.yml must use job upstream-ci-release-gate for CI chain policy"

# --- security-release: after Build and Push completed ---
sr_on_block="$(on_until_concurrency "${WF}/security-release.yml")"
printf '%s\n' "${sr_on_block}" | grep -q 'Build and Push Images' || fail "security-release.yml must trigger on Build and Push Images"
printf '%s\n' "${sr_on_block}" | grep -q 'completed' || fail "security-release.yml workflow_run must use completed"
printf '%s\n' "${sr_on_block}" | grep -qE 'workflow_run:|workflow_dispatch' || fail "security-release.yml must declare workflow_run and may declare workflow_dispatch"
sr_wr_block="$(
  awk '/^  workflow_run:/{p=1;next} /^  workflow_dispatch:/{if(p) exit} p' "${WF}/security-release.yml"
)"
printf '%s\n' "${sr_wr_block}" | grep -qE '^[[:space:]]*branches:' || fail "security-release.yml workflow_run must declare branches: [develop, main] (no direct PR chain)"
printf '%s\n' "${sr_wr_block}" | grep -q 'develop' && printf '%s\n' "${sr_wr_block}" | grep -q 'main' || fail "security-release.yml workflow_run.branches must list develop and main"
grep -q 'github.event.workflow_run.id' "${WF}/security-release.yml" || fail "security-release.yml must use github.event.workflow_run.id for Build artifact run id"
grep -qF "immutable-image-contract" "${WF}/security-release.yml" || fail "security-release.yml must require immutable-image-contract alongside other build artifacts (artifact-driven release)"
grep -q 'TRIGGER_WORKFLOW_EVENT' "${WF}/security-release.yml" || fail "security-release.yml must set TRIGGER_WORKFLOW_EVENT (Build run GHA event; distinct from semantic source_event)"
grep -q 'failure_reasons' "${WF}/security-release.yml" || fail "security-release.yml must emit failure_reasons in security-verdict"
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
if printf '%s\n' "${dd_on_block}" | grep -qE '^[[:space:]]*workflow_dispatch:'; then
  fail "deploy-develop.yml must not declare on.workflow_dispatch (trigger is Security Release completed only)"
fi
rogue_wrk="$(printf '%s\n' "${dd_on_block}" | grep -E '^[[:space:]]*-[[:space:]]*"' | grep -v 'Security Release' || true)"
[[ -z "${rogue_wrk}" ]] || fail "deploy-develop.yml workflow_run.workflows must list only Security Release (not CI, Build, etc.); got: ${rogue_wrk}"
# --- deploy-develop must not be triggered by repo-level "Security" (only Security Release) ---
if grep -E '^[[:space:]]*-[[:space:]]+Security[[:space:]]*$' "${WF}/deploy-develop.yml" >/dev/null 2>&1; then
  fail "deploy-develop.yml must listen to Security Release, not Security"
fi
"${python_exec}" - <<'PY' || fail "deploy-develop.yml must listen to Security Release, not Security (check workflow_run.workflows list)"
import pathlib
import re
import sys

path = pathlib.Path(".github/workflows/deploy-develop.yml")
text = path.read_text(encoding="utf-8")
# List items under on.workflow_run.workflows up to types:
m = re.search(
    r"(?ms)^[ \t]+workflows:\s*\n(?P<items>(?:^[ \t]+-[^\n]+\n)+)\s*^[ \t]+types:",
    text,
)
if not m:
    print("verify_workflow_contracts: could not parse deploy-develop workflow_run.workflows", file=sys.stderr)
    sys.exit(1)
for line in m.group("items").splitlines():
    item = re.sub(r"^[ \t]+-\s*", "", line).strip().strip('"').strip("'")
    if not item:
        continue
    if item != "Security Release":
        print(f"verify_workflow_contracts: disallowed workflow listener {item!r}", file=sys.stderr)
        sys.exit(1)
PY
grep -q "github.event.workflow_run.conclusion == 'success'" "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must require workflow_run.conclusion == success (avoid racing failed upstream)"
grep -qF "github.event.workflow_run.name == 'Security Release'" "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must gate jobs on the triggering workflow display name (Security Release), not Build or repo Security"
grep -q 'ENABLE_REAL_STAGING_DEPLOY' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must reference ENABLE_REAL_STAGING_DEPLOY for real vs no-op staging"
grep -q 'source_build_run_id' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must reference source_build_run_id (from security-verdict / candidate)"
grep -q 'security-verdict' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must consume the security-verdict artifact"
grep -q 'staging deployment skipped because real staging is disabled' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml no-op path must state staging deployment skipped because real staging is disabled"
grep -q 'github.event.workflow_run.id' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must use github.event.workflow_run.id (Security Release run) to download security-verdict"
grep -qF 'verdict == "skipped"' "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must treat verdict=skipped in security-verdict (neutral skip, no failed deploy)"
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

# security-verdict must appear in workflow text before any build resolution via workflow yaml /runs API (regression: head_sha search before verdict)
"${python_exec}" - <<'PY' || fail "deploy-develop.yml must resolve security-verdict (source_build_run_id) before any build-push.yml/runs or actions/workflows/…/runs query"
import pathlib
import re
import sys

p = pathlib.Path(".github/workflows/deploy-develop.yml")
lines = p.read_text(encoding="utf-8").splitlines()
verdict_line = None
bad_line = None
for i, line in enumerate(lines, 1):
    if "security-verdict" in line and not line.lstrip().startswith("#"):
        verdict_line = verdict_line or i
    if re.search(r"build-push\.yml/runs|actions/workflows/[^\"'\s]+\.(?:yml|yaml)/runs", line):
        bad_line = bad_line or i
if bad_line and (verdict_line is None or bad_line < verdict_line):
    print(
        "verify_workflow_contracts: use security-verdict and source_build_run_id before querying build by workflow yml or head_sha",
        file=sys.stderr,
    )
    sys.exit(1)
PY

# --- deploy-prod & deploy-production pointer: no automatic hooks (PR, push, develop branch, workflow_run) ---
for prod_base in deploy-prod deploy-production; do
  prod_path="${WF}/${prod_base}.yml"
  [[ -f "${prod_path}" ]] || continue
  if [[ "${prod_base}" == "deploy-prod" ]]; then
    po="$(on_until_concurrency "${prod_path}")"
  else
    po="$(on_until_jobs "${prod_path}")"
  fi
  printf '%s\n' "${po}" | grep -qE '^[[:space:]]*pull_request[[:space:]]*:' && fail "production workflow ${prod_base}.yml must not declare on.pull_request (Deploy Production must not run from PR; use workflow_dispatch on main only)"
  printf '%s\n' "${po}" | grep -qE '^[[:space:]]*push[[:space:]]*:' && fail "production workflow ${prod_base}.yml must not declare on.push (develop merge must not start Deploy Production)"
  printf '%s\n' "${po}" | grep -qE '^[[:space:]]*schedule[[:space:]]*:' && fail "production workflow ${prod_base}.yml must not declare on.schedule"
  if printf '%s\n' "${po}" | grep -qE '^[[:space:]]*workflow_run[[:space:]]*:'; then
    fail "deploy-prod.yml must be workflow_dispatch-only while production auto is disabled (on.workflow_run is not allowed)"
  fi
  if printf '%s\n' "${po}" | grep -qE '^[[:space:]]*-[[:space:]]*develop[[:space:]]*$'; then
    fail "production workflow ${prod_base}.yml must not use branch develop in on: triggers (no automatic deploy from develop into production)"
  fi
done

if grep -E '^[[:space:]]*-[[:space:]]+Security[[:space:]]*$' "${WF}/deploy-prod.yml" >/dev/null 2>&1; then
  fail "deploy-prod.yml must not list a workflow_run trigger named only 'Security' (use Security Release)."
fi
grep -q 'Security Release' "${WF}/deploy-prod.yml" || fail "deploy-prod.yml must reference Security Release for production evidence"
dp_on_legacy="$(deploy_prod_on_block)"
if echo "${dp_on_legacy}" | grep -qE '^[[:space:]]*workflow_run:'; then
  fail "deploy-prod.yml must be workflow_dispatch-only while production auto is disabled (on.workflow_run in legacy on block)"
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
grep -qF "No releasable staging candidate. Staging deployment skipped." "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must neutral-skip (no deploy) when Security Release verdict is skipped / non-candidate"
grep -qF "outputs.staging_verdict" "${WF}/deploy-develop.yml" || fail "deploy-develop.yml must expose staging_verdict to gate image resolution and deploy on verdict=pass only"
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
