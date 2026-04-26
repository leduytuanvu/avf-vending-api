#!/usr/bin/env bash
# Security Release signal: orchestration (repo Security evidence via resolve_repo_security_evidence.sh).
set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "$REPO_ROOT"

mkdir -p security-reports
generated_at_utc="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
export GENERATED_AT_UTC="${generated_at_utc}"
emergency_on_missing() {
  if [[ -f security-reports/security-verdict.json ]] && [[ -s security-reports/security-verdict.json ]]; then
    if python3 -c "import json,sys; p=json.load(open('security-reports/security-verdict.json',encoding='utf-8')); v=(p.get('verdict') or '').strip().lower(); sys.exit(0 if v in ('pass','fail','skipped','no-candidate') else 1)" 2>/dev/null; then
      return 0
    fi
  fi
  export GENERATED_AT_UTC="${generated_at_utc}"
  echo "::error::security-reports/security-verdict.json was not written or is invalid; emitting emergency fail verdict" >&2
  python3 scripts/security/write_security_verdict.py emergency --emergency-reason "Exit trap: verdict missing or invalid (Security Release signal step ended before a successful contract write)"
  return 0
}
trap 'emergency_on_missing' EXIT
if [[ ! -f scripts/security/write_security_verdict.py ]]; then
  echo "::error::expected scripts/security/write_security_verdict.py from checkout" >&2
  exit 1
fi
emit_verdict_outputs_and_summary() {
  set +e
  python3 scripts/security/emit_security_verdict_outputs.py >> "$GITHUB_OUTPUT"
  _eo=$?
  python3 scripts/security/emit_security_verdict_summary.py --section main >> "$GITHUB_STEP_SUMMARY"
  _es=$?
  set -e
  if [[ "${_eo}" -ne 0 ]]; then
    echo "::warning::emit_security_verdict_outputs failed (non-fatal after verdict JSON write)" >&2
  fi
  if [[ "${_es}" -ne 0 ]]; then
    echo "::warning::emit_security_verdict_summary failed (non-fatal after verdict JSON write)" >&2
  fi
}
emit_source_debug_table_safe() {
  set +e
  python3 scripts/security/emit_release_signal_debug_table.py
  _ed=$?
  set -e
  if [[ "${_ed}" -ne 0 ]]; then
    echo "::warning::emit_release_signal_debug_table failed (non-fatal after verdict write)" >&2
  fi
}
# skip-when-build-incomplete: success => upstream Build ineligible; emit skipped verdict (never reach full gate)
SKIP_R="${SKIP_JOB_RESULT:-}"
if [[ "${SKIP_R}" == "success" ]]; then
  if [[ "${EVENT_NAME}" == "workflow_run" ]]; then
    te="${TRIGGERING_BUILD_EVENT:-}"
    tconc="${TRIGGERING_BUILD_CONCLUSION:-}"
    thb="${TRIGGERING_BUILD_HEAD_BRANCH:-}"
    {
      echo ""
      echo "## Security Release skipped (ineligible **Build and Push Images** run)"
    } >> "$GITHUB_STEP_SUMMARY"
    if [[ "${te}" == "workflow_run" ]]; then
      {
        echo "- **Why skipped**: the triggering **Build and Push Images** run was started by a \`workflow_run\` (CI chain), not a direct \`push\` to \`develop\`/\`main\` or an allowed \`workflow_dispatch\`. Indirect/chain-only builds are not valid release candidates."
        echo "- (diagnostic) \`github.event.workflow_run.event\`=\`${te}\`, \`head_branch\`=\`${thb}\`, \`conclusion\`=\`${tconc}\`."
      } >> "$GITHUB_STEP_SUMMARY"
      python3 scripts/security/write_security_verdict.py unsupported-trigger
    elif [[ -n "${thb}" && "${thb}" != "develop" && "${thb}" != "main" ]]; then
      {
        echo "- **Why skipped**: the triggering **Build and Push Images** \`head_branch\` is \`${thb}\` (only \`develop\` and \`main\` are release lines)."
      } >> "$GITHUB_STEP_SUMMARY"
      python3 scripts/security/write_security_verdict.py skipped
    elif [[ -n "${tconc}" && "${tconc}" != "success" ]]; then
      {
        echo "- **Why skipped**: the triggering **Build and Push Images** \`conclusion\` is \`${tconc}\` (required: \`success\`)."
      } >> "$GITHUB_STEP_SUMMARY"
      python3 scripts/security/write_security_verdict.py skipped
    elif [[ -n "${te}" && "${te}" != "push" && "${te}" != "workflow_dispatch" ]]; then
      {
        echo "- **Why skipped**: the triggering **Build and Push Images** GHA \`event\` is \`${te}\` (valid promotion sources: \`push\` or \`workflow_dispatch\` only)."
      } >> "$GITHUB_STEP_SUMMARY"
      python3 scripts/security/write_security_verdict.py unsupported-trigger
    else
      {
        echo "- **Why skipped**: release gate requires a direct **Build and Push Images** success from \`push\` or \`workflow_dispatch\` on \`develop\`/\`main\` (see reasons in verdict JSON)."
      } >> "$GITHUB_STEP_SUMMARY"
      python3 scripts/security/write_security_verdict.py skipped
    fi
  else
    python3 scripts/security/write_security_verdict.py skipped
  fi
  emit_verdict_outputs_and_summary
  trap - EXIT
  exit 0
fi
RBR_RES="${RESOLVE_BUILD_RUN_RESULT:-}"
RIR_RES="${RESOLVE_IMAGE_REFS_RESULT:-}"
NO_RC=0
if [[ "${RBR_RES}" != "success" ]]; then
  NO_RC=1
fi
if [[ "${RIR_RES}" != "success" ]]; then
  NO_RC=1
fi
if [[ "${EVENT_NAME}" == "workflow_run" ]]; then
  TCONC="${TRIGGERING_BUILD_CONCLUSION:-}"
  TNAME="${TRIGGERING_BUILD_WF_NAME:-}"
  if [[ -n "${TCONC}" && "${TCONC}" != "success" ]]; then
    NO_RC=1
  fi
  if [[ -n "${TNAME}" && "${TNAME}" != "Build and Push Images" ]]; then
    NO_RC=1
  fi
  # Do not use triggering Build head_branch for release-candidate identity (may differ from promotion-manifest when workflow_run chains).
fi
if [[ "${NO_RC}" -eq 1 ]]; then
  python3 scripts/security/write_security_verdict.py no-candidate
  emit_verdict_outputs_and_summary
  {
    echo ""
    echo "## No releasable candidate. Security release gate skipped."
  } >> "$GITHUB_STEP_SUMMARY"
  trap - EXIT
  exit 0
fi
if [[ "${EVENT_NAME}" == "workflow_run" ]]; then
  _ghe="${TRIGGERING_BUILD_EVENT:-}"
  if [[ "${_ghe}" == "workflow_run" ]] || [[ -n "${_ghe}" && "${_ghe}" != "push" && "${_ghe}" != "workflow_dispatch" ]]; then
    {
      echo ""
      echo "## Security Release skipped (defense: triggering Build GHA event is ineligible for promotion)"
      echo "- **Why skipped**: full gate only runs for **Build and Push Images** when \`github.event.workflow_run.event\` is \`push\` or \`workflow_dispatch\` (got: \`${_ghe}\`)."
    } >> "$GITHUB_STEP_SUMMARY"
    python3 scripts/security/write_security_verdict.py unsupported-trigger
    emit_verdict_outputs_and_summary
    trap - EXIT
    exit 0
  fi
  _thb2="${TRIGGERING_BUILD_HEAD_BRANCH:-}"
  if [[ -n "${_thb2}" && "${_thb2}" != "develop" && "${_thb2}" != "main" ]]; then
    {
      echo ""
      echo "## Security Release skipped (defense: triggering **Build and Push Images** \`head_branch\` is not develop/main)"
      echo "- \`head_branch\`=\`${_thb2}\`"
    } >> "$GITHUB_STEP_SUMMARY"
    python3 scripts/security/write_security_verdict.py skipped
    emit_verdict_outputs_and_summary
    trap - EXIT
    exit 0
  fi
  _tco2="${TRIGGERING_BUILD_CONCLUSION:-}"
  if [[ -n "${_tco2}" && "${_tco2}" != "success" ]]; then
    {
      echo ""
      echo "## Security Release skipped (defense: triggering **Build and Push Images** was not a success)"
      echo "- \`conclusion\`=\`${_tco2}\`"
    } >> "$GITHUB_STEP_SUMMARY"
    python3 scripts/security/write_security_verdict.py no-candidate
    emit_verdict_outputs_and_summary
    trap - EXIT
    exit 0
  fi
fi
# Repo Security (push) — candidate identity: promotion-manifest / image-metadata only. Never prefer BUILD_HEAD_* / TRIGGERING_* over RESOLVED_*.
A_EV="${ARTIFACT_SOURCE_EVENT:-}"
# Primary SHA: promotion-manifest / image-metadata (RESOLVED_SOURCE_SHA) only. Never use triggering Build head_sha
# or this Security Release job checkout (github.sha / WORKFLOW_SHA) for automatic workflow_run — that would race
# and mis-associate commits. WORKFLOW_SHA fallback is allowed only for workflow_dispatch when artifacts are absent.
release_source_sha="${RESOLVED_SOURCE_SHA:-}"
if [[ -z "${release_source_sha// }" ]]; then
  if [[ "${EVENT_NAME}" == "workflow_dispatch" ]]; then
    release_source_sha="${WORKFLOW_SHA:-}"
  fi
fi
# Primary branch: RESOLVED, then manual dispatch, then ref — never BUILD_HEAD_BRANCH for Security API matching.
want_branch="${RESOLVED_SOURCE_BRANCH:-${MANUAL_TARGET_BRANCH:-${GITHUB_REF_NAME:-}}}"
c_ev=""
if [[ "${A_EV}" == "manual" ]]; then
  c_ev="workflow_dispatch"
elif [[ -n "${A_EV// }" && "${A_EV}" != "manual" ]]; then
  c_ev="${A_EV}"
fi
export CANONICAL_SOURCE_SHA="${release_source_sha}"
export CANONICAL_SOURCE_BRANCH="${want_branch}"
export CANONICAL_SOURCE_EVENT="${c_ev}"
META_CONFLICT=""
if [[ "${EVENT_NAME}" == "workflow_run" ]]; then
  _tname="${TRIGGERING_BUILD_WF_NAME:-}"
  _tconc="${TRIGGERING_BUILD_CONCLUSION:-}"
  _tev="${TRIGGERING_BUILD_EVENT:-}"
  _thb="${TRIGGERING_BUILD_HEAD_BRANCH:-}"
  _tsha="${TRIGGERING_BUILD_HEAD_SHA:-}"
  _tid="${TRIGGERING_BUILD_ID:-}"
  _bid="${BUILD_RUN_ID:-}"
  _abr="${RESOLVED_SOURCE_BRANCH:-}"
  _asha="${RESOLVED_SOURCE_SHA:-}"
  _aev="${ARTIFACT_SOURCE_EVENT:-}"
  if [[ -n "${_tname}" && "${_tname}" != "Build and Push Images" ]]; then
    META_CONFLICT="Triggering workflow name is ${_tname} (expected Build and Push Images)."
  fi
  if [[ -z "${META_CONFLICT// }" && -n "${_tconc}" && "${_tconc}" != "success" ]]; then
    META_CONFLICT="Triggering workflow conclusion is ${_tconc} (expected success)."
  fi
  if [[ -z "${META_CONFLICT// }" && -n "${_tev}" && "${_tev}" != "push" && "${_tev}" != "workflow_dispatch" ]]; then
    META_CONFLICT="Triggering workflow GHA event is ${_tev} (expected push or workflow_dispatch for automatic release gating)."
  fi
  if [[ -z "${META_CONFLICT// }" && -n "${_thb}" && "${_thb}" != "develop" && "${_thb}" != "main" ]]; then
    META_CONFLICT="Triggering head_branch is ${_thb} (expected develop or main)."
  fi
  if [[ -z "${META_CONFLICT// }" && -n "${_tid}" && -n "${_bid}" && "${_tid}" != "${_bid}" ]]; then
    META_CONFLICT="Build run id mismatch: triggering workflow run id is ${_tid} but artifact build_run_id is ${_bid} (must be the same Build and Push Images run for this Security Release)."
  fi
  if [[ -z "${META_CONFLICT// }" && -n "${_aev}" && "${_aev}" == "push" && "${_tev}" == "workflow_run" ]]; then
    META_CONFLICT="Promotion manifest source_event is push but the triggering Build GHA event is workflow_run (default-branch/chain context cannot match direct push semantics with these artifacts)."
  fi
  # RESOLVED_SOURCE_BRANCH / RESOLVED_SOURCE_SHA win for repo Security matching; triggering head_branch may differ (e.g. default-branch Build context).
  if [[ -z "${META_CONFLICT// }" && "${_tev}" == "push" && "${_aev}" == "push" ]]; then
    if [[ -n "${_tsha}" && -n "${_asha}" && "${#_tsha}" -ge 4 && "${#_asha}" -ge 4 && "${_tsha}" != "${_asha}" && "${PROMOTION_MANIFEST_ALLOW_SHA_MISMATCH:-}" != "1" ]]; then
      META_CONFLICT="SHA mismatch: triggering Build head_sha and promotion-manifest RESOLVED source_sha differ. Set repository variable PROMOTION_MANIFEST_ALLOW_SHA_MISMATCH=1 only for a documented promotion-manifest exception (retarget/rebase policy)."
    fi
  fi
  if [[ -z "${META_CONFLICT// }" && "${_tev}" == "workflow_dispatch" ]]; then
    if [[ ( "${_aev}" == "workflow_dispatch" || "${_aev}" == "manual" ) && -n "${_tsha}" && -n "${_asha}" && "${#_tsha}" -ge 4 && "${#_asha}" -ge 4 && "${_tsha}" != "${_asha}" && "${PROMOTION_MANIFEST_ALLOW_SHA_MISMATCH:-}" != "1" ]]; then
      META_CONFLICT="SHA mismatch for manual/dispatch build: set PROMOTION_MANIFEST_ALLOW_SHA_MISMATCH=1 only for a documented manifest exception."
    fi
  fi
fi
if [[ -n "${META_CONFLICT// }" ]]; then
  export METADATA_CONFLICT_REASON="${META_CONFLICT}"
  {
    echo ""
    echo "## Security Release skipped: artifact / triggering metadata mismatch"
    echo "- **decision**: \`skipped\` (full release gate not run; this is not an emergency-fail)"
    echo ""
    echo "| Field | Value |"
    echo "|---|---|"
    echo "| \`triggering_build_run_id\` | \`${TRIGGERING_BUILD_ID:-}\` |"
    echo "| \`build_run_id_from_artifacts\` | \`${BUILD_RUN_ID:-}\` |"
    echo "| \`triggering_build_event\` | \`${TRIGGERING_BUILD_EVENT:-}\` |"
    echo "| \`triggering_build_head_branch\` | \`${TRIGGERING_BUILD_HEAD_BRANCH:-}\` |"
    echo "| \`triggering_build_head_sha\` | \`${TRIGGERING_BUILD_HEAD_SHA:-}\` |"
    echo "| \`triggering_workflow_name\` | \`${TRIGGERING_BUILD_WF_NAME:-}\` |"
    echo "| \`triggering_workflow_conclusion\` | \`${TRIGGERING_BUILD_CONCLUSION:-}\` |"
    echo "| \`artifact_source_event\` | \`${ARTIFACT_SOURCE_EVENT:-}\` |"
    echo "| \`resolved_source_branch\` | \`${RESOLVED_SOURCE_BRANCH:-}\` |"
    echo "| \`resolved_source_sha\` | \`${RESOLVED_SOURCE_SHA:-}\` |"
  } >> "$GITHUB_STEP_SUMMARY"
  python3 scripts/security/write_security_verdict.py metadata-mismatch
  emit_verdict_outputs_and_summary
  trap - EXIT
  exit 0
fi
{
  echo "- canonical source sha (gate): ${release_source_sha}  (RESOLVED_SOURCE_SHA preferred; WORKFLOW_SHA only for manual workflow_dispatch when artifacts lack SHA)"
  echo "- canonical source branch (gate): ${want_branch}  (RESOLVED_SOURCE_BRANCH preferred)"
  echo "- artifact source event: ${A_EV:-unknown}"
  echo "- Build vs artifact metadata (defensive check): \`ok\` — proceeding to full gate (see **metadata_validation** in JSON)"
  echo "- triggering \`build_run_id\` (GHA): \`${TRIGGERING_BUILD_ID:-}\`  artifact \`build_run_id\`: \`${BUILD_RUN_ID:-}\`"
  echo "- triggering \`head_branch\` (diagnostic only): \`${TRIGGERING_BUILD_HEAD_BRANCH:-unknown}\`"
  echo "- resolved source workflow (artifact): ${RESOLVED_SOURCE_WORKFLOW_NAME:-}"
  echo "- diagnostic triggering GHA \`event\` (Build): \`${TRIGGERING_BUILD_EVENT:-unknown}\`"
} >> "$GITHUB_STEP_SUMMARY"
{
  echo ""
  echo "### Build vs artifact — observability (pre-gate)"
  echo ""
  echo "| Field | Value |"
  echo "|---|---|"
  echo "| \`triggering_build_run_id\` | \`${TRIGGERING_BUILD_ID:-}\` |"
  echo "| \`build_run_id_from_artifacts\` | \`${BUILD_RUN_ID:-}\` |"
  echo "| \`triggering_build_event\` | \`${TRIGGERING_BUILD_EVENT:-}\` |"
  echo "| \`triggering_build_head_branch\` | \`${TRIGGERING_BUILD_HEAD_BRANCH:-}\` |"
  echo "| \`artifact_source_event\` | \`${A_EV:-}\` |"
  echo "| \`resolved_source_branch\` | \`${RESOLVED_SOURCE_BRANCH:-}\` |"
  echo "| \`resolved_source_sha\` | \`${RESOLVED_SOURCE_SHA:-}\` |"
  echo "| \`decision\` (this stage) | \`proceed\` to repo **Security** run match and full verdict writer |"
} >> "$GITHUB_STEP_SUMMARY"
# Semantic source_event in promotion-manifest: only push, workflow_dispatch, or manual; never workflow_run (chain) for release promotion
if [[ -n "${A_EV// }" && "${A_EV}" != "push" && "${A_EV}" != "workflow_dispatch" && "${A_EV}" != "manual" ]]; then
  python3 scripts/security/write_security_verdict.py unsupported-artifact-source-event
  emit_verdict_outputs_and_summary
  trap - EXIT
  exit 0
fi
if [[ -z "${want_branch// }" || ( "${want_branch}" != "develop" && "${want_branch}" != "main" ) ]]; then
  python3 scripts/security/write_security_verdict.py ineligible-branch
  emit_verdict_outputs_and_summary
  trap - EXIT
  exit 0
fi
if [[ -z "${release_source_sha// }" ]]; then
  {
    echo ""
    echo "## Security Release skipped: empty canonical source SHA"
    echo "- **Why skipped / no-candidate**: \`RESOLVED_SOURCE_SHA\` (artifacts) and checkout fallback were empty; cannot evaluate the release gate (expected promotion-manifest / image-metadata or \`WORKFLOW_SHA\` for dispatch)."
  } >> "$GITHUB_STEP_SUMMARY"
  export ARTIFACT_CONSISTENCY_NOTE="empty canonical source SHA: RESOLVED_SOURCE_SHA and WORKFLOW_SHA fallback both empty (artifact chain incomplete)."
  python3 scripts/security/write_security_verdict.py no-candidate
  emit_verdict_outputs_and_summary
  trap - EXIT
  exit 0
fi
_repo_ev="$(mktemp)"
bash "${SCRIPT_DIR}/resolve_repo_security_evidence.sh" "${_repo_ev}"
set -a
# shellcheck disable=SC1090
source "${_repo_ev}"
set +a
rm -f "${_repo_ev}"

# REPO_RELEASE_* and RELEASE_RESOLUTION_ERROR were exported via set -a + source above.
export RELEASE_SOURCE_SHA="${release_source_sha}"
export PROMOTION_MANIFEST_FOR_SBOM=""
export RELEASE_CANDIDATE_JSON=""
_prom_dl="$(mktemp -d)"
if [[ -n "${BUILD_RUN_ID:-}" ]] && [[ "${BUILD_RUN_ID}" =~ ^[0-9]+$ ]]; then
  if gh run download "${BUILD_RUN_ID}" -n promotion-manifest -D "${_prom_dl}" 2>/dev/null; then
    if [[ -f "${_prom_dl}/promotion-manifest.json" ]]; then
      export PROMOTION_MANIFEST_FOR_SBOM="${_prom_dl}/promotion-manifest.json"
    fi
  fi
  if gh run download "${BUILD_RUN_ID}" -n release-candidate -D "${_prom_dl}" 2>/dev/null; then
    if [[ -f "${_prom_dl}/release-candidate.json" ]]; then
      export RELEASE_CANDIDATE_JSON="${_prom_dl}/release-candidate.json"
    fi
  fi
fi
set +e
python3 scripts/security/write_security_verdict.py full
_vw=$?
set -e
if [[ "${_vw}" -ne 0 ]]; then
  echo "::error::write_security_verdict.py full mode failed; writing emergency fail verdict" >&2
  set +e
  python3 scripts/security/write_security_verdict.py emergency --emergency-reason "write_security_verdict.py full exited with code ${_vw}"
  set -e
fi
emit_verdict_outputs_and_summary
emit_source_debug_table_safe
trap - EXIT
